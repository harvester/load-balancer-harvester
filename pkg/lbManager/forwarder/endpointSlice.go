package forwarder

import (
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	discoveryv1 "k8s.io/api/discovery/v1beta1"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1alpha1"
	ctlDiscoveryv1 "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/discovery.k8s.io/v1beta1"
)

const (
	KeyLabelForwarder = "loadbalancer.harvesterhci.io/forwarder"
	KeyServiceName    = "kubernetes.io/service-name"
)

type EndpointSlice struct {
	client ctlDiscoveryv1.EndpointSliceClient
	config *lbv1.LoadBalancer
}

func NewEndpointSlice(client *ctlDiscoveryv1.EndpointSliceClient, config *lbv1.LoadBalancer) *EndpointSlice {
	return &EndpointSlice{
		client: *client,
		config: config,
	}
}

func (e *EndpointSlice) Ensure() error {
	eps, err := e.client.Get(e.config.Namespace, e.config.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if err == nil {
		eps = e.construct(eps)
		_, err := e.client.Update(eps)
		return err
	} else {
		eps = e.construct(nil)
		_, err := e.client.Create(eps)
		return err
	}
}

func (e *EndpointSlice) construct(cur *discoveryv1.EndpointSlice) *discoveryv1.EndpointSlice {
	eps := &discoveryv1.EndpointSlice{}
	if cur != nil {
		eps = cur
	} else {
		eps.Namespace = e.config.Namespace
		eps.Name = e.config.Name
		eps.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: e.config.APIVersion,
				Kind:       e.config.Kind,
				Name:       e.config.Name,
				UID:        e.config.UID,
			}}
		eps.Labels = map[string]string{
			KeyLabelForwarder: "true",
			KeyServiceName:    e.config.Name,
		}
		eps.AddressType = discoveryv1.AddressTypeIPv4
	}

	ports := []discoveryv1.EndpointPort{}
	for _, listener := range e.config.Spec.Listeners {
		port := discoveryv1.EndpointPort{
			Name:     &listener.Name,
			Protocol: &listener.Protocol,
			Port:     &listener.BackendPort,
		}
		ports = append(ports, port)
	}
	eps.Ports = ports

	endpoints := []discoveryv1.Endpoint{}
	for _, server := range e.config.Spec.BackendServers {
		endpoint := discoveryv1.Endpoint{
			Addresses: []string{server},
		}
		endpoints = append(endpoints, endpoint)
	}
	eps.Endpoints = endpoints

	return eps
}

// TODO health check