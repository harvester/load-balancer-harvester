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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	lb "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io"
	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
	"github.com/harvester/harvester-load-balancer/pkg/ipam"
	"github.com/harvester/harvester-load-balancer/pkg/utils"
)

const (
	kubeSystemNamespace    = "kube-system"
	kubevipIPPoolConfigMap = "kubevip"
	kubevipDataKey         = "kubevip-services"
	GlobalIPPoolName       = "global"
	formatCidr             = "cidr"
	formatRange            = "range"

	keyAfterConversion = lb.GroupName + "/after-conversion"
)

var errInvalidFormat = errors.New("invalid format")

type IPPoolConverter struct {
	cmClient ctlcorev1.ConfigMapClient
	// GlobalIPPoolName pool is a special pool that is used to allocate IP for the namespace where no other pools are defined.
	GlobalIPPoolNamePool     *lbv1.IPPool
	GlobalIPPoolNameRangeSet allocator.RangeSet
}

func NewIPPoolConverter(cmClient ctlcorev1.ConfigMapClient) *IPPoolConverter {
	return &IPPoolConverter{
		cmClient: cmClient,
	}
}

// ConvertFromKubevipConfigMap converts the kubevip configmap to be IPPools.
func (c *IPPoolConverter) ConvertFromKubevipConfigMap() ([]*lbv1.IPPool, error) {
	cm, err := c.cmClient.Get(kubeSystemNamespace, kubevipIPPoolConfigMap, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return []*lbv1.IPPool{}, nil
	} else if err != nil {
		return nil, err
	}
	// If the configmap has been converted, return empty pools.
	if cm.Annotations != nil && cm.Annotations[keyAfterConversion] == utils.ValueTrue {
		return []*lbv1.IPPool{}, nil
	}

	pools := make([]*lbv1.IPPool, 0, len(cm.Data))
	kubevipConfigs := make([][2]string, 0, len(cm.Data))
	// GlobalIPPoolName pool should be the first one in the list so that it will be recorded in the IPPoolConverter
	// before assigning allocated IPs to pools.
	for k, v := range cm.Data {
		if k == formatCidr+"-"+GlobalIPPoolName || k == formatRange+"-"+GlobalIPPoolName {
			kubevipConfigs = append([][2]string{{k, v}}, kubevipConfigs...)
		} else {
			kubevipConfigs = append(kubevipConfigs, [2]string{k, v})
		}
	}

	logrus.Infof("kubevipConfigs: %+v", kubevipConfigs)

	for _, config := range kubevipConfigs {
		pool, err := c.convertKubevipIPPool(config[0], config[1])
		if err != nil && !errors.Is(err, errInvalidFormat) {
			return nil, fmt.Errorf("convert kubevip IP pool %s:%s failed, %w", config[0], config[1], err)
		} else if errors.Is(err, errInvalidFormat) {
			logrus.Errorf("invalid config %s", config)
			continue
		}
		logrus.Infof("convert kubevip IP pool %s to IPPool %s whose IP ranges are %v", config, pool.Name, pool.Spec.Ranges)
		pools = append(pools, pool)
	}

	if err := c.assignAllAllocatedIPs(pools); err != nil {
		return nil, fmt.Errorf("assign allocated IPs to pools failed, %w", err)
	}

	return pools, nil
}

// AfterConversion will add the annotation into kubevip configmap to tag that the conversion is done.
func (c *IPPoolConverter) AfterConversion() error {
	cm, err := c.cmClient.Get(kubeSystemNamespace, kubevipIPPoolConfigMap, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	if cm.Annotations != nil && cm.Annotations[keyAfterConversion] == utils.ValueTrue {
		return nil
	}

	cmCopy := cm.DeepCopy()
	if cmCopy.Annotations == nil {
		cmCopy.Annotations = make(map[string]string)
	}
	cmCopy.Annotations[keyAfterConversion] = utils.ValueTrue
	if _, err = c.cmClient.Update(cmCopy); err != nil {
		return err
	}
	return nil
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
		c.GlobalIPPoolNameRangeSet, err = ipam.LBRangesToAllocatorRangeSet(pool.Spec.Ranges)
		if err != nil {
			return nil, err
		}
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
			return nil, errInvalidFormat
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
// Please record the global IP pool into the IPPoolConverter first if existing.
func (c *IPPoolConverter) assignAllAllocatedIPs(pools []*lbv1.IPPool) error {
	m := make(map[string]*lbv1.IPPool, len(pools))
	for _, pool := range pools {
		m[pool.Name] = pool
	}

	cms, err := c.cmClient.List(metav1.NamespaceAll, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list configmaps failed, error: %w", err)
	}

	for i, cm := range cms.Items {
		// skip the configmap which is not kube-vip IP pool configmap
		if cm.Name != kubevipIPPoolConfigMap {
			continue
		}
		if err := c.assignAllocatedIPs(&cms.Items[i], m[cm.Namespace]); err != nil {
			return fmt.Errorf("assign allocated IPs from configmap %s/%s failed, error: %w", cm.Namespace, cm.Name, err)
		}
	}

	return nil
}

// assignAllocatedIPs assigns allocated IPs to the IPPool.
// If the IP is not in the pool, try to assign to GlobalIPPoolName IP pool.
// If the pool is nil, try to assign to GlobalIPPoolName IP pool.
func (c *IPPoolConverter) assignAllocatedIPs(cm *corev1.ConfigMap, pool *lbv1.IPPool) error {
	kubevipServices, ok := cm.Data[kubevipDataKey]
	if !ok {
		return nil
	}

	services, err := decodeKubeVIPServicesJSON(kubevipServices)
	if err != nil {
		return err
	}

	var rangeSet allocator.RangeSet
	if pool != nil {
		rangeSet, err = ipam.LBRangesToAllocatorRangeSet(pool.Spec.Ranges)
		if err != nil {
			return err
		}
	}

	for _, service := range services.Services {
		if service.VIP == utils.Address4AskDHCP {
			continue
		}
		ip := net.ParseIP(service.VIP)
		if ip == nil {
			logrus.Warnf("invalid IP %s of kubevip service %+v", service.VIP, service)
			continue
		}
		// assign the IP to the pool, if it is not in the pool, try to assign to GlobalIPPoolName IP pool
		if pool != nil && len(rangeSet) > 0 && rangeSet.Contains(ip) {
			logrus.Infof("ip %s in the pool %s", ip, pool.Name)
			pool.Status.Allocated[service.VIP] = cm.Namespace + "/" + service.ServiceName
		} else if c.GlobalIPPoolNamePool != nil && c.GlobalIPPoolNameRangeSet != nil && c.GlobalIPPoolNameRangeSet.Contains(ip) {
			logrus.Infof("ip %s in the global pool", ip)
			c.GlobalIPPoolNamePool.Status.Allocated[service.VIP] = cm.Namespace + "/" + service.ServiceName
		}
	}

	return nil
}

// make IPPool according the namespace, IP ranges and allocatedIPs
func makeIPPool(name string, ranges []lbv1.Range) *lbv1.IPPool {
	selector := lbv1.Selector{
		Scope: []lbv1.Tuple{
			{
				Project:      ipam.All,
				Namespace:    name,
				GuestCluster: ipam.All,
			},
		},
	}
	if name == GlobalIPPoolName {
		selector.Scope[0].Namespace = ipam.All
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
