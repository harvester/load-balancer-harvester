package loadbalancer

import (
	"context"

	ctlcore "github.com/rancher/wrangler/pkg/generated/controllers/core"
	ctlcorev1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"k8s.io/klog/v2"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1alpha1"
	ctldiscovery "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/discovery.k8s.io"
	ctldiscoveryv1 "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/discovery.k8s.io/v1beta1"
	ctllb "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/loadbalancer.harvesterhci.io"
	ctllbv1 "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/loadbalancer.harvesterhci.io/v1alpha1"
	"github.com/harvester/harvester-load-balancer/pkg/lb"
	"github.com/harvester/harvester-load-balancer/pkg/lb/servicelb"
)

const controllerName = "harvester-lb-controller"

type Handler struct {
	lbClient            ctllbv1.LoadBalancerClient
	serviceClient       ctlcorev1.ServiceClient
	serviceCache        ctlcorev1.ServiceCache
	endpointSliceClient ctldiscoveryv1.EndpointSliceClient
	endpointSliceCache  ctldiscoveryv1.EndpointSliceCache
	lbManager           lb.Manager
}

func Register(ctx context.Context, lbFactory *ctllb.Factory, coreFactory *ctlcore.Factory, discoveryFactory *ctldiscovery.Factory) error {
	lbs := lbFactory.Loadbalancer().V1alpha1().LoadBalancer()
	services := coreFactory.Core().V1().Service()
	endpointSlices := discoveryFactory.Discovery().V1beta1().EndpointSlice()

	handler := &Handler{
		lbClient:            lbs,
		serviceClient:       services,
		serviceCache:        services.Cache(),
		endpointSliceClient: endpointSlices,
		endpointSliceCache:  endpointSlices.Cache(),
	}

	handler.lbManager = servicelb.NewManager(ctx, handler.serviceClient, handler.serviceCache, handler.endpointSliceClient, handler.endpointSliceCache)

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
	if err := h.lbManager.EnsureLoadBalancer(lb); err != nil {
		lbv1.LoadBalancerReady.False(lbCopy)
		lbv1.LoadBalancerReady.Message(lbCopy, err.Error())
		return h.lbClient.Update(lbCopy)
	}

	lbv1.LoadBalancerReady.True(lbCopy)
	lbv1.LoadBalancerReady.Message(lbCopy, "")
	return h.lbClient.Update(lbCopy)
}

func (h *Handler) OnRemove(key string, lb *lbv1.LoadBalancer) (*lbv1.LoadBalancer, error) {
	if lb == nil {
		return nil, nil
	}

	klog.V(4).Infof("load balancer configuration %s has been deleted", lb.Name)

	h.lbManager.DeleteLoadBalancer(lb)

	return lb, nil
}
