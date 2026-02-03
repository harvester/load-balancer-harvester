package vmi

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubevirtv1 "kubevirt.io/api/core/v1"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
	"github.com/harvester/harvester-load-balancer/pkg/config"
	ctllbv1 "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/loadbalancer.harvesterhci.io/v1beta1"
	"github.com/harvester/harvester-load-balancer/pkg/utils"
)

const controllerName = "harvester-lb-vmi-controller"

type Handler struct {
	lbController ctllbv1.LoadBalancerController
	lbClient     ctllbv1.LoadBalancerClient
	lbCache      ctllbv1.LoadBalancerCache
}

func Register(ctx context.Context, management *config.Management) error {
	vmis := management.KubevirtFactory.Kubevirt().V1().VirtualMachineInstance()
	lbs := management.LbFactory.Loadbalancer().V1beta1().LoadBalancer()

	handler := &Handler{
		lbController: lbs,
		lbClient:     lbs,
		lbCache:      lbs.Cache(),
	}

	vmis.OnChange(ctx, controllerName, handler.OnChange)
	vmis.OnRemove(ctx, controllerName, handler.OnRemove)
	vmis.OnRemove(ctx, controllerName, handler.CleanGuestClusterLBs)

	return nil
}

func (h *Handler) OnChange(_ string, vmi *kubevirtv1.VirtualMachineInstance) (*kubevirtv1.VirtualMachineInstance, error) {
	if vmi == nil || vmi.DeletionTimestamp != nil {
		return nil, nil
	}
	logrus.Debugf("VMI %s/%s is changed", vmi.Namespace, vmi.Name)
	return h.notifyLoadBalancer(vmi)
}

func (h *Handler) OnRemove(_ string, vmi *kubevirtv1.VirtualMachineInstance) (*kubevirtv1.VirtualMachineInstance, error) {
	if vmi == nil {
		return nil, nil
	}
	logrus.Debugf("VMI %s/%s is deleted", vmi.Namespace, vmi.Name)
	return h.notifyLoadBalancer(vmi)
}

func (h *Handler) notifyLoadBalancer(vmi *kubevirtv1.VirtualMachineInstance) (*kubevirtv1.VirtualMachineInstance, error) {
	lbs, err := h.lbCache.List(vmi.Namespace, labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("fail to list load balancers, error: %w", err)
	}

	for _, lb := range lbs {
		// skip the cluster LB or the LB whose server selector is empty
		if lb.DeletionTimestamp != nil || lb.Spec.WorkloadType == lbv1.Cluster || len(lb.Spec.BackendServerSelector) == 0 {
			continue
		}
		// notify LB
		selector, err := utils.NewSelector(lb.Spec.BackendServerSelector)
		if err != nil {
			return nil, fmt.Errorf("fail to parse selector %+v, error: %w", lb.Spec.BackendServerSelector, err)
		}

		if selector.Matches(labels.Set(vmi.Labels)) {
			logrus.Debugf("VMI %s/%s notify lb %s/%s", vmi.Namespace, vmi.Name, lb.Namespace, lb.Name)
			h.lbController.Enqueue(lb.Namespace, lb.Name)
		}
	}
	return vmi, nil
}

func (h *Handler) CleanGuestClusterLBs(_ string, vmi *kubevirtv1.VirtualMachineInstance) (*kubevirtv1.VirtualMachineInstance, error) {
	if !utils.IsGuestClusterVMI(vmi) {
		return vmi, nil
	}
	if !utils.IsVmiWithGuestClusterOnRemoveAnnotation(vmi) {
		return vmi, nil
	}

	lbs, err := h.lbCache.List(vmi.Namespace, labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("fail to list load balancers from namespace %v, error: %w", vmi.Namespace, err)
	}
	if len(lbs) == 0 {
		return vmi, nil
	}

	count := 0
	errCount := 0
	var lastError error
	for _, lb := range lbs {
		// only delete guest cluster type LBs
		if lb.Spec.WorkloadType != lbv1.Cluster {
			continue
		}
		count += 1
		// skip the cluster LB or the LB whose server selector is empty
		if lb.DeletionTimestamp != nil {
			continue
		}
		err = h.lbClient.Delete(lb.Namespace, lb.Name, &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			errCount += 1
			lastError = err
		}
	}

	if errCount != 0 {
		logrus.Infof("detect guest cluster vm %s/%s has annotation %s, delete %v lbs on this namespace, and failed to delete %v, laste error:%v ", vmi.Namespace, vmi.Name, utils.AnnotationKeyGuestClusterOnRemove, count, errCount, err.Error())
		return nil, lastError
	}
	if count != 0 {
		logrus.Infof("detect guest cluster vm %s/%s has annotation %s, delete %v lbs on this namespace", vmi.Namespace, vmi.Name, utils.AnnotationKeyGuestClusterOnRemove, count)
	}
	return vmi, nil
}
