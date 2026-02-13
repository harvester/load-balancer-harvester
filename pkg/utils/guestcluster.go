package utils

import (
	kubevirtv1 "kubevirt.io/api/core/v1"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
)

func IsGuestClusterVMI(vmi *kubevirtv1.VirtualMachineInstance) bool {
	if vmi == nil {
		return false
	}
	return vmi.Labels[LabelKeyHarvesterCreator] == GuestClusterHarvesterNodeDriver
}

func IsGuestClusterVM(vm *kubevirtv1.VirtualMachine) bool {
	if vm == nil {
		return false
	}
	return vm.Labels[LabelKeyHarvesterCreator] == GuestClusterHarvesterNodeDriver
}

// when guest-cluster is on remove, the vmi does NOT have this annotation patched
func IsVmiWithGuestClusterOnRemoveAnnotation(vmi *kubevirtv1.VirtualMachineInstance) bool {
	if vmi == nil {
		return false
	}

	return vmi.Annotations[AnnotationKeyGuestClusterOnRemove] == "true"
}

// when guest-cluster is on remove, the vm has this annotation patched
func IsVmWithGuestClusterOnRemoveAnnotation(vm *kubevirtv1.VirtualMachine) bool {
	if vm == nil {
		return false
	}

	return vm.Annotations[AnnotationKeyGuestClusterOnRemove] == "true"
}

func GetGuestClusterNameFromVM(vm *kubevirtv1.VirtualMachine) (string, bool) {
	if vm == nil {
		return "", false
	}
	gc, ok := vm.Labels[LabelKeyGuestClusterNameOnVM]
	return gc, ok
}

func GetGuestClusterNameFromLB(lb *lbv1.LoadBalancer) (string, bool) {
	if lb == nil {
		return "", false
	}
	gc, ok := lb.Labels[LabelKeyGuestClusterNameOnLB]
	return gc, ok
}
