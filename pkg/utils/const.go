package utils

import lb "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io"

const (
	KeyGlobalIPPool = lb.GroupName + "/global-ip-pool"
	ValueTrue       = "true"

	Address4AskDHCP = "0.0.0.0"

	AnnotationKeyNetwork   = lb.GroupName + "/network"
	AnnotationKeyProject   = lb.GroupName + "/project"
	AnnotationKeyNamespace = lb.GroupName + "/namespace"
	AnnotationKeyCluster   = lb.GroupName + "/cluster"

	// value format: loadbalancer.harvesterhci.io/manuallyReleaseIP: "192.168.5.12: default/cluster1-lb-3"
	AnnotationKeyManuallyReleaseIP = lb.GroupName + "/manuallyReleaseIP"

	DuplicateAllocationKeyWord = "duplicate allocation is not allowed"

	// refer https://github.com/rancher/rancher/blob/e5d419fce68de6dc631a818a2e7e206f2221ebc3/pkg/controllers/provisioningv2/harvestercleanup/controller.go#L29
	// redefine following annotation for LB usage
	//   removedAllPVCsAnnotationKey             = "harvesterhci.io/removeAllPersistentVolumeClaims"
	AnnotationKeyGuestClusterOnRemove = "harvesterhci.io/removeAllPersistentVolumeClaims"

	LabelKeyHarvesterCreator        = "harvesterhci.io/creator"
	GuestClusterHarvesterNodeDriver = "docker-machine-driver-harvester"
)
