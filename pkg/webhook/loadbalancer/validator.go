package loadbalancer

import (
	"fmt"

	"github.com/harvester/webhook/pkg/server/admission"
	"github.com/sirupsen/logrus"
	admissionregv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
	ctlkubevirtv1 "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/kubevirt.io/v1"
	"github.com/harvester/harvester-load-balancer/pkg/utils"
)

type validator struct {
	admission.DefaultValidator

	vmCache  ctlkubevirtv1.VirtualMachineCache
	vmiCache ctlkubevirtv1.VirtualMachineInstanceCache
}

const defaultGuestClusterName = "kubernetes"

var _ admission.Validator = &validator{}

func NewValidator(vmCache ctlkubevirtv1.VirtualMachineCache, vmiCache ctlkubevirtv1.VirtualMachineInstanceCache) admission.Validator {
	return &validator{
		vmCache:  vmCache,
		vmiCache: vmiCache,
	}
}

func (v *validator) Create(_ *admission.Request, newObj runtime.Object) error {
	lb := newObj.(*lbv1.LoadBalancer)

	if err := checkListeners(lb); err != nil {
		return fmt.Errorf("create loadbalancer %s/%s failed: %w", lb.Namespace, lb.Name, err)
	}

	if err := checkHealthyCheck(lb); err != nil {
		return fmt.Errorf("create loadbalancer %s/%s failed with healthyCheck: %w", lb.Namespace, lb.Name, err)
	}

	// when a guest-cluster is on remove, Harvester controller deletes all its LBs automatically
	// but the guest-cluster side might try to recreate them
	// this check blocks the recreation until the guest-cluster if fully gone
	if err := v.checkGuestClusterIsOnRemove(lb); err != nil {
		err := fmt.Errorf("create loadbalancer %s/%s failed with guest cluster check: %w", lb.Namespace, lb.Name, err)
		logrus.Infof("%v", err.Error())
		return err
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

	if err := checkWorkloadType(oldLb, lb); err != nil {
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
	nameMap, portMap, backendMap := map[string]bool{}, map[string]int{}, map[string]int{}

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
		portKey := fmt.Sprintf("%s:%v", listener.Protocol, listener.Port)
		if index, ok := portMap[portKey]; ok {
			return fmt.Errorf("listener %s has duplicate port %s with listener %s", listener.Name,
				portKey, lb.Spec.Listeners[index].Name)
		}
		portMap[portKey] = i

		// check backend port
		backendKey := fmt.Sprintf("%s:%v", listener.Protocol, listener.BackendPort)
		if index, ok := backendMap[backendKey]; ok {
			return fmt.Errorf("listener %s has duplicate backend port %s with listener %s", listener.Name,
				backendKey, lb.Spec.Listeners[index].Name)
		}
		backendMap[backendKey] = i
	}

	for _, listener := range lb.Spec.Listeners {
		// check listener name
		if listener.Port > maxPort {
			return fmt.Errorf("listener port %v must <= %v", listener.Port, maxPort)
		} else if listener.Port < 1 {
			return fmt.Errorf("listener port %v must >= 1", listener.Port)
		}
		if listener.BackendPort > maxPort {
			return fmt.Errorf("listener backend port %v must <= %v", listener.BackendPort, maxPort)
		} else if listener.BackendPort < 1 {
			return fmt.Errorf("listener backend port %v must >= 1", listener.BackendPort)
		}
	}

	return nil
}

func checkHealthyCheck(lb *lbv1.LoadBalancer) error {
	// The healthyCheck related configuration is only valid for VM type LB.
	if lb.Spec.WorkloadType == lbv1.Cluster {
		return nil
	}

	if lb.Spec.HealthCheck != nil && lb.Spec.HealthCheck.Port != 0 {
		wrongProtocol := false
		for _, listener := range lb.Spec.Listeners {
			// check listener port and protocol, only TCP is supported now
			//#nosec
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
// if IPAM is not set, it defaults to lbv1.Pool
func checkIPAM(oldLb, newLb *lbv1.LoadBalancer) error {
	if (oldLb.Spec.IPAM != lbv1.DHCP && newLb.Spec.IPAM == lbv1.DHCP) || (oldLb.Spec.IPAM == lbv1.DHCP && newLb.Spec.IPAM != lbv1.DHCP) {
		return fmt.Errorf("can't change the IPAM from %v to %v", oldLb.Spec.IPAM, newLb.Spec.IPAM)
	}

	return nil
}

// change the WorkloadType makes no sense as they come from different scenarios with different parameters
// Cluster type is created by the cloud-provider-harvester on Rancher and VM type is created by Harvester
// if WorkloadType is not set, it defaults to lbv1.VM
func checkWorkloadType(oldLb, newLb *lbv1.LoadBalancer) error {
	if (oldLb.Spec.WorkloadType != lbv1.Cluster && newLb.Spec.WorkloadType == lbv1.Cluster) || (oldLb.Spec.WorkloadType == lbv1.Cluster && newLb.Spec.WorkloadType != lbv1.Cluster) {
		return fmt.Errorf("can't change the WorkloadType from %v to %v", oldLb.Spec.WorkloadType, newLb.Spec.WorkloadType)
	}

	return nil
}

func (v *validator) checkGuestClusterIsOnRemove(lb *lbv1.LoadBalancer) error {
	if lb.Spec.WorkloadType != lbv1.Cluster {
		return nil
	}

	gcName, ok := utils.GetGuestClusterNameFromLB(lb)
	// The cluster name label is missing from the LoadBalancer metadata
	// This prevents the controller from verifying if the guest cluster is being removed
	// We allow this to maintain backward compatibility with legacy or custom clusters
	if !ok {
		logrus.WithFields(logrus.Fields{
			"namespace": lb.Namespace,
			"name":      lb.Name,
		}).Warnf("guest cluster name is missing from label '%s': skipping cluster removal state check for backward compatibility.",
			utils.LabelKeyGuestClusterNameOnLB)
		return nil
	}

	// For backward compatibility, we don't check when the cluster name is empty or the default ("kubernetes")
	// WARNING: Using the default name may prevent the LoadBalancer auto-reclaim feature
	// from uniquely identifying this cluster, potentially leading to resource leakage or accidental deletion
	if gcName == "" || gcName == defaultGuestClusterName {
		logrus.WithFields(logrus.Fields{
			"namespace": lb.Namespace,
			"name":      lb.Name,
			"cluster":   gcName,
		}).Warn("Harvester LB auto-reclaim risk: ambiguous guest cluster name may cause resource leakage or accidental deletion, skipping cluster removal state check for backward compatibility")

		return nil
	}

	// List all VMs within the same namespace as the LoadBalancer
	// The guest cluster name is required because multiple guest clusters may coexist in a single namespace
	// Note: Only Rancher Manager-managed guest clusters follow this labeling convention
	// custom or manual guest clusters may not adhere to this requirement
	selector := labels.Set(map[string]string{
		utils.LabelKeyHarvesterCreator:     utils.GuestClusterHarvesterNodeDriver,
		utils.LabelKeyGuestClusterNameOnVM: gcName,
	}).AsSelector()
	vms, err := v.vmCache.List(lb.Namespace, selector)
	if err != nil {
		return fmt.Errorf("list vm from %s failed, error: %w", lb.Namespace, err)
	}

	// Return nil to accommodate two scenarios:
	// 1. Custom Clusters: The guest cluster doesn't follow standard labeling
	// 2. Deletion Race: The VMs were already purged during an RKE2 cluster removal
	// To maintain backward compatibility for custom setups, we skip the LB creation check
	if len(vms) == 0 {
		logrus.WithFields(logrus.Fields{
			"namespace": lb.Namespace,
			"name":      lb.Name,
			"cluster":   gcName,
		}).Warnf("No VMs found with selector %s: skipping cluster removal state check for backward compatibility",
			selector.String())
		return nil
	}

	// For backward compatibility, we only deny LoadBalancer creation in this specific state:
	// When Rancher Manager initiates a guest cluster deletion, it marks all constituent VMs
	// with the 'AnnotationKeyGuestClusterOnRemove' annotation
	// If a new LoadBalancer request arrives during this teardown phase, it is rejected
	// to prevent orphaned resources
	for _, vm := range vms {
		if utils.IsVmWithGuestClusterOnRemoveAnnotation(vm) {
			logrus.WithFields(logrus.Fields{
				"namespace": lb.Namespace,
				"name":      lb.Name,
			}).Debugf("the vm %s/%s shows guest cluster %s is being removed",
				vm.Namespace, vm.Name, gcName)

			return fmt.Errorf("the vm %s/%s shows guest cluster %s is being removed", vm.Namespace, vm.Name, gcName)
		}
	}
	return nil
}
