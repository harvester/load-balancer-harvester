package ipam

import (
	"path/filepath"
	"testing"

	"github.com/harvester/harvester-load-balancer/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/harvester-load-balancer/pkg/utils"
	"github.com/harvester/harvester-load-balancer/pkg/utils/fakeclients"
)

const rootDir = "./testdata/selector/"

func TestSelector_Select(t *testing.T) {
	cases, err := utils.GetSubdirectories(rootDir)
	if err != nil {
		t.Error(err)
	}

	testcases := []struct {
		Requirement  *Requirement
		ExpectedPool string
		wantErr      bool
	}{
		{
			Requirement: &Requirement{
				Namespace: "default",
			},
			ExpectedPool: "default",
		},
		{
			Requirement: &Requirement{
				Namespace: "test",
			},
			ExpectedPool: "global",
		},
		{
			Requirement: &Requirement{
				Network:   "default/vlan10",
				Project:   "project1",
				Namespace: "default",
				Cluster:   "cluster1",
			},
			ExpectedPool: "default-vlan10",
		},
		{
			Requirement: &Requirement{
				Namespace: "default",
			},
			ExpectedPool: "default-priority100",
		},
	}

	for i, c := range cases {
		t.Logf("test %s", c)

		objs, err := utils.ParseFromFile(filepath.Join(rootDir, c, "ippool.yaml"))
		if err != nil {
			t.Errorf("test %s failed, error: %v", c, err)
		}
		lbClientset := fake.NewSimpleClientset(objs...)
		selector := NewSelector(fakeclients.IPPoolCache(lbClientset.LoadbalancerV1beta1().IPPools))
		pool, err := selector.Select(testcases[i].Requirement)
		if err != nil {
			t.Errorf("test %s failed, error: %v", c, err)
		}
		if pool.Name != testcases[i].ExpectedPool {
			t.Errorf("test %s failed, expected: %s, got: %s", c, testcases[i].ExpectedPool, pool.Name)
		}
	}
}
