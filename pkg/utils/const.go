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
)
