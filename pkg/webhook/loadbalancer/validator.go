package loadbalancer

import (
	"fmt"

	ctlkubevirtv1 "github.com/harvester/harvester/pkg/generated/controllers/kubevirt.io/v1"
	"github.com/harvester/webhook/pkg/server/admission"
	admissionregv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
	"github.com/harvester/harvester-load-balancer/pkg/utils"
)

type validator struct {
	admission.DefaultValidator
	vmiCache ctlkubevirtv1.VirtualMachineInstanceCache
}

var _ admission.Validator = &validator{}

func NewValidator(vmiCache ctlkubevirtv1.VirtualMachineInstanceCache) admission.Validator {
	return &validator{
		vmiCache: vmiCache,
	}
}

func (v *validator) Create(_ *admission.Request, newObj runtime.Object) error {
	lb := newObj.(*lbv1.LoadBalancer)

	if err := checkListeners(lb); err != nil {
		return fmt.Errorf("create loadbalancer %s/%s failed: %w", lb.Namespace, lb.Name, err)
	}

	ok, err := v.matchAtLeastOneVmi(lb)
	if err != nil {
		return fmt.Errorf("create loadbalancer %s/%s failed: %w", lb.Namespace, lb.Name, err)
	}
	if !ok {
		return fmt.Errorf("create loadbalancer %s/%s failed: no virtual machine instance matched", lb.Namespace, lb.Name)
	}

	return nil
}

func (v *validator) Update(_ *admission.Request, oldObj, newObj runtime.Object) error {
	lb := newObj.(*lbv1.LoadBalancer)

	if lb.DeletionTimestamp != nil {
		return nil
	}

	if err := checkListeners(lb); err != nil {
		return fmt.Errorf("update loadbalancer %s/%s failed: %w", lb.Namespace, lb.Name, err)
	}

	ok, err := v.matchAtLeastOneVmi(lb)
	if err != nil {
		return fmt.Errorf("update loadbalancer %s/%s failed: %w", lb.Namespace, lb.Name, err)
	}
	if !ok {
		return fmt.Errorf("update loadbalancer %s/%s failed: no virtual machine instance matched", lb.Namespace, lb.Name)
	}

	return nil
}

func (v *validator) Resource() admission.Resource {
	return admission.Resource{
		Names:      []string{"loadbalancers"},
		Scope:      admissionregv1.NamespacedScope,
		APIGroup:   lbv1.SchemeGroupVersion.Group,
		APIVersion: lbv1.SchemeGroupVersion.Version,
		ObjectType: &lbv1.LoadBalancer{},
		OperationTypes: []admissionregv1.OperationType{
			admissionregv1.Create,
			admissionregv1.Update,
		},
	}
}

func (v *validator) matchAtLeastOneVmi(lb *lbv1.LoadBalancer) (bool, error) {
	selector, err := utils.NewSelector(lb.Spec.BackendServerSelector)
	if err != nil {
		return false, err
	}

	vmis, err := v.vmiCache.List(lb.Namespace, selector)
	if err != nil {
		return false, err
	}

	return len(vmis) > 0, nil
}

func checkListeners(lb *lbv1.LoadBalancer) error {
	nameMap, portMap, backendMap := map[string]bool{}, map[int32]int{}, map[int32]int{}
	for i, listener := range lb.Spec.Listeners {
		// check listener name
		if _, ok := nameMap[listener.Name]; ok {
			return fmt.Errorf("listener has duplicate name %s", listener.Name)
		}
		nameMap[listener.Name] = true

		// check listener port
		if index, ok := portMap[listener.Port]; ok {
			return fmt.Errorf("listener %s has duplicate port %d with listener %s", listener.Name,
				listener.Port, lb.Spec.Listeners[index].Name)
		}
		portMap[listener.Port] = i

		// check backend port
		if index, ok := backendMap[listener.BackendPort]; ok {
			return fmt.Errorf("listener %s has duplicate backend port %d with listener %s", listener.Name,
				listener.BackendPort, lb.Spec.Listeners[index].Name)
		}
		backendMap[listener.BackendPort] = i
	}

	return nil
}
