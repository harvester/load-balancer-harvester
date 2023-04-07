package kubevip

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/containernetworking/plugins/plugins/ipam/host-local/backend/allocator"
	ctlcorev1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
	"github.com/harvester/harvester-load-balancer/pkg/ipam"
	"github.com/harvester/harvester-load-balancer/pkg/utils"
)

const (
	kubevipIPPoolConfigMap = "kubevip"
	kubevipDataKey         = "kubevip-services"
	GlobalIPPoolName       = "global"
	formatCidr             = "cidr"
	formatRange            = "range"
)

var invalidFormatErr = errors.New("invalid format")

type IPPoolConverter struct {
	cmCache ctlcorev1.ConfigMapCache
	// GlobalIPPoolName pool is a special pool that is used to allocate IP for the namespace where no other pools are defined.
	GlobalIPPoolNamePool     *lbv1.IPPool
	GlobalIPPoolNameRangeSet allocator.RangeSet
}

func NewIPPoolConverter(cmCache ctlcorev1.ConfigMapCache) *IPPoolConverter {
	return &IPPoolConverter{
		cmCache: cmCache,
	}
}

// ConvertFromKubevipConfigMap converts the kubevip configmap to be IPPools.
func (c *IPPoolConverter) ConvertFromKubevipConfigMap() ([]*lbv1.IPPool, error) {
	cm, err := c.cmCache.Get("kube-system", kubevipIPPoolConfigMap)
	if apierrors.IsNotFound(err) {
		return []*lbv1.IPPool{}, nil
	} else if err != nil {
		return nil, err
	}

	pools := make([]*lbv1.IPPool, 0, len(cm.Data))
	kubevipConfigs := make([][2]string, 0, len(cm.Data))
	// GlobalIPPoolName pool should be the first one in the list
	for k, v := range cm.Data {
		if strings.Contains(k, GlobalIPPoolName) {
			kubevipConfigs = append([][2]string{{k, v}}, kubevipConfigs...)
		} else {
			kubevipConfigs = append(kubevipConfigs, [2]string{k, v})
		}
	}

	for _, config := range kubevipConfigs {
		pool, err := c.convertKubevipIPPool(config[0], config[1])
		if err != nil && !errors.Is(err, invalidFormatErr) {
			return nil, fmt.Errorf("convert kubevip IP pool %s:%s failed, %w", config[0], config[1], err)
		} else if errors.Is(err, invalidFormatErr) {
			continue
		}
		pools = append(pools, pool)
	}

	return pools, nil
}

// Parse GlobalIPPoolName or namespace and IP range from configmap data item <key, value> and convert it to be an IPPool.
// data item examples:
// cidr-default: 172.16.10.0/24
// cidr-default: 172.16.10.0/24,172.16.20.0/24
// range-default: 172.16.10.10-172.16.10.100
// range-default: 172.16.10.10-172.16.10.100,172.16.10.150-172.16.10.200
// As a special pool with key range-global or cidr-global, we will convert it as global IP pool.
func (c *IPPoolConverter) convertKubevipIPPool(key, value string) (*lbv1.IPPool, error) {
	// parse key to get the IP pool format and namespace
	format, name, err := getFormatAndName(key)
	if err != nil {
		return nil, err
	}
	// parse kube-vip IP pool according to the format
	ranges, err := parseKubevipIPPool(value, format)
	if err != nil {
		return nil, err
	}
	// make IPPool according the name, IP ranges. If it is the global IP pool, we will set it to IPPoolConverter.
	pool := makeIPPool(name, ranges)
	if name == GlobalIPPoolName {
		c.GlobalIPPoolNamePool = pool
		c.GlobalIPPoolNameRangeSet, err = utils.LBRangesToAllocatorRangeSet(pool.Spec.Ranges)
		if err != nil {
			return nil, err
		}
	}
	// assign allocated IPs
	if err := c.assignAllocatedIPs(pool); err != nil {
		return nil, err
	}
	return pool, nil
}

func getFormatAndName(in string) (string, string, error) {
	s := strings.SplitN(in, "-", 2)
	if len(s) != 2 {
		return "", "", fmt.Errorf("invalid input %s", in)
	}

	return s[0], s[1], nil
}

func parseKubevipIPPool(pool, format string) ([]lbv1.Range, error) {
	multipleRanges := strings.Split(pool, ",")

	poolRanges := make([]lbv1.Range, 0, len(multipleRanges))
	for _, r := range multipleRanges {
		poolRange := lbv1.Range{}
		switch format {
		case formatCidr:
			_, ipNet, err := net.ParseCIDR(r)
			if err != nil {
				return nil, err
			}
			poolRange.Subnet = ipNet.String()
		case formatRange:
			s := strings.Split(r, "-")
			if len(s) != 2 || net.ParseIP(s[0]) == nil || net.ParseIP(s[1]) == nil {
				return nil, fmt.Errorf("invalid range %s", r)
			}
			poolRange.RangeStart, poolRange.RangeEnd = s[0], s[1]
			// get ip net according to range start and range end
			ipNet, err := getIPNet(poolRange.RangeStart, poolRange.RangeEnd)
			if err != nil {
				return nil, fmt.Errorf("get IP net failed for range %s faield, error: %w", r, err)
			}
			poolRange.Subnet = ipNet.String()
		default:
			return nil, invalidFormatErr
		}
		poolRanges = append(poolRanges, poolRange)
	}

	return poolRanges, nil
}

func getIPNet(startIP, endIP string) (*net.IPNet, error) {
	start := net.ParseIP(startIP)
	if start == nil {
		return nil, fmt.Errorf("invalid start IP address: %s", startIP)
	}

	end := net.ParseIP(endIP)
	if end == nil {
		return nil, fmt.Errorf("invalid end IP address: %s", endIP)
	}

	if bytes := len(start.To4()); bytes != len(end.To4()) {
		return nil, fmt.Errorf("IP version mismatch between start (%s) and end (%s) IP addresses", startIP, endIP)
	}

	// Determine the prefix length of the network
	prefixLen := 0
	for i := 31; i >= 0; i-- {
		mask := net.CIDRMask(i, 32)
		if start.Mask(mask).Equal(end.Mask(mask)) {
			prefixLen = i
			break
		}
	}

	// Create the IP network
	ipNet := &net.IPNet{
		IP:   start.Mask(net.CIDRMask(prefixLen, 32)),
		Mask: net.CIDRMask(prefixLen, 32),
	}

	return ipNet, nil
}

// Assign allocated IPs to the IPPool. If the IP is not in the pool, try to assign to GlobalIPPoolName IP pool.
func (c *IPPoolConverter) assignAllocatedIPs(pool *lbv1.IPPool) error {
	if pool.Name == GlobalIPPoolName {
		return nil
	}

	cm, err := c.cmCache.Get(pool.Name, kubevipIPPoolConfigMap)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	} else if apierrors.IsNotFound(err) {
		return nil
	}

	kubevipServices, ok := cm.Data[kubevipDataKey]
	if !ok {
		return nil
	}

	services, err := decodeKubeVIPServicesJSON(kubevipServices)
	if err != nil {
		return err
	}
	rangeSet, err := utils.LBRangesToAllocatorRangeSet(pool.Spec.Ranges)
	if err != nil {
		return err
	}
	for _, service := range services.Services {
		ip := net.ParseIP(service.VIP)
		if ip == nil {
			logrus.Warnf("invalid IP %s of kubevip service %+v", service.VIP, service)
			continue
		}
		if rangeSet.Contains(ip) {
			pool.Status.Allocated[service.VIP] = pool.Name + "/" + service.ServiceName
		} else if c.GlobalIPPoolNamePool != nil && c.GlobalIPPoolNameRangeSet != nil && c.GlobalIPPoolNameRangeSet.Contains(ip) {
			c.GlobalIPPoolNamePool.Status.Allocated[service.VIP] = pool.Name + "/" + service.ServiceName
		}
	}

	return nil
}

// make IPPool according the namespace, IP ranges and allocatedIPs
func makeIPPool(name string, ranges []lbv1.Range) *lbv1.IPPool {
	var selector lbv1.Selector
	if name != GlobalIPPoolName {
		selector.Scope = []lbv1.Tuple{
			{
				Project:      ipam.All,
				Namespace:    name,
				GuestCluster: ipam.All,
			},
		}
	}

	pool := &lbv1.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: lbv1.IPPoolSpec{
			Ranges:   ranges,
			Selector: selector,
		},
		Status: lbv1.IPPoolStatus{
			Allocated: make(map[string]string),
		},
	}

	return pool
}

type kubeVIPService struct {
	VIP         string `json:"vip"`
	ServiceName string `json:"serviceName"`
}

type kubeVIPServices struct {
	Services []kubeVIPService `json:"services"`
}

func decodeKubeVIPServicesJSON(jsonString string) (kubeVIPServices, error) {
	var services kubeVIPServices
	err := json.Unmarshal([]byte(jsonString), &services)
	if err != nil {
		return services, fmt.Errorf("failed to decode JSON: %v", err)
	}
	return services, nil
}
