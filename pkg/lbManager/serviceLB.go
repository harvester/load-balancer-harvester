package lbManager

import (
	ctlCorev1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1alpha1"
	ctlDiscoveryv1 "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/discovery.k8s.io/v1beta1"
	"github.com/harvester/harvester-load-balancer/pkg/lbManager/entry"
	"github.com/harvester/harvester-load-balancer/pkg/lbManager/forwarder"
)

// ServiceLB takes Kubernetes service as the traffic entry and endpointSlice as the forwarder.
type ServiceLB struct {
	service *entry.Service
	endpointSlice *forwarder.EndpointSlice
}

func NewServiceLB(lb *lbv1.LoadBalancer, serviceClient *ctlCorev1.ServiceClient, endpointSliceClient *ctlDiscoveryv1.EndpointSliceClient) *ServiceLB {
	return &ServiceLB{
		service:       entry.NewService(serviceClient, lb),
		endpointSlice: forwarder.NewEndpointSlice(endpointSliceClient, lb),
	}
}

func (s *ServiceLB) EnsureEntry() error {
	return s.service.Ensure()
}

func (s *ServiceLB) EnsureForwarder() error {
	return s.endpointSlice.Ensure()
}
