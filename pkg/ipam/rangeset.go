package ipam

import (
	"github.com/containernetworking/plugins/plugins/ipam/host-local/backend/allocator"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
)

func LBRangesToAllocatorRangeSet(ranges []lbv1.Range) (allocator.RangeSet, error) {
	ars := make([]allocator.Range, len(ranges))
	for i := range ranges {
		ar, err := MakeRange(&ranges[i])
		if err != nil {
			return nil, err
		}
		ars[i] = *ar
	}

	return ars, nil
}
