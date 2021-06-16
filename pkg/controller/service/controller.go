package service

import (
	"context"
	"fmt"

	ctlCore "github.com/rancher/wrangler/pkg/generated/controllers/core"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1alpha1"
	ctlLB "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/loadbalancer.harvesterhci.io"
	ctlLBv1 "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/loadbalancer.harvesterhci.io/v1alpha1"
	"github.com/harvester/harvester-load-balancer/pkg/lb/servicelb"
)

const controllerName = "harvester-service-controller"

type Handler struct {
	lbClient ctlLBv1.LoadBalancerClient
}

func Register(ctx context.Context, lbFactory *ctlLB.Factory, coreFactory *ctlCore.Factory) error {
	services := coreFactory.Core().V1().Service()
	lbs := lbFactory.Loadbalancer().V1alpha1().LoadBalancer()

	handler := &Handler{
		lbClient: lbs,
	}

	services.OnChange(ctx, controllerName, handler.OnChange)

	return nil
}

func (h Handler) OnChange(key string, service *coreV1.Service) (*coreV1.Service, error) {
	if service == nil || service.DeletionTimestamp != nil {
		return nil, nil
	}
	labels := service.GetLabels()
	if v, ok := labels[servicelb.KeyLabel]; !ok || v != "true" {
		return nil, nil
	}
	klog.V(4).Infof("service configuration %s has been changed, spec: %+v", service.Name, service.Spec)

	lb, err := h.lbClient.Get(service.Namespace, service.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get loadbalancer %s failed, error: %w", service.Name, err)
	}
	lbCopy := lb.DeepCopy()

	lbCopy.Status.InternalAddress = service.Spec.ClusterIP
	if lb.Spec.Type == lbv1.External {
		if len(service.Status.LoadBalancer.Ingress) > 0 {
			lbCopy.Status.ExternalAddress = service.Status.LoadBalancer.Ingress[0].IP
		}
	}

	if _, err := h.lbClient.Update(lbCopy); err != nil {
		return nil, fmt.Errorf("update loadbalancer %s failed, error: %w", lbCopy.Name, err)
	}

	return service, nil
}
