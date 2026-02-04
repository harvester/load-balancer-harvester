package vm

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

const controllerName = "harvester-lb-vm-controller"

type Handler struct {
	lbClient ctllbv1.LoadBalancerClient
	lbCache  ctllbv1.LoadBalancerCache
}

func Register(ctx context.Context, management *config.Management) error {
	vms := management.KubevirtFactory.Kubevirt().V1().VirtualMachine()
	lbs := management.LbFactory.Loadbalancer().V1beta1().LoadBalancer()

	handler := &Handler{
		lbClient: lbs,
		lbCache:  lbs.Cache(),
	}

	vms.OnRemove(ctx, controllerName, handler.CleanGuestClusterLBs)
	return nil
}

func (h *Handler) CleanGuestClusterLBs(_ string, vm *kubevirtv1.VirtualMachine) (*kubevirtv1.VirtualMachine, error) {
	if !utils.IsGuestClusterVM(vm) {
		return vm, nil
	}
	if !utils.IsVmWithGuestClusterOnRemoveAnnotation(vm) {
		return vm, nil
	}

	gcName, ok := utils.GetGuestClusterNameFromVM(vm)
	if !ok {
		logrus.Warnf("can't get the guest cluster name from vm %s/%s but it has on remove annotation, skip cleaning lb", vm.Namespace, vm.Name)
		return vm, nil
	}

	// list all the lb instance in the same namespace of the vm
	// the guestcluster name is also required as multi guest clusters might coexist on a namespace
	lbs, err := h.lbCache.List(vm.Namespace, labels.Set(map[string]string{
		utils.LabelKeyGuestClusterNameOnLB: gcName,
	}).AsSelector())
	if err != nil {
		return nil, fmt.Errorf("list lb from %s failed, error: %w", vm.Namespace, err)
	}

	if len(lbs) == 0 {
		return vm, nil
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
		logrus.Infof("detect guest cluster vm %s/%s has annotation %s, delete %v lbs on this namespace, and failed to delete %v, laste error:%v ", vm.Namespace, vm.Name, utils.AnnotationKeyGuestClusterOnRemove, count, errCount, err.Error())
		return nil, lastError
	}
	if count != 0 {
		logrus.Infof("detect guest cluster vm %s/%s has annotation %s, delete %v lbs on this namespace", vm.Namespace, vm.Name, utils.AnnotationKeyGuestClusterOnRemove, count)
	}
	return vm, nil
}
