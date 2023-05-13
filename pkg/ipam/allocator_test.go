package ipam

import (
	"fmt"
	"testing"

	"github.com/containernetworking/plugins/plugins/ipam/host-local/backend/allocator"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
	"github.com/harvester/harvester-load-balancer/pkg/ipam/store"
)

const subnet = "192.168.100.0/24"

func newFakeAllocator(name string, ranges []lbv1.Range) (*Allocator, error) {
	if len(ranges) == 0 {
		return nil, fmt.Errorf("range could not be empty")
	}

	rangeSlice := make([]allocator.Range, 0)
	var total int64

	for _, r := range ranges {
		element, err := MakeRange(&r)
		if err != nil {
			return nil, err
		}

		rangeSlice = append(rangeSlice, *element)
		total += countIP(element)
	}

	rangeSet := allocator.RangeSet(rangeSlice)
	return &Allocator{
		name:        name,
		IPAllocator: allocator.NewIPAllocator(&rangeSet, store.NewFakeStore(name, ranges), 0),
		checkSum:    CalculateCheckSum(ranges),
		total:       total,
	}, nil
}

func TestAllocator_Total(t *testing.T) {
	name := "a1"
	a1, err := newFakeAllocator(name, []lbv1.Range{{Subnet: subnet}})
	if err != nil {
		t.Fatalf("failed to create allocator %s, error: %s", name, err.Error())
	}

	name = "a2"
	a2, err := newFakeAllocator(name, []lbv1.Range{
		{
			Subnet:     subnet,
			RangeStart: "192.168.100.1",
			RangeEnd:   "192.168.100.10",
			Gateway:    "192.168.100.11",
		},
	})
	if err != nil {
		t.Fatalf("failed to create allocator %s, error: %s", name, err.Error())
	}

	name = "a3"
	a3, err := newFakeAllocator(name, []lbv1.Range{
		{
			Subnet:     subnet,
			RangeStart: "192.168.100.1",
			RangeEnd:   "192.168.100.10",
		},
	})
	if err != nil {
		t.Fatalf("failed to create allocator %s, error: %s", name, err.Error())
	}

	name = "a4"
	a4, err := newFakeAllocator(name, []lbv1.Range{
		{
			Subnet:     subnet,
			RangeStart: "192.168.100.251",
		},
	})
	if err != nil {
		t.Fatalf("failed to create allocator %s, error: %s", name, err.Error())
	}

	name = "a5"
	a5, err := newFakeAllocator(name, []lbv1.Range{
		{
			Subnet:   subnet,
			RangeEnd: "192.168.100.10",
		},
	})
	if err != nil {
		t.Fatalf("failed to create allocator %s, error: %s", name, err.Error())
	}

	tests := []struct {
		name      string
		allocator *Allocator
		want      int64
	}{
		{
			name:      "subnetTotal",
			allocator: a1,
			want:      int64(253),
		},
		{
			name:      "rangeTotal",
			allocator: a2,
			want:      int64(10),
		},
		{
			name:      "rangeExcludeDefaultGatewayTotal",
			allocator: a3,
			want:      int64(9),
		},
		{
			name:      "rangeWithRangeStart",
			allocator: a4,
			want:      int64(4),
		},
		{
			name:      "rangeWithRangeEnd",
			allocator: a5,
			want:      int64(9),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Allocator{
				IPAllocator: tt.allocator.IPAllocator,
				name:        tt.allocator.name,
				checkSum:    tt.allocator.checkSum,
				total:       tt.allocator.total,
				cache:       tt.allocator.cache,
			}
			if got := a.Total(); got != tt.want {
				t.Errorf("Allocator.Total() = %v, want %v", got, tt.want)
			}
		})
	}
}
