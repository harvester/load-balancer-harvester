package loadbalancer

import (
	"fmt"

	"github.com/harvester/webhook/pkg/server/admission"
	admissionregv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
)

type validator struct {
	admission.DefaultValidator
}

var _ admission.Validator = &validator{}

func NewValidator() admission.Validator {
	return &validator{}
}

func (v *validator) Create(_ *admission.Request, newObj runtime.Object) error {
	lb := newObj.(*lbv1.LoadBalancer)

	if err := checkListeners(lb); err != nil {
		return fmt.Errorf("create loadbalancer %s/%s failed: %w", lb.Namespace, lb.Name, err)
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
