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
	"github.com/harvester/harvester-load-balancer/pkg/lb/servicelb"
)

const controllerName = "harvester-lb-controller"

type Handler struct {
	lbClient            ctlLBv1.LoadBalancerClient
	serviceClient       ctlCorev1.ServiceClient
	endpointSliceClient ctlDiscoveryv1.EndpointSliceClient
	serviceLBManager    *servicelb.Manager
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

	handler.serviceLBManager = servicelb.NewManager(ctx, &handler.serviceClient, &handler.endpointSliceClient)

	lbs.OnChange(ctx, controllerName, handler.OnChange)
	lbs.OnRemove(ctx, controllerName, handler.OnRemove)

	return nil
}

func (h *Handler) OnChange(key string, lb *lbv1.LoadBalancer) (*lbv1.LoadBalancer, error) {
	if lb == nil || lb.DeletionTimestamp != nil {
		return nil, nil
	}
	klog.V(4).Infof("load balancer configuration %s has been changed, spec: %+v", lb.Name, lb.Spec)

	lbCopy := lb.DeepCopy()
	if err := h.serviceLBManager.EnsureLoadBalancer(lb); err != nil {
		lbv1.LoadBalancerReady.SetError(lbCopy, "", err)
		return h.lbClient.Update(lbCopy)
	}

	lbv1.LoadBalancerReady.True(lbCopy)
	return h.lbClient.Update(lbCopy)
}

func (h *Handler) OnRemove(key string, lb *lbv1.LoadBalancer) (*lbv1.LoadBalancer, error) {
	if lb == nil {
		return nil, nil
	}

	klog.V(4).Infof("load balancer configuration %s has been deleted", lb.Name)

	h.serviceLBManager.DeleteLoadBalancer(lb)

	return lb, nil
}
