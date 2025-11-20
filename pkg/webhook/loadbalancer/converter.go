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

	lbv1alpha1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1alpha1"
	lbv1beta1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
	ctlkubevirtv1 "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/kubevirt.io/v1"
	ctllbv1 "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/loadbalancer.harvesterhci.io/v1beta1"
	"github.com/harvester/harvester-load-balancer/pkg/lb/servicelb"
	"github.com/harvester/harvester-load-balancer/pkg/utils"
)

const keyVMName = "harvesterhci.io/vmName"

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
	spec := obj.Object[keySpec].(map[string]interface{})
	// spec.pool and status.allocatedAddress
	status := obj.Object[keyStatus].(map[string]interface{})
	addr, ok := status[keyAddress]
	if ok {
		if spec[keyIPAM] == string(lbv1beta1.Pool) {
			pool, err := c.getPoolByAddressOfV1alpha1LB(addr.(string), obj.GetName(), obj.GetNamespace())
			if err != nil {
				return err
			}
			spec[keyIPPool] = pool
			status[keyAllocatedAddress] = lbv1beta1.AllocatedAddress{
				IPPool: pool,
				IP:     addr.(string),
			}
		} else {
			status[keyAllocatedAddress] = lbv1beta1.AllocatedAddress{
				IP: utils.Address4AskDHCP,
			}
		}
	}
	// listeners
	if spec[keyListeners] != nil {
		listeners := spec[keyListeners].([]interface{})
		v1beta1Listeners := make([]lbv1beta1.Listener, 0, len(listeners))
		for _, listener := range listeners {
			l := listener.(map[string]interface{})
			v1beta1Listeners = append(v1beta1Listeners, lbv1beta1.Listener{
				Name: l[keyName].(string),
				//#nosec
				Port:     int32(l[keyPort].(int64)),
				Protocol: corev1.Protocol(l[keyProtocol].(string)),
				//#nosec
				BackendPort: int32(l[keyBackendPort].(int64)),
			})
		}
		spec[keyListeners] = v1beta1Listeners
	}
	// backendServerSelector
	if spec[keyBackendServers] != nil {
		backendServers := make([]string, len(spec[keyBackendServers].([]interface{})))
		for i, server := range spec[keyBackendServers].([]interface{}) {
			backendServers[i] = server.(string)
		}
		selector, err := c.convertBackendServersToBackendServerSelector(backendServers, obj.GetNamespace())
		if err != nil {
			return err
		}
		spec[keyBackendServerSelector] = selector
		status[keyBackendServers] = backendServers
	}

	obj.Object[keySpec] = spec
	obj.Object[keyStatus] = status

	return nil
}

func (c *converter) convertFromV1beta1ToV1alpha1(obj *unstructured.Unstructured) error {
	spec := obj.Object[keySpec].(map[string]interface{})
	// listeners
	if spec[keyListeners] != nil {
		listeners := spec[keyListeners].([]interface{})
		v1alpha1Listeners := make([]*lbv1alpha1.Listener, 0, len(listeners))
		for _, listener := range listeners {
			l := listener.(map[string]interface{})
			v1alpha1Listeners = append(v1alpha1Listeners, &lbv1alpha1.Listener{
				Name: l[keyName].(string),
				//#nosec
				Port:     int32(l[keyPort].(int64)),
				Protocol: corev1.Protocol(l[keyProtocol].(string)),
				//#nosec
				BackendPort: int32(l[keyBackendPort].(int64)),
			})
		}
		spec[keyListeners] = v1alpha1Listeners
	}

	// BackendServers
	if obj.Object[keyStatus] != nil {
		status := obj.Object[keyStatus].(map[string]interface{})
		spec[keyBackendServers] = status[keyBackendServers]
	}

	obj.Object[keySpec] = spec

	return nil
}

// convertBackendServersToBackendServerSelector converts backendServers to backendServerSelector
func (c *converter) convertBackendServersToBackendServerSelector(backendServers []string, namespace string) (map[string][]string, error) {
	// Backend servers are in the same namespace with the LB
	vmis, err := c.vmiCache.List(namespace, labels.Everything())
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
		keyVMName: make([]string, 0, len(vmis)),
	}
	for _, backendServer := range backendServers {
		vmi, ok := addrVmiMap[backendServer]
		if !ok {
			continue
		}
		selector[keyVMName] = append(selector[keyVMName], vmi.Name)
	}

	return selector, nil
}

// List all the pools and find the pool which has allocated the address of the lb
func (c *converter) getPoolByAddressOfV1alpha1LB(addr, lbName, lbNamespace string) (string, error) {
	name := fmt.Sprintf("%s/%s", lbNamespace, lbName)

	pools, err := c.ippoolCache.List(labels.Everything())
	if err != nil {
		return "", err
	}

	for _, pool := range pools {
		if pool.Status.Allocated[addr] == name {
			logrus.Infof("pool: %s, name: %s", pool.Name, name)
			return pool.Name, nil
		}
	}

	return "", fmt.Errorf("not found pool for lb %s with address %s", name, addr)
}
