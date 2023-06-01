package servicelb

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	ctlkubevirtv1 "github.com/harvester/harvester/pkg/generated/controllers/kubevirt.io/v1"
	ctlCorev1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"

	"github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io"
	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
	ctldiscoveryv1 "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/discovery.k8s.io/v1"
	pkglb "github.com/harvester/harvester-load-balancer/pkg/lb"
	"github.com/harvester/harvester-load-balancer/pkg/prober"
	"github.com/harvester/harvester-load-balancer/pkg/utils"
)

const (
	KeyLabel       = loadbalancer.GroupName + "/servicelb"
	KeyServiceName = "kubernetes.io/service-name"

	defaultSuccessThreshold = 1
	defaultFailureThreshold = 3
	defaultTimeout          = 3 * time.Second
	defaultPeriod           = 5 * time.Second
)

type Manager struct {
	serviceClient       ctlCorev1.ServiceClient
	serviceCache        ctlCorev1.ServiceCache
	endpointSliceClient ctldiscoveryv1.EndpointSliceClient
	endpointSliceCache  ctldiscoveryv1.EndpointSliceCache
	vmiCache            ctlkubevirtv1.VirtualMachineInstanceCache
	*prober.Manager
}

var _ pkglb.Manager = &Manager{}

func NewManager(ctx context.Context, serviceClient ctlCorev1.ServiceClient, serviceCache ctlCorev1.ServiceCache,
	endpointSliceClient ctldiscoveryv1.EndpointSliceClient, endpointSliceCache ctldiscoveryv1.EndpointSliceCache,
	vmiCache ctlkubevirtv1.VirtualMachineInstanceCache) *Manager {
	m := &Manager{
		serviceClient:       serviceClient,
		serviceCache:        serviceCache,
		endpointSliceClient: endpointSliceClient,
		endpointSliceCache:  endpointSliceCache,
		vmiCache:            vmiCache,
	}
	m.Manager = prober.NewManager(ctx, m.updateHealthCondition)

	return m
}

func (m *Manager) updateHealthCondition(uid string, isHealthy bool) error {
	logrus.Infof("uid: %s, isHealthy: %t", uid, isHealthy)
	ns, name, server, err := unMarshalUID(uid)
	if err != nil {
		return err
	}

	eps, err := m.endpointSliceCache.Get(ns, name)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("get cache of endpointslice %s/%s failed, error: %w", ns, name, err)
	} else if errors.IsNotFound(err) {
		logrus.Warnf("endpointSlice %s/%s not found", ns, name)
		return nil
	}

	for i, ep := range eps.Endpoints {
		if len(ep.Addresses) != 1 {
			return fmt.Errorf("the length of addresses is not 1, endpoint: %+v", ep)
		}
		if ep.Addresses[0] == server {
			epsCopy := eps.DeepCopy()
			epsCopy.Endpoints[i].Conditions = discoveryv1.EndpointConditions{Ready: &isHealthy}
			if _, err := m.endpointSliceClient.Update(epsCopy); err != nil {
				return fmt.Errorf("update status of endpointslice %s/%s failed, error: %w", ns, name, err)
			}
		}
	}

	return nil
}

func (m *Manager) EnsureLoadBalancer(lb *lbv1.LoadBalancer) error {
	if err := m.ensureService(lb); err != nil {
		return fmt.Errorf("ensure service failed, error: %w", err)
	}

	eps, err := m.ensureEndpointSlice(lb)
	if err != nil {
		return fmt.Errorf("ensure endpointslice failed, error: %w", err)
	}

	m.ensureProbes(lb, eps)

	return nil
}

func (m *Manager) DeleteLoadBalancer(lb *lbv1.LoadBalancer) error {
	eps, err := m.endpointSliceCache.Get(lb.Namespace, lb.Name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		logrus.Warnf("endpointSlice %s/%s not found", lb.Namespace, lb.Name)
		return nil
	}

	m.removeProbes(lb, eps)

	return nil
}

func (m *Manager) GetBackendServers(lb *lbv1.LoadBalancer) ([]pkglb.BackendServer, error) {
	eps, err := m.endpointSliceCache.Get(lb.Namespace, lb.Name)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	} else if errors.IsNotFound(err) {
		logrus.Warnf("endpointSlice %s/%s not found", lb.Namespace, lb.Name)
		return []pkglb.BackendServer{}, nil
	}

	servers := make([]pkglb.BackendServer, 0, len(eps.Endpoints))
	for _, ep := range eps.Endpoints {
		vmi, err := m.vmiCache.Get(ep.TargetRef.Namespace, ep.TargetRef.Name)
		if err != nil {
			return nil, err
		}

		servers = append(servers, &Server{vmi})
	}

	return servers, nil
}

func (m *Manager) AddBackendServers(lb *lbv1.LoadBalancer, servers []pkglb.BackendServer) (bool, error) {
	eps, err := m.endpointSliceCache.Get(lb.Namespace, lb.Name)
	if err != nil && !errors.IsNotFound(err) {
		return false, err
	} else if errors.IsNotFound(err) {
		logrus.Warnf("endpointSlice %s/%s not found", lb.Namespace, lb.Name)
		return false, nil
	}

	var epsCopy *discoveryv1.EndpointSlice
	endpoints := make([]*discoveryv1.Endpoint, 0, len(servers))
	for _, server := range servers {
		if flag, _, err := isExisting(eps, server); err != nil || flag {
			continue
		}

		address, ok := server.GetAddress()
		if !ok {
			continue
		}

		endpoint := discoveryv1.Endpoint{
			Addresses: []string{address},
			TargetRef: &corev1.ObjectReference{
				Namespace: server.GetNamespace(),
				Name:      server.GetName(),
				UID:       server.GetUID(),
			},
			Conditions: discoveryv1.EndpointConditions{
				Ready: pointer.Bool(true),
			},
		}
		if epsCopy == nil {
			epsCopy = eps.DeepCopy()
		}
		epsCopy.Endpoints = append(epsCopy.Endpoints, endpoint)
		endpoints = append(endpoints, &endpoint)
	}

	if epsCopy != nil {
		if _, err := m.endpointSliceClient.Update(epsCopy); err != nil {
			return false, err
		}
	}

	if lb.Spec.HealthCheck != nil && lb.Spec.HealthCheck.Port != 0 {
		for _, endpoint := range endpoints {
			m.addOneProbe(lb, endpoint)
		}
	}

	return epsCopy != nil, nil
}

func (m *Manager) RemoveBackendServers(lb *lbv1.LoadBalancer, servers []pkglb.BackendServer) (bool, error) {
	eps, err := m.endpointSliceCache.Get(lb.Namespace, lb.Name)
	if err != nil && !errors.IsNotFound(err) {
		return false, err
	} else if errors.IsNotFound(err) {
		logrus.Warnf("endpointSlice %s/%s not found", lb.Namespace, lb.Name)
		return false, nil
	}

	indexes := make([]int, 0)
	for _, server := range servers {
		flag, index, err := isExisting(eps, server)
		if err != nil || !flag {
			continue
		}
		indexes = append(indexes, index)
	}

	if len(indexes) == 0 {
		return false, nil
	}

	epsCopy := eps.DeepCopy()
	epsCopy.Endpoints = make([]discoveryv1.Endpoint, 0)
	preIndex := -1
	for _, index := range indexes {
		epsCopy.Endpoints = append(epsCopy.Endpoints, eps.Endpoints[preIndex+1:index]...)
		preIndex = index
	}
	epsCopy.Endpoints = append(epsCopy.Endpoints, eps.Endpoints[preIndex+1:]...)
	if _, err = m.endpointSliceClient.Update(epsCopy); err != nil {
		return false, err
	}

	if lb.Spec.HealthCheck != nil && lb.Spec.HealthCheck.Port != 0 {
		for _, i := range indexes {
			m.removeOneProbe(lb, &eps.Endpoints[i])
		}
	}

	return true, nil
}

// Check whether the server is existing in the endpointSlice
func isExisting(eps *discoveryv1.EndpointSlice, server pkglb.BackendServer) (bool, int, error) {
	address, ok := server.GetAddress()
	if !ok {
		return false, 0, fmt.Errorf("could not get address of server %s/%s", server.GetNamespace(), server.GetName())
	}

	for i, ep := range eps.Endpoints {
		if len(ep.Addresses) != 1 {
			return false, 0, fmt.Errorf("the length of addresses is not 1, endpoint: %+v", ep)
		}

		if ep.TargetRef != nil && server.GetUID() == ep.TargetRef.UID && address == ep.Addresses[0] {
			return true, i, nil
		}
	}

	return false, 0, nil
}

func (m *Manager) ensureProbes(lb *lbv1.LoadBalancer, eps *discoveryv1.EndpointSlice) {
	if lb.Spec.HealthCheck == nil || lb.Spec.HealthCheck.Port == 0 {
		m.removeProbes(lb, eps)
		return
	}

	for i := range eps.Endpoints {
		m.addOneProbe(lb, &eps.Endpoints[i])
	}
}

func (m *Manager) addOneProbe(lb *lbv1.LoadBalancer, ep *discoveryv1.Endpoint) {
	if len(ep.Addresses) == 0 {
		return
	}

	option := prober.HealthOption{
		Address:          ep.Addresses[0] + ":" + strconv.Itoa(int(lb.Spec.HealthCheck.Port)),
		InitialCondition: true,
	}
	if lb.Spec.HealthCheck.SuccessThreshold == 0 {
		option.SuccessThreshold = defaultSuccessThreshold
	} else {
		option.SuccessThreshold = lb.Spec.HealthCheck.SuccessThreshold
	}
	if lb.Spec.HealthCheck.FailureThreshold == 0 {
		option.FailureThreshold = defaultFailureThreshold
	} else {
		option.FailureThreshold = lb.Spec.HealthCheck.FailureThreshold
	}
	if lb.Spec.HealthCheck.TimeoutSeconds == 0 {
		option.Timeout = defaultTimeout
	} else {
		option.Timeout = time.Duration(lb.Spec.HealthCheck.TimeoutSeconds) * time.Second
	}
	if lb.Spec.HealthCheck.PeriodSeconds == 0 {
		option.Period = defaultPeriod
	} else {
		option.Period = time.Duration(lb.Spec.HealthCheck.PeriodSeconds) * time.Second
	}
	if ep.Conditions.Ready != nil {
		option.InitialCondition = *ep.Conditions.Ready
	}

	uid := marshalUID(lb.Namespace, lb.Name, ep.Addresses[0])

	m.AddWorker(uid, option)
}

func (m *Manager) removeProbes(lb *lbv1.LoadBalancer, eps *discoveryv1.EndpointSlice) {
	for i := range eps.Endpoints {
		m.removeOneProbe(lb, &eps.Endpoints[i])
	}
}

func (m *Manager) removeOneProbe(lb *lbv1.LoadBalancer, ep *discoveryv1.Endpoint) {
	if len(ep.Addresses) == 0 {
		return
	}
	uid := marshalUID(lb.Namespace, lb.Name, ep.Addresses[0])
	m.RemoveWorker(uid)
}

func unMarshalUID(uid string) (namespace, name, server string, err error) {
	fields := strings.Split(uid, "/")
	if len(fields) != 3 {
		err = fmt.Errorf("invalid uid %s", uid)
		return
	}
	namespace, name, server = fields[0], fields[1], fields[2]
	return
}

func marshalUID(ns, name, server string) (uid string) {
	uid = ns + "/" + name + "/" + server
	return
}

func (m *Manager) ensureService(lb *lbv1.LoadBalancer) error {
	svc, err := m.serviceCache.Get(lb.Namespace, lb.Name)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("get cache of service %s/%s failed, error: %w", lb.Namespace, lb.Name, err)
	} else if err == nil {
		svc = constructService(svc, lb)
		if _, err := m.serviceClient.Update(svc.DeepCopy()); err != nil {
			return fmt.Errorf("update service %s/%s failed, error: %w", lb.Namespace, lb.Name, err)
		}
	} else {
		svc = constructService(nil, lb)
		if _, err := m.serviceClient.Create(svc); err != nil {
			return fmt.Errorf("create service %s/%s failed, error: %w", lb.Namespace, lb.Name, err)
		}
	}

	return nil
}

func constructService(cur *corev1.Service, lb *lbv1.LoadBalancer) *corev1.Service {
	svc := &corev1.Service{}
	if cur != nil {
		svc = cur
	} else {
		svc.Name = lb.Name
		svc.Namespace = lb.Namespace
		svc.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: lb.APIVersion,
			Kind:       lb.Kind,
			Name:       lb.Name,
			UID:        lb.UID,
		}}
		svc.Labels = map[string]string{
			KeyLabel: utils.ValueTrue,
		}
		svc.Spec.Type = corev1.ServiceTypeLoadBalancer
	}

	if lb.Spec.IPAM == lbv1.DHCP {
		// Kube-vip gets external IP for service of type LoadBalancer by DHCP if LoadBalancerIP is set as "0.0.0.0"
		svc.Spec.LoadBalancerIP = utils.Address4AskDHCP
	} else {
		svc.Spec.LoadBalancerIP = lb.Status.AllocatedAddress.IP
	}

	ports := make([]corev1.ServicePort, 0, len(lb.Spec.Listeners))
	for _, listener := range lb.Spec.Listeners {
		port := corev1.ServicePort{
			Name:       listener.Name,
			Protocol:   listener.Protocol,
			Port:       listener.Port,
			TargetPort: intstr.IntOrString{IntVal: listener.BackendPort},
		}
		ports = append(ports, port)
	}
	svc.Spec.Ports = ports

	return svc
}

func (m *Manager) ensureEndpointSlice(lb *lbv1.LoadBalancer) (*discoveryv1.EndpointSlice, error) {
	eps, err := m.endpointSliceCache.Get(lb.Namespace, lb.Name)
	if err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("get cache of endpointslice %s/%s failed, error: %w", lb.Namespace, lb.Name, err)
	} else if err == nil {
		eps, err = m.constructEndpointSlice(eps, lb)
		if err != nil {
			return nil, err
		}
		if eps, err = m.endpointSliceClient.Update(eps); err != nil {
			return nil, fmt.Errorf("update endpointslice %s/%s failed, error: %w", lb.Namespace, lb.Name, err)
		}
	} else {
		eps, err = m.constructEndpointSlice(nil, lb)
		if err != nil {
			return nil, err
		}
		if eps, err = m.endpointSliceClient.Create(eps); err != nil {
			return nil, fmt.Errorf("create endpointslice %s/%s failed, error: %w", lb.Namespace, lb.Name, err)
		}
	}

	return eps, nil
}

func (m *Manager) constructEndpointSlice(cur *discoveryv1.EndpointSlice, lb *lbv1.LoadBalancer) (*discoveryv1.EndpointSlice, error) {
	eps := &discoveryv1.EndpointSlice{}
	if cur != nil {
		eps = cur.DeepCopy()
	} else {
		eps.Namespace = lb.Namespace
		eps.Name = lb.Name
		eps.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: lb.APIVersion,
				Kind:       lb.Kind,
				Name:       lb.Name,
				UID:        lb.UID,
			}}
		eps.Labels = map[string]string{
			KeyLabel:       utils.ValueTrue,
			KeyServiceName: lb.Name,
		}
		eps.AddressType = discoveryv1.AddressTypeIPv4
	}

	ports := make([]discoveryv1.EndpointPort, 0, len(lb.Spec.Listeners))
	for i := range lb.Spec.Listeners {
		port := discoveryv1.EndpointPort{
			Name:     &(lb.Spec.Listeners[i].Name),
			Protocol: &(lb.Spec.Listeners[i].Protocol),
			Port:     &(lb.Spec.Listeners[i].BackendPort),
		}
		ports = append(ports, port)
	}
	eps.Ports = ports

	// It's necessary to reserve the condition of old endpoints.
	var endpoints []discoveryv1.Endpoint
	var flag bool
	servers, err := m.matchBackendServers(lb)
	if err != nil {
		return nil, err
	}

	for _, server := range servers {
		flag = false
		address, ok := server.GetAddress()
		if !ok {
			continue
		}
		for _, ep := range eps.Endpoints {
			if len(ep.Addresses) != 1 {
				return nil, fmt.Errorf("the length of addresses is not 1, endpoint: %+v", ep)
			}

			if ep.TargetRef != nil && server.GetUID() == ep.TargetRef.UID && address == ep.Addresses[0] {
				flag = true
				// add the existing endpoint
				endpoints = append(endpoints, ep)
				break
			}
		}
		// add the non-existing endpoint
		if !flag {
			endpoint := discoveryv1.Endpoint{
				Addresses: []string{address},
				TargetRef: &corev1.ObjectReference{
					Namespace: server.GetNamespace(),
					Name:      server.GetName(),
					UID:       server.GetUID(),
				},
				Conditions: discoveryv1.EndpointConditions{
					Ready: pointer.Bool(true),
				},
			}
			endpoints = append(endpoints, endpoint)
		}
	}
	eps.Endpoints = endpoints

	logrus.Infoln("constructEndpointSlice: ", eps)

	return eps, nil
}

// Find the VMIs which match the selector
func (m *Manager) matchBackendServers(lb *lbv1.LoadBalancer) ([]pkglb.BackendServer, error) {
	if len(lb.Spec.BackendServerSelector) == 0 {
		return []pkglb.BackendServer{}, nil
	}
	selector, err := utils.NewSelector(lb.Spec.BackendServerSelector)
	if err != nil {
		return nil, err
	}
	// limit the lb to the same namespace of the workload
	vmis, err := m.vmiCache.List(lb.Namespace, selector)
	if err != nil {
		return nil, err
	}

	servers := make([]pkglb.BackendServer, len(vmis))

	for i, vmi := range vmis {
		servers[i] = &Server{vmi}
	}

	return servers, nil
}
