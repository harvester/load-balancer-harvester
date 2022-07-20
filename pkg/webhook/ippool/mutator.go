package ippool

import (
	"fmt"
	"net"
	"strconv"

	ctlcniv1 "github.com/harvester/harvester/pkg/generated/controllers/k8s.cni.cncf.io/v1"
	"github.com/yaocw2020/webhook/pkg/types"
	admissionregv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1alpha1"
	"github.com/harvester/harvester-load-balancer/pkg/utils"
)

type ipPoolMutator struct {
	types.DefaultMutator
	nadCache ctlcniv1.NetworkAttachmentDefinitionCache
}

var _ types.Mutator = &ipPoolMutator{}

func NewIPPoolMutator(nadCache ctlcniv1.NetworkAttachmentDefinitionCache) types.Mutator {
	return &ipPoolMutator{
		nadCache: nadCache,
	}
}

func (i *ipPoolMutator) Create(_ *types.Request, newObj runtime.Object) (types.Patch, error) {
	pool := newObj.(*lbv1.IPPool)
	vidPatchOp, err := i.getVidPatchOp(pool)
	if err != nil {
		return nil, fmt.Errorf(createErr, pool.Name, err)
	}

	if vidPatchOp != nil {
		return types.Patch{*vidPatchOp}, nil
	}

	return nil, nil
}

func (i *ipPoolMutator) Update(_ *types.Request, _, newObj runtime.Object) (types.Patch, error) {
	pool := newObj.(*lbv1.IPPool)
	var patch types.Patch

	allocatedHistoryPatchOp, err := getAllocatedHistoryPatchOp(pool)
	if err != nil {
		return nil, fmt.Errorf(updateErr, pool.Name, err)
	}
	if allocatedHistoryPatchOp != nil {
		patch = append(patch, *allocatedHistoryPatchOp)
	}

	vidPatchOp, err := i.getVidPatchOp(pool)
	if err != nil {
		return nil, fmt.Errorf(updateErr, pool.Name, err)
	}
	if vidPatchOp != nil {
		patch = append(patch, *vidPatchOp)
	}

	return patch, nil
}

func (i *ipPoolMutator) Resource() types.Resource {
	return types.Resource{
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

func getAllocatedHistoryPatchOp(pool *lbv1.IPPool) (*types.PatchOp, error) {
	rs, err := toAllocatorRangeSet(pool.Spec.Ranges)
	if err != nil {
		return nil, err
	}

	patchValue := map[string]string{}
	for ipStr := range pool.Status.AllocatedHistory {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			return nil, fmt.Errorf("invalid ip %s", ipStr)
		}

		if rs.Contains(ip) {
			patchValue[ipStr] = pool.Status.AllocatedHistory[ipStr]
		}
	}

	if len(patchValue) != 0 {
		return &types.PatchOp{
			Op:    types.PatchOpReplace,
			Path:  "/status/allocatedHistory",
			Value: patchValue,
		}, nil
	}

	return nil, nil
}

func (i *ipPoolMutator) getVidPatchOp(pool *lbv1.IPPool) (*types.PatchOp, error) {
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
	return &types.PatchOp{
		Op:    types.PatchOpReplace,
		Path:  "/metadata/labels",
		Value: labels,
	}, nil
}
