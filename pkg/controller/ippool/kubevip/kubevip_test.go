package kubevip

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/harvester/harvester/pkg/util/fakeclients"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corefake "k8s.io/client-go/kubernetes/fake"

	"github.com/harvester/harvester-load-balancer/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/harvester-load-balancer/pkg/utils"
)

func TestConvertFromKubevipConfigMap(t *testing.T) {
	cases, err := utils.GetSubdirectories("./testdata")
	if err != nil {
		t.Error(err)
	}

	for _, c := range cases {
		objs, err := utils.ParseFromFile(filepath.Join("./testdata", c, "configmap.yaml"))
		if err != nil {
			t.Errorf("test %s failed, error: %v", c, err)
		}
		coreClientset := corefake.NewSimpleClientset(objs...)
		cmClient := fakeclients.ConfigmapCache(coreClientset.CoreV1().ConfigMaps)
		fakeConverter := NewIPPoolConverter(cmClient)
		pools, err := fakeConverter.ConvertFromKubevipConfigMap()
		if err != nil {
			t.Errorf("test %s failed, error: %v", c, err)
		}

		expectedPools, err := utils.ParseFromFile(filepath.Join("./testdata", c, "ippool.yaml"))
		lbClientset := fake.NewSimpleClientset(expectedPools...)
		for _, pool := range pools {
			expectedPool, err := lbClientset.LoadbalancerV1beta1().IPPools().Get(context.TODO(), pool.Name, metav1.GetOptions{})
			if err != nil {
				t.Errorf("test %s failed, error: %v", c, err)
			}
			if !reflect.DeepEqual(pool.Spec, expectedPool.Spec) || !reflect.DeepEqual(pool.Status, expectedPool.Status) {
				t.Errorf("test %s failed\n expected: %+v\n      got: %+v", c, expectedPool, pool)
			}
		}
	}
}
