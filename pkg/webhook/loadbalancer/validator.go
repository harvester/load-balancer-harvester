package loadbalancer

import (
	"fmt"

	"github.com/harvester/webhook/pkg/server/admission"
	admissionregv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
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

	if err := checkHealthyCheck(lb); err != nil {
		return fmt.Errorf("create loadbalancer %s/%s failed with healthyCheck: %w", lb.Namespace, lb.Name, err)
	}

	return nil
}

func (v *validator) Update(_ *admission.Request, oldObj, newObj runtime.Object) error {
	lb := newObj.(*lbv1.LoadBalancer)
	oldLb := oldObj.(*lbv1.LoadBalancer)

	if lb.DeletionTimestamp != nil {
		return nil
	}

	if err := checkListeners(lb); err != nil {
		return fmt.Errorf("update loadbalancer %s/%s failed: %w", lb.Namespace, lb.Name, err)
	}

	if err := checkHealthyCheck(lb); err != nil {
		return fmt.Errorf("update loadbalancer %s/%s failed with healthyCheck: %w", lb.Namespace, lb.Name, err)
	}

	if err := checkIPAM(oldLb, lb); err != nil {
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

const maxPort = 65535

func checkListeners(lb *lbv1.LoadBalancer) error {
	nameMap, portMap, backendMap := map[string]bool{}, map[int32]int{}, map[int32]int{}

	// Cluster-type load balancers can have no listeners since the actual load-balancing is done in the guest cluster.
	if lb.Spec.WorkloadType == lbv1.Cluster {
		return nil
	}

	if len(lb.Spec.Listeners) == 0 {
		return fmt.Errorf("the loadbalancer needs to have at least one listener")
	}
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

	for _, listener := range lb.Spec.Listeners {
		// check listener name
		if listener.Port > maxPort {
			return fmt.Errorf("listener port %v must <= %v", listener.Port, maxPort)
		} else if listener.Port < 1 {
			return fmt.Errorf("listener port %v must >= 1", listener.Port)
		}
		if listener.BackendPort > maxPort {
			return fmt.Errorf("listener backend port %v must <= %v", listener.Port, maxPort)
		} else if listener.BackendPort < 1 {
			return fmt.Errorf("listener backend port %v must >= 1", listener.Port)
		}
	}

	return nil
}

func checkHealthyCheck(lb *lbv1.LoadBalancer) error {
	if lb.Spec.HealthCheck != nil && lb.Spec.HealthCheck.Port != 0 {
		wrongProtocol := false
		for _, listener := range lb.Spec.Listeners {
			// check listener port and protocol, only TCP is supported now
			if uint(listener.BackendPort) == lb.Spec.HealthCheck.Port {
				if listener.Protocol == corev1.ProtocolTCP {
					if lb.Spec.HealthCheck.SuccessThreshold == 0 {
						return fmt.Errorf("healthcheck SuccessThreshold should > 0")
					}
					if lb.Spec.HealthCheck.FailureThreshold == 0 {
						return fmt.Errorf("healthcheck FailureThreshold should > 0")
					}
					if lb.Spec.HealthCheck.PeriodSeconds == 0 {
						return fmt.Errorf("healthcheck PeriodSeconds should > 0")
					}
					if lb.Spec.HealthCheck.TimeoutSeconds == 0 {
						return fmt.Errorf("healthcheck TimeoutSeconds should > 0")
					}
					return nil
				}
				// not the expected TCP
				wrongProtocol = true
			}
		}
		if wrongProtocol {
			return fmt.Errorf("healthcheck port %v can only be a TCP backend port", lb.Spec.HealthCheck.Port)
		}
		return fmt.Errorf("healthcheck port %v is not in listener backend port list", lb.Spec.HealthCheck.Port)
	}

	return nil
}

// change the IPAM may cause IP leaking
// user may re-create the LB to change the IPAM
// if IPAM is not setting, it defaults to lbv1.Pool
func checkIPAM(oldLb, newLb *lbv1.LoadBalancer) error {
	if (oldLb.Spec.IPAM != lbv1.DHCP && newLb.Spec.IPAM == lbv1.DHCP) || (oldLb.Spec.IPAM == lbv1.DHCP && newLb.Spec.IPAM != lbv1.DHCP) {
		return fmt.Errorf("can't change the IPAM from %v to %v", oldLb.Spec.IPAM, newLb.Spec.IPAM)
	}

	return nil
}
