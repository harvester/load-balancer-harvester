package loadbalancer

import (
	"fmt"
	"strings"

	ctlkubevirtv1 "github.com/harvester/harvester/pkg/generated/controllers/kubevirt.io/v1"
	"github.com/harvester/webhook/pkg/server/admission"
	rancherproject "github.com/rancher/rancher/pkg/project"
	ctlcorev1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	admissionregv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
	"github.com/harvester/harvester-load-balancer/pkg/utils"
)

const (
	keyCreator          = "harvesterhci.io/creator"
	harvesterNodeDriver = "docker-machine-driver-harvester"
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

	return m.getAnnotationsPatch(lb)
}

func (m *mutator) Update(_ *admission.Request, _, newObj runtime.Object) (admission.Patch, error) {
	lb := newObj.(*lbv1.LoadBalancer)

	if lb.DeletionTimestamp != nil {
		return nil, nil
	}

	return m.getAnnotationsPatch(lb)
}

func (m *mutator) getAnnotationsPatch(lb *lbv1.LoadBalancer) (admission.Patch, error) {
	project, err := m.findProject(lb.Namespace)
	if err != nil {
		return nil, err
	}

	var network string
	if lb.Spec.WorkloadType == lbv1.Cluster && lb.Annotations != nil && lb.Annotations[utils.AnnotationKeyCluster] != "" {
		network, err = m.findNetwork(lb.Namespace, lb.Annotations[utils.AnnotationKeyCluster])
		if err != nil {
			return nil, err
		}
	}

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

	return strings.Replace(ns.Annotations[rancherproject.ProjectIDAnn], ":", "/", 1), nil
}

// Find the first network where the guest cluster is running
// We assume that all the virtual machines composed the guest cluster are running in the same namespace and
// have the same network configuration.
func (m *mutator) findNetwork(namespace, clusterName string) (string, error) {
	// list all the vmi instance in the same namespace of the load balancer
	vmis, err := m.vmiCache.List(namespace, labels.Set(map[string]string{
		keyCreator: harvesterNodeDriver,
	}).AsSelector())
	if err != nil {
		return "", fmt.Errorf("list vmis failed, error: %w", err)
	}

	// find the first network of the first vmi whose name starts with the cluster name
	for _, vmi := range vmis {
		if strings.HasPrefix(vmi.Name, clusterName) {
			for _, network := range vmi.Spec.Networks {
				if network.NetworkSource.Multus != nil {
					return network.NetworkSource.Multus.NetworkName, nil
				}
			}
		}
	}

	return "", nil
}
