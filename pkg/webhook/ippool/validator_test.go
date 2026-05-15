package ippool

import (
	"net"
	"path/filepath"
	"testing"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/plugins/plugins/ipam/host-local/backend/allocator"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
	"github.com/harvester/harvester-load-balancer/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/harvester-load-balancer/pkg/ipam"
	"github.com/harvester/harvester-load-balancer/pkg/utils"
	"github.com/harvester/harvester-load-balancer/pkg/utils/fakeclients"
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
		rs, err := ipam.LBRangesToAllocatorRangeSet(r)
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

func TestCheckAllocated(t *testing.T) {
	_, ipNet, _ := net.ParseCIDR("192.168.0.0/24")
	rs := allocator.RangeSet([]allocator.Range{
		{
			RangeStart: net.ParseIP("192.168.0.10"),
			RangeEnd:   net.ParseIP("192.168.0.20"),
			Subnet:     types.IPNet(*ipNet),
			Gateway:    net.ParseIP("192.168.0.1"),
		},
	})
	allocatedMaps := []map[string]string{
		{
			"192.168.0.11": "",
			"192.168.0.12": "",
		},
		{
			"xxxxxxxx": "",
		},
		{
			"192.168.0.11": "",
			"192.168.0.30": "",
		},
		{
			"192.168.1.1": "",
		},
	}

	expected := []bool{true, false, false, false}

	for i, allocatedMap := range allocatedMaps {
		if err := checkAllocated(rs, allocatedMap); (err == nil) != expected[i] {
			t.Errorf("case%d failed, checkAllocated(%v, %v)", i, rs, allocatedMap)
		}
	}
}

func TestCheckSelector(t *testing.T) {
	cases, err := utils.GetSubdirectories("./testdata")
	if err != nil {
		t.Error(err)
	}
	type expectedResult struct {
		description string
		pass        bool
	}
	expected := map[string]expectedResult{
		"case1": {
			description: "It's only allowed to have one global IP pool for one network, new global IP pool is denied",
			pass:        false,
		},
		"case2": {
			description: "New global IP pool is allowed to add when there is no global IP pool",
			pass:        true,
		},
		"case3": {
			description: "There is no scope overlaps of the input IP pool itself",
			pass:        true,
		},
		"case4": {
			description: "There are scope overlaps of the input IP pool itself",
			pass:        false,
		},
		"case5": {
			description: "The input IP pool has different priority with the existing IP pools",
			pass:        true,
		},
		"case6": {
			description: "The input IP pool has the same priority with the existing IP pools",
			pass:        false,
		},
		"case7": {
			description: "There is no scope overlap of the input IP pool with the existing IP pools",
			pass:        true,
		},
		"case8": {
			description: "There are scope overlaps of the input IP pool with the existing IP pools",
			pass:        false,
		},
		"case9": {
			description: "Each of the two networks has one global IP pool",
			pass:        true,
		},
		"case10": {
			description: "The input IP pool has the same priority with the existing IP pools, but they are from different networks",
			pass:        true,
		},
	}

	for _, c := range cases {
		// load test data
		cacheObjs, err := utils.ParseFromFile(filepath.Join("./testdata", c, "cache.yaml"))
		if err != nil {
			t.Errorf("test %s failed, error: %v", c, err)
		}
		clientSet := fake.NewSimpleClientset(cacheObjs...)
		input, err := utils.ParseFromFile(filepath.Join("./testdata", c, "input.yaml"))
		if err != nil {
			t.Errorf("test %s failed, error: %v", c, err)
		}
		if len(input) != 1 {
			t.Errorf("test %s failed, input length is not 1", c)
		}

		validator := &ipPoolValidator{
			ipPoolCache: fakeclients.IPPoolCache(clientSet.LoadbalancerV1beta1().IPPools),
		}
		pool := input[0].(*lbv1.IPPool)
		if err := validator.checkSelector(pool); (err == nil) != expected[c].pass {
			t.Errorf("test %s %s failed, input: %+v, error: %v", c, expected[c].description, pool, err)
		}
	}
}
