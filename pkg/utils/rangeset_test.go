package utils

import (
	"testing"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
)

func TestLBRangesToAllocatorRangeSet(t *testing.T) {
	rangeLists := [][]lbv1.Range{
		{
			{
				RangeStart: "192.168.0.10",
				RangeEnd:   "192.168.0.20",
				Subnet:     "192.168.0.0/24",
			},
		},
		{
			{
				Subnet: "192.168.0.0/24",
			},
		},
		{
			{
				RangeStart: "192.168.0.10",
				RangeEnd:   "192.168.0.20",
				Subnet:     "192.168.0.0/24",
			},
			{
				Subnet: "192.168.10.0/24",
			},
		},
		{
			{
				RangeStart: "192.168.0.20",
				RangeEnd:   "192.168.0.10",
				Subnet:     "192.168.0.0/24",
			},
		},
		{
			{
				RangeStart: "192.168.0.1",
				RangeEnd:   "192.168.0.10",
			},
		},
		{
			{
				RangeStart: "192.168.0.10",
				Subnet:     "192.168.10.0/24",
			},
		},
	}
	expected := []struct {
		isErr    bool
		rangeStr string
	}{
		{isErr: false, rangeStr: "192.168.0.10-192.168.0.20"},
		{isErr: false, rangeStr: "192.168.0.1-192.168.0.254"},
		{isErr: false, rangeStr: "192.168.0.10-192.168.0.20,192.168.10.1-192.168.10.254"},
		{isErr: false, rangeStr: "192.168.0.10-192.168.0.20"},
		{isErr: true},
		{isErr: true},
	}

	for i, r := range rangeLists {
		rs, err := LBRangesToAllocatorRangeSet(r)
		if (err != nil) != expected[i].isErr {
			if expected[i].isErr {
				t.Errorf("expect to return error but not")
			} else {
				t.Errorf("expect to return no error but got error %q", err.Error())
			}
			continue
		}
		if err == nil && rs.String() != expected[i].rangeStr {
			t.Errorf("LBRangesToAllocatorRangeSet() got = %v, want %v", rs.String(), expected[i].rangeStr)
		}
	}
}
