package loadbalancer

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	harvesterfake "github.com/harvester/harvester/pkg/generated/clientset/versioned/fake"
	harvesterfakeclients "github.com/harvester/harvester/pkg/util/fakeclients"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	lbv1alpha1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1alpha1"
	lbv1beta1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
	"github.com/harvester/harvester-load-balancer/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/harvester-load-balancer/pkg/utils"
	"github.com/harvester/harvester-load-balancer/pkg/utils/fakeclients"
)

func TestConverter_Convert(t *testing.T) {
	vmis, err := utils.ParseFromFile(filepath.Join("./testdata/vmi.yaml"))
	if err != nil {
		t.Error(err)
	}
	pools, err := utils.ParseFromFile(filepath.Join("./testdata/pool.yaml"))
	if err != nil {
		t.Error(err)
	}
	virtualMachineInstanceCache := harvesterfakeclients.VirtualMachineInstanceCache(harvesterfake.NewSimpleClientset(vmis...).KubevirtV1().VirtualMachineInstances)
	ippoolCache := fakeclients.IPPoolCache(fake.NewSimpleClientset(pools...).LoadbalancerV1beta1().IPPools)
	converter := NewConverter(virtualMachineInstanceCache, ippoolCache)
	cases, err := utils.GetSubdirectories("./testdata")
	if err != nil {
		t.Error(err)
	}

	for _, c := range cases {
		v1alpha1LoadBalancer, v1beta1LoadBalancer, err := getLBResourceFromYAMLFile(filepath.Join("./testdata", c, "loadbalancer.yaml"))
		if err != nil {
			t.Errorf("test %s failed, error: %v", c, err)
		}
		if err := convert(converter, v1alpha1LoadBalancer, v1beta1LoadBalancer, lbv1beta1.SchemeGroupVersion.String()); err != nil {
			t.Errorf("test %s to convert from v1aplha1 to v1beta1 failed, error: %v", c, err)
		}
		if err := convert(converter, v1beta1LoadBalancer, v1alpha1LoadBalancer, lbv1alpha1.SchemeGroupVersion.String()); err != nil {
			t.Errorf("test %s to convert from v1beta1 to v1alpha1 failed, error: %v", c, err)
		}
	}
}

func getLBResourceFromYAMLFile(filepath string) (*lbv1alpha1.LoadBalancer, *lbv1beta1.LoadBalancer, error) {
	lbs, err := utils.ParseFromFile(filepath)
	if err != nil {
		return nil, nil, err
	}
	if len(lbs) != 2 {
		return nil, nil, fmt.Errorf("every case should have two loadbalancer resources")
	}
	var v1alpha1LoadBalancer *lbv1alpha1.LoadBalancer
	var v1beta1LoadBalancer *lbv1beta1.LoadBalancer
	for _, lb := range lbs {
		if lb.GetObjectKind().GroupVersionKind().Version == "v1alpha1" {
			v1alpha1LoadBalancer = lb.(*lbv1alpha1.LoadBalancer)
		} else if lb.GetObjectKind().GroupVersionKind().Version == "v1beta1" {
			v1beta1LoadBalancer = lb.(*lbv1beta1.LoadBalancer)
		}
	}

	return v1alpha1LoadBalancer, v1beta1LoadBalancer, nil
}

func convert(converter *converter, obj, expectedObj runtime.Object, toVersion string) error {
	expectedObjVersion := expectedObj.GetObjectKind().GroupVersionKind().GroupVersion().String()
	if toVersion != expectedObjVersion {
		return fmt.Errorf("expected version %s is not same as toVersion %s", expectedObjVersion, toVersion)
	}

	unstructured, err := toUnstructured(obj)
	if err != nil {
		return err
	}
	convertedUnstructured, err := converter.Convert(unstructured, toVersion)
	if err != nil {
		return err
	}

	convertedObj, err := toObj(convertedUnstructured)
	if err != nil {
		return err
	}

	if !reflect.DeepEqual(convertedObj, expectedObj) {
		return fmt.Errorf("\nexpected: %+v\n got:     %+v\n", expectedObj, convertedObj)
	}

	return nil
}

func toUnstructured(obj runtime.Object) (*unstructured.Unstructured, error) {
	// Marshal the object to JSON
	data, err := runtime.Encode(unstructured.UnstructuredJSONScheme, obj)
	if err != nil {
		return nil, err
	}
	// Unmarshal the JSON into an unstructured object
	unstructuredObj := &unstructured.Unstructured{}
	err = unstructuredObj.UnmarshalJSON(data)
	if err != nil {
		return nil, err
	}

	return unstructuredObj, nil
}

func toObj(unstructured *unstructured.Unstructured) (runtime.Object, error) {
	data, err := json.Marshal(unstructured.Object)
	if err != nil {
		return nil, err
	}

	groupVersion := unstructured.GroupVersionKind().GroupVersion().String()
	switch groupVersion {
	case lbv1alpha1.SchemeGroupVersion.String():
		obj := &lbv1alpha1.LoadBalancer{}
		if err := json.Unmarshal(data, obj); err != nil {
			return nil, err
		}
		return obj, nil
	case lbv1beta1.SchemeGroupVersion.String():
		obj := &lbv1beta1.LoadBalancer{}
		if err := json.Unmarshal(data, obj); err != nil {
			return nil, err
		}
		return obj, nil
	default:
		return nil, fmt.Errorf("unknown group version %s", groupVersion)
	}
}
