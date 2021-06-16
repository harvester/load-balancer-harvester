package servicelb

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	ctlCorev1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1alpha1"
	ctlDiscoveryv1 "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/discovery.k8s.io/v1beta1"
	"github.com/harvester/harvester-load-balancer/pkg/prober"
)

const (
	KeyLabel        = "loadbalancer.harvesterhci.io/servicelb"
	KeyServiceName  = "kubernetes.io/service-name"
	Address4AskDHCP = "0.0.0.0"
)

type Manager struct {
	serviceClient       ctlCorev1.ServiceClient
	endpointSliceClient ctlDiscoveryv1.EndpointSliceClient
	*prober.Manager
}

func NewManager(ctx context.Context, serviceClient *ctlCorev1.ServiceClient, endpointSliceClient *ctlDiscoveryv1.EndpointSliceClient) *Manager {
	m := &Manager{
		serviceClient:       *serviceClient,
		endpointSliceClient: *endpointSliceClient,
	}
	m.Manager = prober.NewManager(ctx, m.updateHealthCondition)

	return m
}

func (m *Manager) updateHealthCondition(uid string, isHealthy bool) error {
	klog.Infof("uid: %s, isHealthy: %t", uid, isHealthy)
	ns, name, server, err := unMarshalUID(uid)
	if err != nil {
		return err
	}

	eps, err := m.endpointSliceClient.Get(ns, name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	for i, ep := range eps.Endpoints {
		if len(ep.Addresses) != 1 {
			return fmt.Errorf("the length of addresses is not 1, endpoint: %+v", ep)
		}
		if ep.Addresses[0] == server {
			epsCopy := eps.DeepCopy()
			epsCopy.Endpoints[i].Conditions = discoveryv1.EndpointConditions{Ready: &isHealthy}
			_, err := m.endpointSliceClient.Update(epsCopy)
			return err
		}
	}

	return nil
}

func (m *Manager) EnsureLoadBalancer(lb *lbv1.LoadBalancer) error {
	if err := m.ensureService(lb); err != nil {
		return err
	}
	if err := m.ensureEndpointSlice(lb); err != nil {
		return err
	}

	return m.ensureProber(lb)
}

func (m *Manager) DeleteLoadBalancer(lb *lbv1.LoadBalancer) {
	m.removeProber(lb)
}

func (m *Manager) ensureProber(lb *lbv1.LoadBalancer) error {
	if lb.Spec.HeathCheck == nil {
		m.removeProber(lb)
		return nil
	}
	option := prober.HealthOption{
		SuccessThreshold: lb.Spec.HeathCheck.SuccessThreshold,
		FailureThreshold: lb.Spec.HeathCheck.FailureThreshold,
		Timeout:          time.Duration(lb.Spec.HeathCheck.TimeoutSeconds) * time.Second,
		Period:           time.Duration(lb.Spec.HeathCheck.PeriodSeconds) * time.Second,
		InitialCondition: true,
	}

	eps, err := m.endpointSliceClient.Get(lb.Namespace, lb.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	existingEpUIDs := map[string]bool{}

	for _, ep := range eps.Endpoints {
		if len(ep.Addresses) != 1 {
			return fmt.Errorf("the length of addresses is not 1, endpoint: %+v", ep)
		}
		option.Address = ep.Addresses[0] + ":" + strconv.Itoa(lb.Spec.HeathCheck.Port)
		uid := marshalUID(lb.Namespace, lb.Name, ep.Addresses[0])
		existingEpUIDs[uid] = true
		if ep.Conditions.Ready != nil {
			option.InitialCondition = *ep.Conditions.Ready
		}
		m.AddWorker(uid, option)
	}

	// remove the outdated workers
	workerMap := m.ListWorkers()
	for uid := range workerMap {
		if !existingEpUIDs[uid] {
			m.RemoveWorker(uid)
		}
	}

	return nil
}

func (m *Manager) removeProber(lb *lbv1.LoadBalancer) {
	for _, server := range lb.Spec.BackendServers {
		uid := marshalUID(lb.Namespace, lb.Name, server)
		m.RemoveWorker(uid)
	}
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
	svc, err := m.serviceClient.Get(lb.Namespace, lb.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if err == nil {
		svc = constructService(svc, lb)
		_, err = m.serviceClient.Update(svc.DeepCopy())
		return err
	} else {
		svc = constructService(nil, lb)
		_, err := m.serviceClient.Create(svc)
		return err
	}
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
			KeyLabel: "true",
		}
	}

	if lb.Spec.Type == lbv1.Internal {
		svc.Spec.Type = corev1.ServiceTypeClusterIP
	} else {
		svc.Spec.Type = corev1.ServiceTypeLoadBalancer
		if len(svc.Status.LoadBalancer.Ingress) == 0 {
			// Kube-vip gets external IP for service of type LoadBalancer by DHCP if LoadBalancerIP is set as "0.0.0.0"
			svc.Spec.LoadBalancerIP = Address4AskDHCP
		}
	}

	ports := []corev1.ServicePort{}
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

func (m *Manager) ensureEndpointSlice(lb *lbv1.LoadBalancer) error {
	eps, err := m.endpointSliceClient.Get(lb.Namespace, lb.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if err == nil {
		eps, err = constructEndpointSlice(eps, lb)
		if err != nil {
			return err
		}
		_, err = m.endpointSliceClient.Update(eps)
		return err
	} else {
		eps, err = constructEndpointSlice(nil, lb)
		if err != nil {
			return err
		}
		_, err := m.endpointSliceClient.Create(eps)
		return err
	}
}

func constructEndpointSlice(cur *discoveryv1.EndpointSlice, lb *lbv1.LoadBalancer) (*discoveryv1.EndpointSlice, error) {
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
			KeyLabel:       "true",
			KeyServiceName: lb.Name,
		}
		eps.AddressType = discoveryv1.AddressTypeIPv4
	}

	ports := []discoveryv1.EndpointPort{}
	for _, listener := range lb.Spec.Listeners {
		port := discoveryv1.EndpointPort{
			Name:     &listener.Name,
			Protocol: &listener.Protocol,
			Port:     &listener.BackendPort,
		}
		ports = append(ports, port)
	}
	eps.Ports = ports

	//it's necessary to reserve the condition of old endpoints.
	var endpoints []discoveryv1.Endpoint
	var flag bool
	for _, server := range lb.Spec.BackendServers {
		flag = false
		for _, ep := range eps.Endpoints {
			if len(ep.Addresses) != 1 {
				return nil, fmt.Errorf("the length of addresses is not 1, endpoint: %+v", ep)
			}
			if server == ep.Addresses[0] {
				flag = true
				//add the existing endpoint
				endpoints = append(endpoints, ep)
				break
			}
		}
		// add the non-existing endpoint
		if !flag {
			ready := true
			endpoint := discoveryv1.Endpoint{
				Addresses: []string{server},
				Conditions: discoveryv1.EndpointConditions{
					Ready: &ready,
				},
			}
			endpoints = append(endpoints, endpoint)
		}
	}
	eps.Endpoints = endpoints

	return eps, nil
}
