package servicelb

import (
	"context"
	"fmt"
	"reflect"
	"slices"
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
	healthHandler       pkglb.HealthCheckHandler
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

func (m *Manager) updateHealthCondition(uid, address string, isHealthy bool) error {
	ns, name, err := unMarshalUID(uid)
	if err != nil {
		return err
	}

	eps, err := m.endpointSliceCache.Get(ns, name)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("fail to get endpointslice %s/%s, error: %w", ns, name, err)
	} else if errors.IsNotFound(err) {
		logrus.Warnf("endpointSlice %s/%s is not found", ns, name)
		return nil
	}

	ip, _, err := unMarshalPorberAddress(address)
	if err != nil {
		return err
	}

	for i := range eps.Endpoints {
		if len(eps.Endpoints[i].Addresses) != 1 {
			return fmt.Errorf("the length of lb %s endpoint addresses is %v, endpoint: %+v", uid, len(eps.Endpoints[i].Addresses), eps.Endpoints[i])
		}
		if eps.Endpoints[i].Addresses[0] == ip {
			// only update the Ready condition when necessary
			if needUpdateEndpointConditions(&eps.Endpoints[i].Conditions, isHealthy) {
				// notify controller that some endpoint conditions change ( success <---> fail)
				// otherweise, the controller needs to watch all endpointslice object to know the health probe result on time
				// or the controller actively loop Enqueue all lbs which enables health check
				if m.healthHandler != nil {
					if err := m.healthHandler(ns, name); err != nil {
						return fmt.Errorf("fail to notify lb %s, error: %w", uid, err)
					}
				}
				epsCopy := eps.DeepCopy()
				updateEndpointConditions(&epsCopy.Endpoints[i].Conditions, isHealthy)
				logrus.Infof("update condition of lb %s endpoint ip %s to %t", uid, ip, isHealthy)
				if _, err := m.endpointSliceClient.Update(epsCopy); err != nil {
					return fmt.Errorf("fail to update condition of lb %s endpoint ip %s to %t, error: %w", uid, ip, isHealthy, err)
				}
			}
			break
		}
	}

	return nil
}

func isEndpointConditionsReady(ec *discoveryv1.EndpointConditions) bool {
	return ec.Ready != nil && *ec.Ready
}

func needUpdateEndpointConditions(ec *discoveryv1.EndpointConditions, ready bool) bool {
	return ec.Ready == nil || *ec.Ready != ready
}

func updateEndpointConditions(ec *discoveryv1.EndpointConditions, ready bool) {
	if ec.Ready == nil {
		v := ready
		ec.Ready = &v
	} else if *ec.Ready != ready {
		*ec.Ready = ready
	}
}

func (m *Manager) updateAllConditions(lb *lbv1.LoadBalancer, eps *discoveryv1.EndpointSlice, isHealthy bool) error {
	epsCopy := eps.DeepCopy()
	updated := false
	for i := range epsCopy.Endpoints {
		if !isDummyEndpoint(&epsCopy.Endpoints[i]) && needUpdateEndpointConditions(&epsCopy.Endpoints[i].Conditions, isHealthy) {
			updateEndpointConditions(&epsCopy.Endpoints[i].Conditions, isHealthy)
			updated = true
		}
	}

	if updated {
		logrus.Infof("update all conditions of lb %s/%s endpoints to %t", lb.Namespace, lb.Name, isHealthy)
		if _, err := m.endpointSliceClient.Update(epsCopy); err != nil {
			return fmt.Errorf("fail to update all conditions of lb %s/%s endpoints to %t error: %w", lb.Namespace, lb.Name, isHealthy, err)
		}
	}
	return nil
}

// if probe is disabled, then return the endpint count
func (m *Manager) GetProbeReadyBackendServerCount(lb *lbv1.LoadBalancer) (int, error) {
	eps, err := m.endpointSliceCache.Get(lb.Namespace, lb.Name)
	if err != nil && !errors.IsNotFound(err) {
		return 0, err
	} else if errors.IsNotFound(err) {
		logrus.Warnf("lb %s/%s endpointSlice is not found", lb.Namespace, lb.Name)
		return 0, err
	}

	count := 0
	// if use `for _, ep := range eps.Endpoints`
	// get: G601: Implicit memory aliasing in for loop. (gosec)
	for i := range eps.Endpoints {
		if !isDummyEndpoint(&eps.Endpoints[i]) && isEndpointConditionsReady(&eps.Endpoints[i].Conditions) {
			count++
		}
	}

	return count, nil
}

func (m *Manager) EnsureLoadBalancer(lb *lbv1.LoadBalancer) error {
	return m.ensureService(lb)
}

// call only once before controller starts looping on OnChange ...
func (m *Manager) RegisterHealthCheckHandler(handler pkglb.HealthCheckHandler) error {
	if m.healthHandler != nil {
		return fmt.Errorf("health check handler can only be registered once")
	}
	m.healthHandler = handler
	return nil
}

func (m *Manager) DeleteLoadBalancer(lb *lbv1.LoadBalancer) error {
	_, err := m.endpointSliceCache.Get(lb.Namespace, lb.Name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		logrus.Infof("delete lb %s/%s endpointSlice is not found", lb.Namespace, lb.Name)
		// no harm to call the following removeLBProbers
	}

	_, err = m.removeLBProbers(lb)
	return err
}

// get the qualified backend servers of one LB
func (m *Manager) getServiceBackendServers(lb *lbv1.LoadBalancer) ([]pkglb.BackendServer, error) {
	// if user does not set the selector, then return nil
	if len(lb.Spec.BackendServerSelector) == 0 {
		return nil, nil
	}
	// get related vmis
	selector, err := utils.NewSelector(lb.Spec.BackendServerSelector)
	if err != nil {
		return nil, fmt.Errorf("fail to new selector, error: %w", err)
	}
	vmis, err := m.vmiCache.List(lb.Namespace, selector)
	if err != nil {
		return nil, fmt.Errorf("fail to list vmi per selector, error: %w", err)
	}

	servers := make([]pkglb.BackendServer, 0, len(vmis))

	// skip being-deleted vmi, no-address vmi
	for _, vmi := range vmis {
		if vmi.DeletionTimestamp != nil {
			continue
		}
		server := &Server{VirtualMachineInstance: vmi}
		if _, ok := server.GetAddress(); ok {
			servers = append(servers, server)
		}
	}
	return servers, nil
}

func (m *Manager) EnsureBackendServers(lb *lbv1.LoadBalancer) ([]pkglb.BackendServer, error) {
	// ensure service is existing
	if svc, err := m.getService(lb); err != nil {
		return nil, err
	} else if svc == nil {
		return nil, fmt.Errorf("service is not existing, ensure it first")
	}

	var eps *discoveryv1.EndpointSlice
	eps, err := m.endpointSliceCache.Get(lb.Namespace, lb.Name)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("fail to get endpointslice, error: %w", err)
		}
		eps = nil
	}

	servers, err := m.getServiceBackendServers(lb)
	if err != nil {
		return nil, err
	}

	epsNew, err := m.constructEndpointSliceFromBackendServers(eps, lb, servers)
	if err != nil {
		return nil, err
	}

	// create a new one
	if eps == nil {
		// it is ok to do not check IsAlreadyExists, reconciler will pass
		eps, err = m.endpointSliceClient.Create(epsNew)
		if err != nil {
			return nil, fmt.Errorf("fail to create endpointslice, error: %w", err)
		}
	} else {
		if !reflect.DeepEqual(eps, epsNew) {
			logrus.Debugf("update endpointslice %s/%s", lb.Namespace, lb.Name)
			eps, err = m.endpointSliceClient.Update(epsNew)
			if err != nil {
				return nil, fmt.Errorf("fail to update endpointslice, error: %w", err)
			}
		}
	}

	// always ensure probs
	if err := m.ensureProbes(lb, eps); err != nil {
		return nil, fmt.Errorf("fail to ensure probs, error: %w", err)
	}

	// always ensure dummy endpoint
	if err := m.ensureDummyEndpoint(lb, eps); err != nil {
		return nil, fmt.Errorf("fail to ensure dummy endpointslice, error: %w", err)
	}

	return servers, nil
}

func (m *Manager) EnsureLoadBalancerServiceIP(lb *lbv1.LoadBalancer) (string, error) {
	// ensure service is existing
	svc, err := m.getService(lb)
	if err != nil {
		return "", err
	}
	if svc == nil {
		return "", fmt.Errorf("service is not existing, ensure it first")
	}
	// kube-vip will update this field, wait if not existing
	if len(svc.Status.LoadBalancer.Ingress) > 0 {
		return svc.Status.LoadBalancer.Ingress[0].IP, nil
	}
	// no ip, wait
	return "", pkglb.ErrWaitExternalIP
}

func (m *Manager) ListBackendServers(lb *lbv1.LoadBalancer) ([]pkglb.BackendServer, error) {
	return m.getServiceBackendServers(lb)
}

func (m *Manager) ensureProbes(lb *lbv1.LoadBalancer, eps *discoveryv1.EndpointSlice) error {
	// disabled
	if lb.Spec.HealthCheck == nil || lb.Spec.HealthCheck.Port == 0 {
		if _, err := m.removeLBProbers(lb); err != nil {
			return err
		}
		// user may disable the healthy checker e.g. it is not working as expected
		// then set all endpoints to be Ready thus they can continue to work
		if err := m.updateAllConditions(lb, eps, true); err != nil {
			return err
		}
		return nil
	}

	uid := marshalUID(lb.Namespace, lb.Name)
	targetProbers := make(map[string]prober.HealthOption)
	// indexing to skip G601 in go v121
	for i := range eps.Endpoints {
		if len(eps.Endpoints[i].Addresses) == 0 || isDummyEndpoint(&eps.Endpoints[i]) {
			continue
		}
		targetProbers[marshalPorberAddress(lb, &eps.Endpoints[i])] = m.generateOneProber(lb, &eps.Endpoints[i])
	}

	// get a copy of data for safe operation
	activeProbers, _ := m.GetWorkerHealthOptionMap(uid)

	return m.updateAllProbers(uid, activeProbers, targetProbers)
}

// according to the activeProbers and targetProbers
//
//	keep unchanged
//	remove out-dated
//	add newly observed
func (m *Manager) updateAllProbers(uid string, activeProbers, targetProbers map[string]prober.HealthOption) error {
	for _, ap := range activeProbers {
		if tp, ok := targetProbers[ap.Address]; ok {
			// find in both
			if !ap.Equal(tp) {
				// changed
				// remove active one
				if _, err := m.RemoveWorker(uid, ap.Address); err != nil {
					return err
				}
				// add target one
				if err := m.AddWorker(uid, tp.Address, tp); err != nil {
					return err
				}
			}
			// replaced or equal; then delete it from both maps
			delete(activeProbers, ap.Address)
			delete(targetProbers, tp.Address)
		}
		// for those not found in the targetProbers, will be processed in next lines
	}

	// remove all remainings of activeProbers
	for _, ap := range activeProbers {
		logrus.Debugf("-probe %s %s", uid, ap.Address)
		if _, err := m.RemoveWorker(uid, ap.Address); err != nil {
			return err
		}
	}

	// add all remainings of targetProbers
	for _, tp := range targetProbers {
		logrus.Debugf("+probe %s %s", uid, tp.Address)
		if err := m.AddWorker(uid, tp.Address, tp); err != nil {
			return err
		}
	}

	return nil
}

// without at least one Ready (dummy) endpoint, the service may route traffic to local host
func (m *Manager) ensureDummyEndpoint(lb *lbv1.LoadBalancer, eps *discoveryv1.EndpointSlice) error {
	dummyCount := 0
	activeCount := 0
	// if use `for _, ep := range eps.Endpoints`
	// get: G601: Implicit memory aliasing in for loop. (gosec)
	for i := range eps.Endpoints {
		if isDummyEndpoint(&eps.Endpoints[i]) {
			dummyCount++
		} else if isEndpointConditionsReady(&eps.Endpoints[i].Conditions) {
			activeCount++
		}
	}

	// add the dummy endpoint
	if activeCount == 0 && dummyCount == 0 {
		epsCopy := eps.DeepCopy()
		epsCopy.Endpoints = appendDummyEndpoint(epsCopy.Endpoints, lb)
		if _, err := m.endpointSliceClient.Update(epsCopy); err != nil {
			return fmt.Errorf("fail to append dummy endpoint to lb %v endpoint, error: %w", lb.Name, err)
		}
		return nil
	}

	// remove the dummy endpoint
	if activeCount > 0 && dummyCount > 0 {
		epsCopy := eps.DeepCopy()
		epsCopy.Endpoints = slices.DeleteFunc(epsCopy.Endpoints, func(ep discoveryv1.Endpoint) bool {
			return ep.TargetRef.UID == dummyEndpointID
		})
		if _, err := m.endpointSliceClient.Update(epsCopy); err != nil {
			return fmt.Errorf("fail to remove dummy endpoint from lb %v endpoint, error: %w", lb.Name, err)
		}
		return nil
	}

	return nil
}

func (m *Manager) removeLBProbers(lb *lbv1.LoadBalancer) (int, error) {
	return m.RemoveWorkersByUid(marshalUID(lb.Namespace, lb.Name))
}

func (m *Manager) generateOneProber(lb *lbv1.LoadBalancer, ep *discoveryv1.Endpoint) prober.HealthOption {
	option := prober.HealthOption{
		Address:          marshalPorberAddress(lb, ep),
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
	return option
}

// the prober.Manager is managed per uid
// lb namespace/name is a qualified group uid
func marshalUID(ns, name string) (uid string) {
	uid = ns + "/" + name
	return
}

func unMarshalUID(uid string) (namespace, name string, err error) {
	fields := strings.Split(uid, "/")
	if len(fields) != 2 {
		err = fmt.Errorf("invalid uid %s", uid)
		return
	}
	namespace, name = fields[0], fields[1]
	return
}

func marshalPorberAddress(lb *lbv1.LoadBalancer, ep *discoveryv1.Endpoint) string {
	return ep.Addresses[0] + ":" + strconv.Itoa(int(lb.Spec.HealthCheck.Port))
}

// probe address is like: 10.52.0.214:80
func unMarshalPorberAddress(address string) (ip, port string, err error) {
	fields := strings.Split(address, ":")
	if len(fields) != 2 {
		err = fmt.Errorf("invalid probe address %s", address)
		return
	}
	ip, port = fields[0], fields[1]
	return
}

func (m *Manager) getService(lb *lbv1.LoadBalancer) (*corev1.Service, error) {
	svc, err := m.serviceCache.Get(lb.Namespace, lb.Name)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("fail to get service, error: %w", err)
		}
		return nil, nil
	}
	return svc, nil
}

func (m *Manager) ensureService(lb *lbv1.LoadBalancer) error {
	svc, err := m.getService(lb)
	if err != nil {
		return err
	}

	if svc != nil {
		svcCopy := svc.DeepCopy()
		svcCopy = constructService(svcCopy, lb)
		if !reflect.DeepEqual(svc, svcCopy) {
			if _, err := m.serviceClient.Update(svcCopy); err != nil {
				return fmt.Errorf("fail to update service, error: %w", err)
			}
		}
	} else {
		svc = constructService(nil, lb)
		if _, err := m.serviceClient.Create(svc); err != nil && !errors.IsAlreadyExists(err) {
			return fmt.Errorf("fail to create service, error: %w", err)
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

const dummyEndpointIPv4Address = "10.52.0.255"
const dummyEndpointID = "dummy347-546a-4642-9da6-5608endpoint"

func appendDummyEndpoint(eps []discoveryv1.Endpoint, lb *lbv1.LoadBalancer) []discoveryv1.Endpoint {
	endpoint := discoveryv1.Endpoint{
		Addresses: []string{dummyEndpointIPv4Address},
		TargetRef: &corev1.ObjectReference{
			Namespace: lb.Namespace,
			Name:      lb.Name,
			UID:       dummyEndpointID,
		},
		Conditions: discoveryv1.EndpointConditions{
			Ready: pointer.Bool(true),
		},
	}
	eps = append(eps, endpoint)
	return eps
}

func isDummyEndpoint(ep *discoveryv1.Endpoint) bool {
	return ep.TargetRef.UID == dummyEndpointID
}

func (m *Manager) constructEndpointSliceFromBackendServers(cur *discoveryv1.EndpointSlice, lb *lbv1.LoadBalancer, servers []pkglb.BackendServer) (*discoveryv1.EndpointSlice, error) {
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
	var existing bool

	for _, server := range servers {
		existing = false
		// already checked when getting servers, but keep to take care of history data
		address, ok := server.GetAddress()
		if !ok {
			continue
		}
		for _, ep := range eps.Endpoints {
			if len(ep.Addresses) != 1 {
				return nil, fmt.Errorf("the length of addresses is not 1, endpoint: %+v", ep)
			}

			if ep.TargetRef != nil && server.GetUID() == ep.TargetRef.UID && address == ep.Addresses[0] {
				existing = true
				// add the existing endpoint
				endpoints = append(endpoints, ep)
				break
			}
		}
		// add the non-existing endpoint
		if !existing {
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
	// a dummy endpoint avoids the LB traffic is routed to other services/local host accidentally
	if len(endpoints) == 0 {
		endpoints = appendDummyEndpoint(endpoints, lb)
	}
	eps.Endpoints = endpoints

	logrus.Debugln("constructEndpointSliceFromBackendServers: ", eps)

	return eps, nil
}
