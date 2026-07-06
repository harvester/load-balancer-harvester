package utils

import lb "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io"

const (
	// KeyGlobalIPPool is the label key used to represent a global IPPool.
	//
	// Note: This constant is maintained for backward compatibility. The validator
	// and controller do not rely on this label for internal logic; they use
	// ipam.IsGlobalIPPool(pool) as the definitive source of truth.
	//
	// Each network can have at most one global IPPool.
	KeyGlobalIPPool = lb.GroupName + "/global-ip-pool"
	ValueTrue       = "true"

	Address4AskDHCP = "0.0.0.0"

	// guest cluster could set this for a specific target network
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

	HarvesterCloudProvider = "cloudprovider.harvesterhci.io"

	HarvesterCloudProviderPrefix = HarvesterCloudProvider + "/"

	NetworkTypeManagement = "managementNetwork"

	// When the guest cluster has multiple networks, it can explicitly specify which one is the management network, instead of guessing or hardcoding.
	AnnotationKeyGuestClusterManagementNetworkOnLB = HarvesterCloudProviderPrefix + NetworkTypeManagement

	// When HCP requests a specific network for the guest cluster, this annotation is used to save the original input
	AnnotationKeyGuestClusterRequestedNetworkOnLB = HarvesterCloudProviderPrefix + "network"

	// guest cluster used VM has such label: guestcluster.harvesterhci.io/name: gc3
	LabelKeyGuestClusterNameOnVM = "guestcluster.harvesterhci.io/name"

	// guest cluster create LB has such label: cloudprovider.harvesterhci.io/cluster: gc3
	LabelKeyGuestClusterNameOnLB = "cloudprovider.harvesterhci.io/cluster"
)
