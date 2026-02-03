package utils

import kubevirtv1 "kubevirt.io/api/core/v1"

func IsGuestClusterVMI(vmi *kubevirtv1.VirtualMachineInstance) bool {
	if vmi == nil {
		return false
	}
	return vmi.Labels[LabelKeyHarvesterCreator] == GuestClusterHarvesterNodeDriver
}

func IsVmiWithGuestClusterOnRemoveAnnotation (vmi *kubevirtv1.VirtualMachineInstance) bool {
	if vmi == nil {
		return false
	}

	return vmi.Annotations[AnnotationKeyGuestClusterOnRemove] == "true"
}
