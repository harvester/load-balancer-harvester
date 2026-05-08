package loadbalancer

import (
	"fmt"
	"strings"

	"github.com/harvester/webhook/pkg/server/admission"
	rancherproject "github.com/rancher/rancher/pkg/project"
	ctlcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	admissionregv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubevirtv1 "kubevirt.io/api/core/v1"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
	ctlkubevirtv1 "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/kubevirt.io/v1"
	"github.com/harvester/harvester-load-balancer/pkg/utils"
)

type mutator struct {
	admission.DefaultMutator

	namespaceCache ctlcorev1.NamespaceCache
	vmiCache       ctlkubevirtv1.VirtualMachineInstanceCache
}

var _ admission.Mutator = &mutator{}

func NewMutator(namespaceCache ctlcorev1.NamespaceCache,
	vmiCache ctlkubevirtv1.VirtualMachineInstanceCache) admission.Mutator {
	return &mutator{
		namespaceCache: namespaceCache,
		vmiCache:       vmiCache,
	}
}

func (m *mutator) Create(_ *admission.Request, newObj runtime.Object) (admission.Patch, error) {
	lb := newObj.(*lbv1.LoadBalancer)

	ap, err := m.getAnnotationsPatch(lb)
	if err != nil {
		return nil, err
	}

	hcp, err := m.getHealthyCheckPatch(lb)
	if err != nil {
		return nil, err
	}

	if len(ap) == 0 {
		return hcp, nil
	}
	return append(ap, hcp...), nil
}

func (m *mutator) Update(_ *admission.Request, _, newObj runtime.Object) (admission.Patch, error) {
	lb := newObj.(*lbv1.LoadBalancer)

	if lb.DeletionTimestamp != nil {
		return nil, nil
	}

	ap, err := m.getAnnotationsPatch(lb)
	if err != nil {
		return nil, err
	}

	hcp, err := m.getHealthyCheckPatch(lb)
	if err != nil {
		return nil, err
	}

	if len(ap) == 0 {
		return hcp, nil
	}
	return append(ap, hcp...), nil
}

// those fields are not checked in the past, necessary to overwrite them to at least 1
func (m *mutator) getHealthyCheckPatch(lb *lbv1.LoadBalancer) (admission.Patch, error) {
	if lb.Spec.HealthCheck == nil || lb.Spec.HealthCheck.Port == 0 {
		return nil, nil
	}

	hc := *lb.Spec.HealthCheck
	patched := false

	if hc.SuccessThreshold == 0 {
		hc.SuccessThreshold = 2
		patched = true
	}

	if hc.FailureThreshold == 0 {
		hc.FailureThreshold = 2
		patched = true
	}

	if hc.PeriodSeconds == 0 {
		hc.PeriodSeconds = 1
		patched = true
	}

	if hc.TimeoutSeconds == 0 {
		hc.TimeoutSeconds = 1
		patched = true
	}

	if patched {
		return []admission.PatchOp{
			{
				Op:    admission.PatchOpReplace,
				Path:  "/spec/healthCheck",
				Value: hc,
			},
		}, nil
	}

	return nil, nil
}

// for VM type LB, it does not expose the concept of Project, Network
func (m *mutator) getAnnotationsPatchVM(lb *lbv1.LoadBalancer) (admission.Patch, error) {
	// already patched
	if lb.Annotations[utils.AnnotationKeyNamespace] == lb.Namespace {
		return nil, nil
	}

	annotations := lb.Annotations
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[utils.AnnotationKeyNamespace] = lb.Namespace
	return []admission.PatchOp{
		{
			Op:    admission.PatchOpReplace,
			Path:  "/metadata/annotations",
			Value: annotations,
		},
	}, nil
}

// for Cluster type LB
func (m *mutator) getAnnotationsPatchCluster(lb *lbv1.LoadBalancer) (admission.Patch, error) {
	if lb.Spec.WorkloadType != lbv1.Cluster {
		return nil, nil
	}

	project, err := m.findProject(lb.Namespace)
	if err != nil {
		return nil, err
	}

	network, err := m.findNetwork(lb, lb.Annotations[utils.AnnotationKeyCluster])
	if err != nil {
		return nil, err
	}

	// per the carried annotation like `loadbalancer.harvesterhci.io/cluster: gc1`
	// additional annotations are mutated
	//
	//   loadbalancer.harvesterhci.io/namespace: default
	//   loadbalancer.harvesterhci.io/network: default/vm-untag
	//   loadbalancer.harvesterhci.io/project: c-q4xz6/p-6vvfz

	annotations := lb.Annotations
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[utils.AnnotationKeyNamespace] = lb.Namespace
	annotations[utils.AnnotationKeyProject] = project
	annotations[utils.AnnotationKeyNetwork] = network

	return []admission.PatchOp{
		{
			Op:    admission.PatchOpReplace,
			Path:  "/metadata/annotations",
			Value: annotations,
		},
	}, nil
}

func (m *mutator) getAnnotationsPatch(lb *lbv1.LoadBalancer) (admission.Patch, error) {
	if lb.Spec.WorkloadType == lbv1.VM {
		return m.getAnnotationsPatchVM(lb)
	}
	return m.getAnnotationsPatchCluster(lb)
}

func (m *mutator) Resource() admission.Resource {
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

// Find ProjectID through the namespace
func (m *mutator) findProject(namespace string) (string, error) {
	ns, err := m.namespaceCache.Get(namespace)
	if err != nil {
		return "", fmt.Errorf("get namespace %s failed, error: %w", namespace, err)
	}

	// the valid format is like `c-q4xz6:p-6vvfz`
	return strings.Replace(ns.Annotations[rancherproject.ProjectIDAnn], ":", "/", 1), nil
}

// findNetwork identifies the target network for the guest cluster using a priority-based approach:
// 1. Explicit Network: AnnotationKeyGuestClusterNetworkNameOnLB
// 2. Management Network: AnnotationKeyGuestClusterManagementNetworkOnLB
// 3. Discovery (New): Use both creator and cluster-name to match, it ensures the vm is belonging to this cluster.
// 4. Discovery (Legacy): It selects vm by creator label, and then match by vm name prefix.
//
// Note: This assumes all VMs in the guest cluster share the same namespace and network configuration.
// The remote application is responsible for the correctness of the appointed network name;
// if it's wrong, it could lead to a failure to allocate IPs from the target pool.
func (m *mutator) findNetwork(lb *lbv1.LoadBalancer, clusterName string) (string, error) {
	// 1 & 2: Check annotations first
	if net := lb.Annotations[utils.AnnotationKeyGuestClusterNetworkNameOnLB]; net != "" {
		return net, nil
	}
	if net := lb.Annotations[utils.AnnotationKeyGuestClusterManagementNetworkOnLB]; net != "" {
		return net, nil
	}

	// 3: Modern Discovery (Label-based)
	// Use both creator and cluster-name to match, it ensures the vm is belonging to this cluster
	modernSelector := utils.NewGuestClusterNameAndCreatorNameSelecotr(clusterName)
	modernVMIs, err := m.vmiCache.List(lb.Namespace, modernSelector)
	if err != nil {
		return "", fmt.Errorf("list vmis with modern selector failed: %w", err)
	}
	if name, found := getFirstMultusNetworkName(modernVMIs); found {
		return name, nil
	}

	// 4: Legacy Fallback (label + name prefix)
	// It selects vm by creator label, and then match by vm name prefix
	legacySelector := utils.NewGuestClusterCreatorSelecotr()
	legacyVMIs, err := m.vmiCache.List(lb.Namespace, legacySelector)
	if err != nil {
		return "", fmt.Errorf("list vmis with legacy selector failed: %w", err)
	}
	if name, found := findNetworkByLegacyNameMatch(legacyVMIs, clusterName); found {
		return name, nil
	}

	return "", nil
}

// getFirstMultusNetworkName searches a list of VMIs and returns the first Multus network name found.
func getFirstMultusNetworkName(vmis []*kubevirtv1.VirtualMachineInstance) (string, bool) {
	for _, vmi := range vmis {
		for _, network := range vmi.Spec.Networks {
			if network.Multus != nil {
				return network.Multus.NetworkName, true
			}
		}
	}
	return "", false
}

// findNetworkByLegacyNameMatch filters VMIs by name prefix and returns the first Multus network name found.
func findNetworkByLegacyNameMatch(vmis []*kubevirtv1.VirtualMachineInstance, clusterName string) (string, bool) {
	for _, vmi := range vmis {
		if strings.HasPrefix(vmi.Name, clusterName) {
			// Reuse the spec-traversal helper for consistency
			if name, found := getFirstMultusNetworkName([]*kubevirtv1.VirtualMachineInstance{vmi}); found {
				return name, true
			}
		}
	}
	return "", false
}
