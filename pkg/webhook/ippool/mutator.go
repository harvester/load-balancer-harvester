package ippool

import (
	"fmt"
	"strconv"

	ctlcniv1 "github.com/harvester/harvester/pkg/generated/controllers/k8s.cni.cncf.io/v1"
	"github.com/harvester/webhook/pkg/server/admission"
	admissionregv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
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

func (i *ipPoolMutator) Create(_ *admission.Request, newObj runtime.Object) (admission.Patch, error) {
	pool := newObj.(*lbv1.IPPool)
	vidPatchOp, err := i.getVidPatchOp(pool)
	if err != nil {
		return nil, fmt.Errorf(createErr, pool.Name, err)
	}

	if vidPatchOp != nil {
		return admission.Patch{*vidPatchOp}, nil
	}

	return nil, nil
}

func (i *ipPoolMutator) Update(_ *admission.Request, _, newObj runtime.Object) (admission.Patch, error) {
	pool := newObj.(*lbv1.IPPool)
	var patch admission.Patch

	vidPatchOp, err := i.getVidPatchOp(pool)
	if err != nil {
		return nil, fmt.Errorf(updateErr, pool.Name, err)
	}
	if vidPatchOp != nil {
		patch = append(patch, *vidPatchOp)
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

// getVidPatchOp returns a patch operation to set the vid label.
// TODO: If the net-attach-def changed, the VLAN ID may change.
func (i *ipPoolMutator) getVidPatchOp(pool *lbv1.IPPool) (*admission.PatchOp, error) {
	vid, err := utils.GetVid(pool.Spec.Selector.Network, i.nadCache)
	if err != nil {
		return nil, err
	}
	labels := pool.Labels
	if labels == nil {
		labels = make(map[string]string)
	}
	vidStr := strconv.Itoa(vid)
	if labels[utils.KeyVid] == vidStr {
		return nil, nil
	}

	labels[utils.KeyVid] = vidStr
	return &admission.PatchOp{
		Op:    admission.PatchOpReplace,
		Path:  "/metadata/labels",
		Value: labels,
	}, nil
}
