package ipam

import (
	"crypto/sha256"
	"fmt"
	"math/big"
	"net"
	"net/netip"
	"sync"

	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/plugins/ipam/host-local/backend/allocator"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
	ctllbv1 "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/loadbalancer.harvesterhci.io/v1beta1"
	"github.com/harvester/harvester-load-balancer/pkg/ipam/store"
)

const p2pMaskStr = "ffffffff"

type Allocator struct {
	*allocator.IPAllocator
	name     string
	checkSum string
	total    int64
	cache    ctllbv1.IPPoolCache
}

type SafeAllocatorMap struct {
	allocators map[string]*Allocator
	mutex      sync.RWMutex
}

func NewAllocator(name string, ranges []lbv1.Range, cache ctllbv1.IPPoolCache, client ctllbv1.IPPoolClient) (*Allocator, error) {
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
		IPAllocator: allocator.NewIPAllocator(&rangeSet, store.New(name, cache, client), 0),
		checkSum:    CalculateCheckSum(ranges),
		total:       total,
		cache:       cache,
	}, nil
}

func MakeRange(r *lbv1.Range) (*allocator.Range, error) {
	ipv4, ipNet, err := net.ParseCIDR(r.Subnet)
	if err != nil {
		return nil, fmt.Errorf("invalide range %+v", r)
	}

	var start, end, gateway net.IP
	mask := ipNet.Mask.String()
	// The rangeStart defaults to “.1” IP inside the “subnet” block.
	if r.RangeStart == "" {
		// To return the IP with 16 bytes representation as same as what the function net.ParseIP returns
		if mask == p2pMaskStr {
			start = ipv4.To16()
		} else {
			start = ip.NextIP(ipNet.IP).To16()
		}
	} else {
		start = net.ParseIP(r.RangeStart)
		if start == nil {
			return nil, fmt.Errorf("invalid rangeStart %s", r.RangeStart)
		}

		if !ipNet.Contains(start) {
			return nil, fmt.Errorf("range start IP %s is out of subnet %s", start.String(), ipNet.String())
		}
	}

	// The rangeEnd defaults to “.254” IP inside the “subnet” block for ipv4, “.255” for IPv6.
	if r.RangeEnd == "" {
		if mask == p2pMaskStr {
			end = ipv4.To16()
		} else {
			end = lastIP(*ipNet).To16()
		}
	} else {
		end = net.ParseIP(r.RangeEnd)
		if end == nil {
			return nil, fmt.Errorf("invalid rangeEnd %s", r.RangeEnd)
		}
		if !ipNet.Contains(end) {
			return nil, fmt.Errorf("range end IP %s is out of subnet %s", start.String(), ipNet.String())
		}
	}

	// Ensure start IP is smaller than end IP
	startAddr, _ := netip.AddrFromSlice(start)
	endAddr, _ := netip.AddrFromSlice(end)
	if startAddr.Compare(endAddr) > 0 {
		start, end = end, start
	}

	// The gateway defaults to “.1” IP inside the “subnet” block.
	// If the subnet is point to point IP, leave the gateway as empty
	// The gateway will be skipped during allocation.
	if r.Gateway == "" {
		if mask == p2pMaskStr {
			gateway = nil
		} else {
			gateway = ip.NextIP(ipNet.IP).To16()
		}
	} else {
		gateway = net.ParseIP(r.Gateway)
		if gateway == nil {
			return nil, fmt.Errorf("invalid gateway %s", r.Gateway)
		}
	}

	return &allocator.Range{
		RangeStart: start,
		RangeEnd:   end,
		Subnet:     types.IPNet(*ipNet),
		Gateway:    gateway,
	}, nil
}

// Determine the last IP of a subnet, excluding the broadcast if IPv4
func lastIP(subnet net.IPNet) net.IP {
	var end net.IP
	for i := 0; i < len(subnet.IP); i++ {
		end = append(end, subnet.IP[i]|^subnet.Mask[i])
	}
	if subnet.IP.To4() != nil {
		end[3]--
	}

	return end
}

func countIP(r *allocator.Range) int64 {
	count := big.NewInt(0).Add(big.NewInt(0).Sub(ipToInt(r.RangeEnd), ipToInt(r.RangeStart)), big.NewInt(1)).Int64()

	if r.Gateway != nil && r.Contains(r.Gateway) {
		count--
	}

	return count
}

func ipToInt(ip net.IP) *big.Int {
	if v := ip.To4(); v != nil {
		return big.NewInt(0).SetBytes(v)
	}
	return big.NewInt(0).SetBytes(ip.To16())
}

func (a *Allocator) CheckSum() string {
	return a.checkSum
}

func (a *Allocator) Total() int64 {
	return a.total
}

func (a *Allocator) Get(id string) (*current.IPConfig, error) {
	pool, err := a.cache.Get(a.name)
	if err != nil {
		return nil, err
	}

	// apply the IP allocated before in priority
	if pool.Status.AllocatedHistory != nil {
		for k, v := range pool.Status.AllocatedHistory {
			if id == v {
				return a.IPAllocator.Get(id, "", net.ParseIP(k))
			}
		}
	}

	return a.IPAllocator.Get(id, "", nil)
}

func CalculateCheckSum(ranges []lbv1.Range) string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%v", ranges)))

	return fmt.Sprintf("%x", h.Sum(nil))
}

func NewSafeAllocatorMap() *SafeAllocatorMap {
	return &SafeAllocatorMap{
		allocators: make(map[string]*Allocator),
		mutex:      sync.RWMutex{},
	}
}

func (s *SafeAllocatorMap) AddOrUpdate(name string, allocator *Allocator) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.allocators[name] = allocator
}

func (s *SafeAllocatorMap) Delete(name string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.allocators, name)
}

func (s *SafeAllocatorMap) Get(name string) *Allocator {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.allocators[name]
}
