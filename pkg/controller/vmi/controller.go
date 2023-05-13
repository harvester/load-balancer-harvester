package vmi

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
	kubevirtv1 "kubevirt.io/api/core/v1"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
	"github.com/harvester/harvester-load-balancer/pkg/config"
	ctllbv1 "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/loadbalancer.harvesterhci.io/v1beta1"
	lbpkg "github.com/harvester/harvester-load-balancer/pkg/lb"
	"github.com/harvester/harvester-load-balancer/pkg/lb/servicelb"
	"github.com/harvester/harvester-load-balancer/pkg/utils"
)

const controllerName = "harvester-lb-vmi-controller"

type Handler struct {
	lbController ctllbv1.LoadBalancerController
	lbClient     ctllbv1.LoadBalancerClient
	lbCache      ctllbv1.LoadBalancerCache

	lbManager lbpkg.Manager
}

func Register(ctx context.Context, management *config.Management) error {
	vmis := management.KubevirtFactory.Kubevirt().V1().VirtualMachineInstance()
	lbs := management.LbFactory.Loadbalancer().V1beta1().LoadBalancer()

	handler := &Handler{
		lbController: lbs,
		lbClient:     lbs,
		lbCache:      lbs.Cache(),

		lbManager: management.LBManager,
	}

	vmis.OnChange(ctx, controllerName, handler.OnChange)
	vmis.OnRemove(ctx, controllerName, handler.OnRemove)

	return nil
}

func (h *Handler) OnChange(_ string, vmi *kubevirtv1.VirtualMachineInstance) (*kubevirtv1.VirtualMachineInstance, error) {
	if vmi == nil || vmi.DeletionTimestamp != nil {
		return nil, nil
	}
	logrus.Debugf("VMI %s/%s has been changed", vmi.Namespace, vmi.Name)

	lbs, err := h.lbCache.List(vmi.Namespace, labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("list load balancers failed, error: %w", err)
	}

	for _, lb := range lbs {
		// skip the cluster LB or the LB whose server selector is empty
		if lb.Spec.WorkloadType == lbv1.Cluster || len(lb.Spec.BackendServerSelector) == 0 {
			continue
		}

		selector, err := utils.NewSelector(lb.Spec.BackendServerSelector)
		if err != nil {
			return nil, fmt.Errorf("parse selector %+v failed, error: %w", lb.Spec.BackendServerSelector, err)
		}
		// add or update the backend server to the matched load balancer
		isChanged := false
		if selector.Matches(labels.Set(vmi.Labels)) {
			if isChanged, err = h.addServerToLB(vmi, lb); err != nil {
				return nil, fmt.Errorf("add server %s/%s to lb %s/%s failed, error: %w", vmi.Namespace, vmi.Name, lb.Namespace, lb.Name, err)
			}
		} else { // remove the backend server from the unmatched load balancer
			if isChanged, err = h.removeServerFromLB(vmi, lb); err != nil {
				return nil, fmt.Errorf("remove server %s/%s from lb %s/%s failed, error: %w", vmi.Namespace, vmi.Name, lb.Namespace, lb.Name, err)
			}
		}
		// update the load balancer status
		if isChanged {
			h.lbController.Enqueue(lb.Namespace, lb.Name)
		}
	}

	return vmi, nil
}

func (h *Handler) OnRemove(_ string, vmi *kubevirtv1.VirtualMachineInstance) (*kubevirtv1.VirtualMachineInstance, error) {
	if vmi == nil {
		return nil, nil
	}

	logrus.Debugf("VMI %s/%s has been deleted", vmi.Namespace, vmi.Name)

	lbs, err := h.lbCache.List("", labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("list load balancers failed, error: %w", err)
	}

	for _, lb := range lbs {
		// skip the cluster LB or the LB whose server selector is empty
		if lb.Spec.WorkloadType == lbv1.Cluster || len(lb.Spec.BackendServerSelector) == 0 {
			continue
		}
		// remove the backend server from the load balancers
		if ok, err := h.removeServerFromLB(vmi, lb); err != nil {
			return nil, fmt.Errorf("remove server %s/%s from lb %s/%s failed, error: %w", vmi.Namespace, vmi.Name, lb.Namespace, lb.Name, err)
		} else if ok {
			h.lbController.Enqueue(lb.Namespace, lb.Name)
		}
	}

	return vmi, nil
}

func (h *Handler) removeServerFromLB(vmi *kubevirtv1.VirtualMachineInstance, lb *lbv1.LoadBalancer) (bool, error) {
	server := &servicelb.Server{VirtualMachineInstance: vmi}
	return h.lbManager.RemoveBackendServers(lb, []lbpkg.BackendServer{server})
}

func (h *Handler) addServerToLB(vmi *kubevirtv1.VirtualMachineInstance, lb *lbv1.LoadBalancer) (bool, error) {
	server := &servicelb.Server{VirtualMachineInstance: vmi}
	return h.lbManager.AddBackendServers(lb, []lbpkg.BackendServer{server})
}
