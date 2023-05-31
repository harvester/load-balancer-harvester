package ipam

import (
	"fmt"
	"net"
	"testing"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/plugins/plugins/ipam/host-local/backend/allocator"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
	"github.com/harvester/harvester-load-balancer/pkg/ipam/store"
)

var (
	cClassSubnet        = "192.168.100.0/24"
	cClassSubnetIP      = net.IP{192, 168, 100, 0}
	cClassSubnetMask    = net.IPv4Mask(255, 255, 255, 0)
	cClassSubnetIPStart = net.IP{192, 168, 100, 1}
	cClassSubnetIPEnd   = net.IP{192, 168, 100, 254}
	p2pIPStr            = "192.168.100.10/32"
	p2pIP               = net.IP{192, 168, 100, 10}
	p2pMask             = net.IPv4Mask(255, 255, 255, 255)
)

func newFakeAllocator(name string, ranges []lbv1.Range) (*Allocator, error) {
	if len(ranges) == 0 {
		return nil, fmt.Errorf("range could not be empty")
	}

	rangeSlice := make([]allocator.Range, 0)
	var total int64

	for i := range ranges {
		element, err := MakeRange(&ranges[i])
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
	a1, err := newFakeAllocator(name, []lbv1.Range{{Subnet: cClassSubnet}})
	if err != nil {
		t.Fatalf("failed to create allocator %s, error: %s", name, err.Error())
	}

	name = "a2"
	a2, err := newFakeAllocator(name, []lbv1.Range{
		{
			Subnet:     cClassSubnet,
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
			Subnet:     cClassSubnet,
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
			Subnet:     cClassSubnet,
			RangeStart: "192.168.100.251",
		},
	})
	if err != nil {
		t.Fatalf("failed to create allocator %s, error: %s", name, err.Error())
	}

	name = "a5"
	a5, err := newFakeAllocator(name, []lbv1.Range{
		{
			Subnet:   cClassSubnet,
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

func TestMakeRange(t *testing.T) {
	tests := []struct {
		name    string
		r       *lbv1.Range
		want    *allocator.Range
		wantErr bool
	}{
		{
			name: "p2pIPNet",
			r: &lbv1.Range{
				Subnet: p2pIPStr,
			},
			want: &allocator.Range{
				Subnet: types.IPNet(net.IPNet{
					IP:   p2pIP,
					Mask: p2pMask,
				}),
				RangeStart: p2pIP,
				RangeEnd:   p2pIP,
			},
			wantErr: false,
		},
		{
			name: "p2pIPNetWithWrongRangeStart",
			r: &lbv1.Range{
				Subnet:     p2pIPStr,
				RangeStart: "192.168.101.10",
			},
			wantErr: true,
		},
		{
			name: "cClassIPNet",
			r: &lbv1.Range{
				Subnet: cClassSubnet,
			},
			want: &allocator.Range{
				Subnet: types.IPNet(net.IPNet{
					IP:   cClassSubnetIP,
					Mask: cClassSubnetMask,
				}),
				RangeStart: cClassSubnetIPStart,
				RangeEnd:   cClassSubnetIPEnd,
				Gateway:    cClassSubnetIPStart,
			},
			wantErr: false,
		},
		{
			name: "cClassIPNetWithRangeStartAndEnd",
			r: &lbv1.Range{
				Subnet:     cClassSubnet,
				RangeStart: "192.168.100.30",
				RangeEnd:   "192.168.100.40",
			},
			want: &allocator.Range{
				Subnet: types.IPNet(net.IPNet{
					IP:   cClassSubnetIP,
					Mask: cClassSubnetMask,
				}),
				RangeStart: net.ParseIP("192.168.100.30"),
				RangeEnd:   net.ParseIP("192.168.100.40"),
				Gateway:    cClassSubnetIPStart,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		got, err := MakeRange(tt.r)
		if (err != nil) != tt.wantErr || !rangesEqual(got, tt.want) {
			fmt.Printf("got: %v, %v, %v. %v\n", got.Subnet, got.RangeStart, got.RangeEnd, got.Gateway)
			fmt.Printf("want: %v, %v, %v. %v\n", tt.want.Subnet, tt.want.RangeStart, tt.want.RangeEnd, tt.want.Gateway)
			t.Errorf("test case %s failed, got = (%v,%v,%v.%v), want (%v,%v,%v,%v), err = %v, wantErr %v",
				tt.name, got.Subnet, got.RangeStart, got.RangeEnd, got.Gateway,
				tt.want.Subnet, tt.want.RangeStart, tt.want.RangeEnd, tt.want.Gateway, err, tt.wantErr)
		}
	}
}

func rangesEqual(r1, r2 *allocator.Range) bool {
	if r1 == nil || r2 == nil {
		return r1 == r2
	}

	return r1.RangeStart.Equal(r2.RangeStart) &&
		r1.RangeEnd.Equal(r2.RangeEnd) &&
		r1.Subnet.IP.Equal(r2.Subnet.IP) &&
		r1.Subnet.Mask.String() == r2.Subnet.Mask.String() &&
		r1.Gateway.Equal(r2.Gateway)
}
