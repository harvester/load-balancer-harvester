package loadbalancer

import (
	"context"

	"k8s.io/klog/v2"

	lbv1 "github.com/harvester/harvester-conveyor/pkg/apis/network.harvesterhci.io/v1alpha1"
	"github.com/harvester/harvester-conveyor/pkg/generated/controllers/core"
	v1 "github.com/harvester/harvester-conveyor/pkg/generated/controllers/core/v1"
	"github.com/harvester/harvester-conveyor/pkg/generated/controllers/core/v1beta1"
	"github.com/harvester/harvester-conveyor/pkg/generated/controllers/network.harvesterhci.io"
)

const controllerName = "harvester-lb-controller"

type Handler struct {
	serviceCtr       v1.ServiceController
	endpointSliceCtr v1beta1.EndpointSliceController
}

func Register(ctx context.Context, lbFactory *network.Factory, coreFactory *core.Factory) error {
	lbs := lbFactory.Network().V1alpha1().LoadBalancer()
	services := coreFactory.Core().V1().Service()
	epSlices := coreFactory.Core().V1beta1().EndpointSlice()

	handler := &Handler{
		serviceCtr:       services,
		endpointSliceCtr: epSlices,
	}

	lbs.OnChange(ctx, controllerName, handler.OnChange)
	lbs.OnRemove(ctx, controllerName, handler.OnRemove)

	return nil
}

func (h Handler) OnChange(key string, lb *lbv1.LoadBalancer) (*lbv1.LoadBalancer, error) {
	if lb == nil || lb.DeletionTimestamp != nil {
		return nil, nil
	}

	klog.Infof("load balancer configuration %s has been changed, spec: %+v", lb.Name, lb.Spec)

	return nil, nil
}

func (h Handler) OnRemove(key string, lb *lbv1.LoadBalancer) (*lbv1.LoadBalancer, error) {
	if lb == nil {
		return nil, nil
	}

	klog.Infof("load balancer configuration %s has been deleted", lb.Name)

	return nil, nil
}
