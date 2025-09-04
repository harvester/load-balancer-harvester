package ipam

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"

	"github.com/harvester/harvester-load-balancer/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/harvester-load-balancer/pkg/utils/fakeclients"
)

const (
	rootDir = "./testdata/"

	defaultNamespace = "default"

	defaultPoolName         = "default"
	defaultPriorityPoolName = "defaultPriority"
	globalPoolName          = "global"
	emptyPoolName           = ""

	globalIpPoolKey = "loadbalancer.harvesterhci.io/global-ip-pool"
	vidKey          = "loadbalancer.harvesterhci.io/vid"

	vlan10Network = "default/vlan10"

	vlan100NoScope   = "vlan-100-no-scope"
	vlan100WithScope = "vlan-100-with-scope"

	cluster1 = "cluster1"
	project1 = "project1"
)

var (
	defaultPool = &lbv1.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: defaultPoolName,
			Labels: map[string]string{
				globalIpPoolKey: "false",
				vidKey:          "0",
			},
		},
		Spec: lbv1.IPPoolSpec{
			Ranges: []lbv1.Range{
				{
					Subnet: "172.19.110.192/27",
				},
			},
			Selector: lbv1.Selector{
				Network: vlan10Network,
				Scope: []lbv1.Tuple{
					{
						GuestCluster: All,
						Namespace:    defaultNamespace,
						Project:      All,
					},
				},
			},
		},
	}

	defaultPriorityPool = &lbv1.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: defaultPriorityPoolName,
			Labels: map[string]string{
				globalIpPoolKey: "false",
				vidKey:          "0",
			},
		},
		Spec: lbv1.IPPoolSpec{
			Ranges: []lbv1.Range{
				{
					Subnet: "172.19.110.192/27",
				},
			},
			Selector: lbv1.Selector{
				Priority: 100, // the higher, the better
				Network:  vlan10Network,
				Scope: []lbv1.Tuple{
					{
						GuestCluster: All,
						Namespace:    defaultNamespace,
						Project:      All,
					},
				},
			},
		},
	}

	globalPool = &lbv1.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: globalPoolName,
			Labels: map[string]string{
				globalIpPoolKey: "true",
				vidKey:          "0",
			},
		},
		Spec: lbv1.IPPoolSpec{
			Ranges: []lbv1.Range{
				{
					Subnet: "172.19.111.192/27",
				},
			},
			Selector: lbv1.Selector{
				Scope: []lbv1.Tuple{
					{
						GuestCluster: All,
						Namespace:    All,
						Project:      All,
					},
				},
			},
		},
	}

	vlan100NoScopePool = &lbv1.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: vlan100NoScope,
			Labels: map[string]string{
				globalIpPoolKey: "false",
				vidKey:          "0",
			},
		},
		Spec: lbv1.IPPoolSpec{
			Ranges: []lbv1.Range{
				{
					Subnet: "172.19.111.10/27",
				},
			},
			Selector: lbv1.Selector{
				Network: vlan10Network,
				Scope:   []lbv1.Tuple{ // scope is empty
				},
			},
		},
	}

	vlan100WithScopePool = &lbv1.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: vlan100WithScope,
			Labels: map[string]string{
				globalIpPoolKey: "false",
				vidKey:          "0",
			},
		},
		Spec: lbv1.IPPoolSpec{
			Ranges: []lbv1.Range{
				{
					Subnet: "172.19.111.10/27",
				},
			},
			Selector: lbv1.Selector{
				Network: vlan10Network,
				Scope: []lbv1.Tuple{
					{
						GuestCluster: cluster1,
						Namespace:    defaultNamespace,
						Project:      project1,
					},
				},
			},
		},
	}

	testFunc = func(t *testing.T, lbClientset *fake.Clientset, testcases []testCase) {
		for _, tc := range testcases {
			t.Logf("test %s", tc.name)
			selector := NewSelector(fakeclients.IPPoolCache(lbClientset.LoadbalancerV1beta1().IPPools))
			pool, err := selector.Select(tc.Requirement)
			if tc.wantErr != (err != nil) {
				t.Errorf("test %s failed, return error does not equal wantErr: %v", tc.name, tc.wantErr)
			}
			if tc.wantErr {
				continue
			}
			if tc.ExpectedPool == emptyPoolName {
				if pool != nil {
					t.Errorf("test %s failed, expected: %s, got: %s", tc.name, tc.ExpectedPool, pool.Name)
				}
				continue
			}
			if pool == nil {
				t.Errorf("test %s failed, expected: %s, but get empty pool", tc.name, tc.ExpectedPool)
				continue
			}
			if pool.Name != tc.ExpectedPool {
				t.Errorf("test %s failed, expected: %s, got: %s", tc.name, tc.ExpectedPool, pool.Name)
			}
		}
	}
)

type testCase struct {
	name         string
	Requirement  *Requirement
	ExpectedPool string
	wantErr      bool
	nilPool      bool
}

func TestSelectorWithGlobalPool_Select(t *testing.T) {
	testcases := []testCase{
		{
			name: "get default pool when namespace match",
			Requirement: &Requirement{
				Network:   vlan10Network,
				Namespace: defaultNamespace,
			},
			ExpectedPool: defaultPoolName,
		},
		{
			name: "namespace does not match specific but still get global",
			Requirement: &Requirement{
				Network:   vlan10Network,
				Namespace: "test",
			},
			ExpectedPool: globalPoolName,
		},
		{
			name: "can't match a guest cluster related pool but still get global",
			Requirement: &Requirement{
				Network:   vlan10Network,
				Project:   "unknow",
				Namespace: "", // guest cluster does not set it
				Cluster:   "cluster1",
			},
			ExpectedPool: globalPoolName,
			wantErr:      false,
		},
	}

	lbClientset := fake.NewSimpleClientset(defaultPool, globalPool)

	testFunc(t, lbClientset, testcases)
}

func TestSelectorWithoutGlobalPool_Select(t *testing.T) {
	testcases := []testCase{
		{
			name: "get default pool when namespace match",
			Requirement: &Requirement{
				Network:   vlan10Network,
				Namespace: defaultNamespace,
			},
			ExpectedPool: defaultPoolName,
		},
		{
			name: "doest not get pool when namespace does not match specific",
			Requirement: &Requirement{
				Network:   vlan10Network,
				Namespace: "test",
			},
			ExpectedPool: emptyPoolName,
			wantErr:      false,
		},
		{
			name: "doest not get pool when can't match a guest cluster related pool",
			Requirement: &Requirement{
				Network:   vlan10Network,
				Project:   "unknow",
				Namespace: "", // guest cluster does not set it
				Cluster:   "cluster1",
			},
			ExpectedPool: emptyPoolName,
			wantErr:      false,
		},
	}

	lbClientset := fake.NewSimpleClientset(defaultPool)

	testFunc(t, lbClientset, testcases)
}

func TestSelectorWitPriorityWithoutGlobalPool_Select(t *testing.T) {
	testcases := []testCase{
		{
			name: "get default priority pool when namespace match",
			Requirement: &Requirement{
				Network:   vlan10Network,
				Namespace: defaultNamespace,
			},
			ExpectedPool: defaultPriorityPoolName,
		},
	}

	lbClientset := fake.NewSimpleClientset(defaultPool, defaultPriorityPool)

	testFunc(t, lbClientset, testcases)
}

func TestSelectorGuestClusterCaseExactMatchWithoutGlobalPool_Select(t *testing.T) {
	testcases := []testCase{
		{
			name: "exact match a pool for guest cluster",
			Requirement: &Requirement{
				Network:   vlan10Network,
				Project:   project1,
				Namespace: defaultNamespace,
				Cluster:   cluster1,
			},
			ExpectedPool: vlan100WithScope,
		},
		{
			name: "can't match a guest cluster related pool",
			Requirement: &Requirement{
				Network:   vlan10Network,
				Project:   "unknow",
				Namespace: defaultNamespace,
				Cluster:   cluster1,
			},
			ExpectedPool: "",
			wantErr:      false,
		},
		{
			name: "can't match a guest cluster related pool",
			Requirement: &Requirement{
				Network:   vlan10Network,
				Project:   "", // guest cluster does not set it
				Namespace: "", // guest cluster does not set it
				Cluster:   cluster1,
			},
			ExpectedPool: "",
			wantErr:      false,
		},
	}

	lbClientset := fake.NewSimpleClientset(vlan100WithScopePool)
	testFunc(t, lbClientset, testcases)
}

func TestSelectorGuestClusterCaseLooseMatchWithoutGlobalPool_Select(t *testing.T) {
	testcases := []testCase{
		{
			name: "exact match a pool for guest cluster",
			Requirement: &Requirement{
				Network:   vlan10Network,
				Project:   project1,
				Namespace: defaultNamespace,
				Cluster:   cluster1,
			},
			ExpectedPool: vlan100WithScope,
		},
		{
			name: "can't match a guest cluster related pool, but get the no scope pool",
			Requirement: &Requirement{
				Network:   vlan10Network,
				Project:   "unknow",
				Namespace: defaultNamespace,
				Cluster:   cluster1,
			},
			ExpectedPool: vlan100NoScope,
			wantErr:      false,
		},
		{
			name: "can't match a guest cluster related pool, but get the no scope pool",
			Requirement: &Requirement{
				Network:   vlan10Network,
				Project:   "", // guest cluster does not set it
				Namespace: "", // guest cluster does not set it
				Cluster:   cluster1,
			},
			ExpectedPool: vlan100NoScope,
			wantErr:      false,
		},
	}

	lbClientset := fake.NewSimpleClientset(vlan100WithScopePool, vlan100NoScopePool)
	testFunc(t, lbClientset, testcases)
}
