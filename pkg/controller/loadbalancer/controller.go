package loadbalancer

import (
	"context"

	ctlCore "github.com/rancher/wrangler/pkg/generated/controllers/core"
	ctlCorev1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"k8s.io/klog/v2"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1alpha1"
	ctlDiscovery "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/discovery.k8s.io"
	ctlDiscoveryv1 "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/discovery.k8s.io/v1beta1"
	ctlLB "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/loadbalancer.harvesterhci.io"
	ctlLBv1 "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/loadbalancer.harvesterhci.io/v1alpha1"
	"github.com/harvester/harvester-load-balancer/pkg/lbManager"
)

const controllerName = "harvester-lb-controller"

type Handler struct {
	lbClient            ctlLBv1.LoadBalancerClient
	serviceClient       ctlCorev1.ServiceClient
	endpointSliceClient ctlDiscoveryv1.EndpointSliceClient
}

func Register(ctx context.Context, lbFactory *ctlLB.Factory, coreFactory *ctlCore.Factory, discoveryFactory *ctlDiscovery.Factory) error {
	lbs := lbFactory.Loadbalancer().V1alpha1().LoadBalancer()
	services := coreFactory.Core().V1().Service()
	endpointSlices := discoveryFactory.Discovery().V1beta1().EndpointSlice()

	handler := &Handler{
		lbClient:            lbs,
		serviceClient:       services,
		endpointSliceClient: endpointSlices,
	}

	lbs.OnChange(ctx, controllerName, handler.OnChange)

	return nil
}

func (h *Handler) OnChange(key string, lb *lbv1.LoadBalancer) (*lbv1.LoadBalancer, error) {
	if lb == nil || lb.DeletionTimestamp != nil {
		return nil, nil
	}
	klog.Infof("load balancer configuration %s has been changed, spec: %+v", lb.Name, lb.Spec)

	serviceLB := lbManager.NewServiceLB(lb, &h.serviceClient, &h.endpointSliceClient)
	lbCopy := lb.DeepCopy()
	if err := serviceLB.EnsureEntry(); err != nil {
		lbv1.LoadBalancerReady.SetError(lbCopy, "EntryError", err)
		goto UpdateStatus
	}
	if err := serviceLB.EnsureForwarder(); err != nil {
		lbv1.LoadBalancerReady.SetError(lbCopy, "ForwarderError", err)
		goto UpdateStatus
	}

	lbv1.LoadBalancerReady.True(lbCopy)

UpdateStatus:
	return h.lbClient.Update(lbCopy)
}
