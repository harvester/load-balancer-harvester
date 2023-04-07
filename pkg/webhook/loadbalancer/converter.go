package loadbalancer

import (
	"fmt"

	"github.com/harvester/webhook/pkg/server/conversion"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubevirtv1 "kubevirt.io/api/core/v1"

	ctlkubevirtv1 "github.com/harvester/harvester/pkg/generated/controllers/kubevirt.io/v1"

	lbv1alpha1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1alpha1"
	lbv1beta1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
	"github.com/harvester/harvester-load-balancer/pkg/controller/ippool/kubevip"
	ctllbv1 "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/loadbalancer.harvesterhci.io/v1beta1"
	"github.com/harvester/harvester-load-balancer/pkg/lb/servicelb"
)

const keyVmName = "harvesterhci.io/vmName"

type converter struct {
	vmiCache    ctlkubevirtv1.VirtualMachineInstanceCache
	ippoolCache ctllbv1.IPPoolCache
}

var _ conversion.Converter = &converter{}

func NewConverter(vmiCache ctlkubevirtv1.VirtualMachineInstanceCache, ippoolCache ctllbv1.IPPoolCache) conversion.Converter {
	return &converter{
		vmiCache:    vmiCache,
		ippoolCache: ippoolCache,
	}
}

func (c *converter) GroupResource() schema.GroupResource {
	return lbv1beta1.Resource(lbv1beta1.LoadBalancerResourceName)
}

func (c *converter) Convert(obj *unstructured.Unstructured, toVersion string) (*unstructured.Unstructured, error) {
	fromVersion := obj.GetAPIVersion()
	logrus.Debugf("convert %s from %q to %q, obj: %s/%s", obj.GetKind(), fromVersion, toVersion, obj.GetNamespace(), obj.GetName())

	if fromVersion == toVersion {
		return nil, fmt.Errorf("conversion from a version to itself should not call the webhook: %s", toVersion)
	}

	convertedObj := obj.DeepCopy()
	convertedObj.SetAPIVersion(toVersion)

	switch toVersion {
	case lbv1beta1.SchemeGroupVersion.String():
		if err := c.convertFromV1alpha1ToV1beta1(convertedObj); err != nil {
			return nil, err
		}
	case lbv1alpha1.SchemeGroupVersion.String():
		if err := c.convertFromV1beta1ToV1alpha1(convertedObj); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unexpected conversion version %q", toVersion)
	}

	return convertedObj, nil
}

func (c *converter) convertFromV1alpha1ToV1beta1(obj *unstructured.Unstructured) error {
	spec := obj.Object["spec"].(map[string]interface{})
	// spec.pool and status.allocatedAddress
	status := obj.Object["status"].(map[string]interface{})
	addr, ok := status["address"]
	if ok {
		if spec["ipam"] == string(lbv1beta1.Pool) {
			pool, err := c.getPoolByAddressOfV1alpha1LB(addr.(string), obj.GetName(), obj.GetNamespace())
			if err != nil {
				return err
			}
			spec["ipPool"] = pool
			status["allocatedAddress"] = lbv1beta1.AllocatedAddress{
				IPPool: pool,
				IP:     addr.(string),
			}
		} else {
			status["allocatedAddress"] = lbv1beta1.AllocatedAddress{
				IP: servicelb.Address4AskDHCP,
			}
		}
	}
	// listeners
	if spec["listeners"] != nil {
		listeners := spec["listeners"].([]interface{})
		v1beta1Listeners := make([]lbv1beta1.Listener, 0, len(listeners))
		for _, listener := range listeners {
			l := listener.(map[string]interface{})
			v1beta1Listeners = append(v1beta1Listeners, lbv1beta1.Listener{
				Name:        l["name"].(string),
				Port:        int32(l["port"].(int64)),
				Protocol:    corev1.Protocol(l["protocol"].(string)),
				BackendPort: int32(l["backendPort"].(int64)),
			})
		}
		spec["listeners"] = v1beta1Listeners
	}
	// backendServerSelector
	if spec["backendServers"] != nil {
		backendServers := make([]string, len(spec["backendServers"].([]interface{})))
		for i, server := range spec["backendServers"].([]interface{}) {
			backendServers[i] = server.(string)
		}
		selector, err := c.convertBackendServersToBackendServerSelector(backendServers)
		if err != nil {
			return err
		}
		spec["backendServerSelector"] = selector
		status["backendServers"] = backendServers
	}

	obj.Object["spec"] = spec
	obj.Object["status"] = status

	return nil
}

func (c *converter) convertFromV1beta1ToV1alpha1(obj *unstructured.Unstructured) error {
	spec := obj.Object["spec"].(map[string]interface{})
	// listeners
	if spec["listeners"] != nil {
		listeners := spec["listeners"].([]interface{})
		v1alpha1Listeners := make([]*lbv1alpha1.Listener, 0, len(listeners))
		for _, listener := range listeners {
			l := listener.(map[string]interface{})
			v1alpha1Listeners = append(v1alpha1Listeners, &lbv1alpha1.Listener{
				Name:        l["name"].(string),
				Port:        int32(l["port"].(int64)),
				Protocol:    corev1.Protocol(l["protocol"].(string)),
				BackendPort: int32(l["backendPort"].(int64)),
			})
		}
		spec["listeners"] = v1alpha1Listeners
	}

	// BackendServers
	status := obj.Object["status"].(map[string]interface{})
	spec["backendServers"] = status["backendServers"]

	obj.Object["spec"] = spec

	return nil
}

func (c *converter) convertBackendServersToBackendServerSelector(backendServers []string) (map[string][]string, error) {
	vmis, err := c.vmiCache.List("", labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("list vmi failed, error: %v", err)
	}
	addrVmiMap := make(map[string]*kubevirtv1.VirtualMachineInstance, len(vmis))
	for _, vmi := range vmis {
		s := &servicelb.Server{VirtualMachineInstance: vmi}
		addr, ok := s.GetAddress()
		if ok {
			addrVmiMap[addr] = vmi
		}
	}

	selector := map[string][]string{
		keyVmName: make([]string, 0, len(vmis)),
	}
	for _, backendServer := range backendServers {
		vmi, ok := addrVmiMap[backendServer]
		if !ok {
			continue
		}
		selector[keyVmName] = append(selector[keyVmName], vmi.Name)
	}

	return selector, nil
}

// The address allocated by the pool with the name as the namespace or the global pool
func (c *converter) getPoolByAddressOfV1alpha1LB(addr, lbName, lbNamespace string) (string, error) {
	name := fmt.Sprintf("%s/%s", lbNamespace, lbName)
	// Find from the pool with the name as the namespace first
	pool, err := c.ippoolCache.Get(lbNamespace)
	if err != nil {
		return "", err
	}
	if applicant, ok := pool.Status.Allocated[addr]; ok && applicant == name {
		return lbNamespace, nil
	}
	// Find from the global pool
	pool, err = c.ippoolCache.Get(kubevip.GlobalIPPoolName)
	if err != nil {
		return "", err
	}
	if applicant, ok := pool.Status.Allocated[addr]; ok && applicant == name {
		return lbNamespace, nil
	}

	return "", fmt.Errorf("address %s is not allocated by the %s pool or the global pool", addr, lbNamespace)
}
