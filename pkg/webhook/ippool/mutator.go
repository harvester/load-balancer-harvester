package ippool

import (
	"fmt"
	"strconv"

	"github.com/harvester/webhook/pkg/server/admission"
	admissionregv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
	ctlcniv1 "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/k8s.cni.cncf.io/v1"
	"github.com/harvester/harvester-load-balancer/pkg/ipam"
	"github.com/harvester/harvester-load-balancer/pkg/utils"
)

type ipPoolMutator struct {
	admission.DefaultMutator
	nadCache ctlcniv1.NetworkAttachmentDefinitionCache
}

var _ admission.Mutator = &ipPoolMutator{}

func NewIPPoolMutator(nadCache ctlcniv1.NetworkAttachmentDefinitionCache) admission.Mutator {
	return &ipPoolMutator{
		nadCache: nadCache,
	}
}

// Create method is called when creating a new IPPool.
// Add a patch operation to set the VID of the IPPool.
func (i *ipPoolMutator) Create(_ *admission.Request, newObj runtime.Object) (admission.Patch, error) {
	pool := newObj.(*lbv1.IPPool)
	patch, err := i.getLabelPatch(pool)
	if err != nil {
		return nil, fmt.Errorf(createErr, pool.Name, err)
	}

	return patch, nil
}

// Update method is called when updating an existing IPPool.
// Add a patch operation to set the VID of the IPPool.
func (i *ipPoolMutator) Update(_ *admission.Request, _, newObj runtime.Object) (admission.Patch, error) {
	pool := newObj.(*lbv1.IPPool)

	if pool.DeletionTimestamp != nil {
		return nil, nil
	}

	patch, err := i.getLabelPatch(pool)
	if err != nil {
		return nil, fmt.Errorf(updateErr, pool.Name, err)
	}

	return patch, nil
}

func (i *ipPoolMutator) Resource() admission.Resource {
	return admission.Resource{
		Names:      []string{"ippools"},
		Scope:      admissionregv1.ClusterScope,
		APIGroup:   lbv1.SchemeGroupVersion.Group,
		APIVersion: lbv1.SchemeGroupVersion.Version,
		ObjectType: &lbv1.IPPool{},
		OperationTypes: []admissionregv1.OperationType{
			admissionregv1.Create,
			admissionregv1.Update,
		},
	}
}

func (i *ipPoolMutator) getLabelPatch(pool *lbv1.IPPool) (admission.Patch, error) {
	patch := admission.Patch{}

	// TODO: If the net-attach-def changed, the VLAN ID may change.
	vid, err := utils.GetVid(pool.Spec.Selector.Network, i.nadCache)
	if err != nil {
		return patch, err
	}

	labels := pool.Labels
	if labels == nil {
		labels = make(map[string]string)
	}

	vidStr := strconv.Itoa(vid)
	isGlobalStr := strconv.FormatBool(isGlobalIPPool(pool))
	if labels[utils.KeyVid] == vidStr && labels[utils.KeyGlobalIPPool] == isGlobalStr {
		return patch, nil
	}

	labels[utils.KeyVid] = vidStr
	labels[utils.KeyGlobalIPPool] = isGlobalStr

	return append(patch, admission.PatchOp{
		Op:    admission.PatchOpReplace,
		Path:  "/metadata/labels",
		Value: labels,
	}), nil
}

func isGlobalIPPool(pool *lbv1.IPPool) bool {
	return len(pool.Spec.Selector.Scope) == 1 && pool.Spec.Selector.Scope[0].Namespace == ipam.All &&
		pool.Spec.Selector.Scope[0].Project == ipam.All && pool.Spec.Selector.Scope[0].GuestCluster == ipam.All
}
