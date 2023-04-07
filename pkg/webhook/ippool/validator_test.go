package ippool

import (
	"testing"

	"github.com/containernetworking/plugins/plugins/ipam/host-local/backend/allocator"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
	"github.com/harvester/harvester-load-balancer/pkg/utils"
)

func TestCheckRange(t *testing.T) {
	ranges := [][]lbv1.Range{
		{
			{
				RangeStart: "192.168.100.10",
				RangeEnd:   "192.168.100.20",
				Subnet:     "192.168.100.0/24",
			},
			{
				RangeStart: "192.168.100.30",
				RangeEnd:   "192.168.100.40",
				Subnet:     "192.168.100.0/24",
			},
		},
		{
			{
				Subnet: "192.168.100.0/24",
			},
			{
				RangeStart: "192.168.100.10",
				RangeEnd:   "192.168.100.20",
				Subnet:     "192.168.100.0/24",
			},
		},
		{
			{
				Subnet: "192.168.100.0/24",
			},
		},
		{
			{
				RangeStart: "192.168.100.50",
				RangeEnd:   "192.168.100.60",
				Subnet:     "192.168.100.0/24",
			},
		},
	}

	rss := make([]allocator.RangeSet, 4)
	for i, r := range ranges {
		rs, err := utils.LBRangesToAllocatorRangeSet(r)
		if err != nil {
			t.Fatalf("transfer %v to rangeset failed, error: %s", r, err.Error())
		}
		rss[i] = rs
	}
	if err := checkRange(rss[0]); err != nil {
		t.Errorf("case1 failed, checkRange(%v)", rss[0])
	}

	if err := checkRange(rss[1]); err == nil {
		t.Errorf("case2 failed, checkRange(%+v)", rss[1])
	}

	if err := checkRange(rss[0], rss[2]); err == nil {
		t.Errorf("case3 failed, checkRange(%+v, %+v)", rss[0], rss[2])
	}

	if err := checkRange(rss[0], rss[3]); err != nil {
		t.Errorf("case4 failed, checkRange(%v, %v)", rss[0], rss[3])
	}
}
