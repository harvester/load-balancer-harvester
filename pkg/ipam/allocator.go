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
	cnip "github.com/containernetworking/plugins/pkg/ip"
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
	ip, ipNet, err := net.ParseCIDR(r.Subnet)
	if err != nil {
		return nil, fmt.Errorf("invalid range %+v", r)
	}

	var defaultStart, defaultEnd, defaultGateway, start, end, gateway net.IP
	mask := ipNet.Mask.String()
	// If the subnet is a point to point IP
	if mask == p2pMaskStr {
		defaultStart = ip.To16()
		defaultEnd = ip.To16()
		defaultGateway = nil
	} else {
		// The rangeStart defaults to `.1` IP inside the `subnet` block.
		// The rangeEnd defaults to `.254` IP inside the `subnet` block for ipv4, `.255` for IPv6.
		// The gateway defaults to `.1` IP inside the `subnet` block.
		// Example:
		// 	  subnet: 192.168.0.0/24
		// 	  rangeStart: 192.168.0.1
		// 	  rangeEnd: 192.168.0.254
		// 	  gateway: 192.168.0.1
		// The gateway will be skipped during allocation.
		// To return the IP with 16 bytes representation as same as what the function net.ParseIP returns
		defaultStart = cnip.NextIP(ipNet.IP).To16()
		defaultEnd = lastIP(*ipNet).To16()
		defaultGateway = cnip.NextIP(ipNet.IP).To16()
	}

	start, err = parseIP(r.RangeStart, ipNet, defaultStart)
	if err != nil {
		return nil, fmt.Errorf("invalid range start %s: %w", r.RangeStart, err)
	}
	end, err = parseIP(r.RangeEnd, ipNet, defaultEnd)
	if err != nil {
		return nil, fmt.Errorf("invalid range end %s: %w", r.RangeEnd, err)
	}
	gateway, err = parseIP(r.Gateway, ipNet, defaultGateway)
	if err != nil {
		return nil, fmt.Errorf("invalid gateway %s: %w", r.Gateway, err)
	}

	// Ensure start IP is smaller than end IP
	startAddr, _ := netip.AddrFromSlice(start)
	endAddr, _ := netip.AddrFromSlice(end)
	if startAddr.Compare(endAddr) > 0 {
		start, end = end, start
	}

	return &allocator.Range{
		RangeStart: start,
		RangeEnd:   end,
		Subnet:     types.IPNet(*ipNet),
		Gateway:    gateway,
	}, nil
}

func parseIP(ipStr string, ipNet *net.IPNet, defaultIP net.IP) (net.IP, error) {
	if ipStr == "" {
		return defaultIP, nil
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP %s", ipStr)
	}
	if !ipNet.Contains(ip) {
		return nil, fmt.Errorf("IP %s is out of subnet %s", ipStr, ipNet.String())
	}
	if ip.Equal(networkIP(*ipNet)) {
		return nil, fmt.Errorf("IP %s is the network address", ipStr)
	}
	if ip.Equal(broadcastIP(*ipNet)) {
		return nil, fmt.Errorf("IP %s is the broadcast address", ipStr)
	}

	return ip, nil
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

func networkIP(n net.IPNet) net.IP {
	return n.IP.Mask(n.Mask)
}

func broadcastIP(n net.IPNet) net.IP {
	broadcast := make(net.IP, len(n.IP))
	for i := 0; i < len(n.IP); i++ {
		broadcast[i] = n.IP[i] | ^n.Mask[i]
	}
	return broadcast
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
