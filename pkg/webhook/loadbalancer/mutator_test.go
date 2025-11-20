package loadbalancer

import (
	"path/filepath"
	"testing"

	corefake "k8s.io/client-go/kubernetes/fake"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/harvester/harvester-load-balancer/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/harvester-load-balancer/pkg/utils"
	"github.com/harvester/harvester-load-balancer/pkg/utils/fakeclients"
)

const mutatorCaseDirectory = "./testdata/mutator/"

// TestFindProject tests the function findProject
func TestFindProject(t *testing.T) {
	namespaces, err := utils.ParseFromFile(filepath.Join(mutatorCaseDirectory + "namespace.yaml"))
	if err != nil {
		t.Error(err)
	}
	// create a new mutator
	coreclientset := corefake.NewSimpleClientset(namespaces...)
	m := &mutator{
		namespaceCache: fakeclients.NamespaceCache(coreclientset.CoreV1().Namespaces),
	}

	tests := []struct {
		namespace   string
		wantProject string
	}{
		{
			namespace:   "default",
			wantProject: "local/p-abcde",
		},
		{
			namespace:   "withoutProject",
			wantProject: "",
		},
	}

	testsHealthCheckMutatored := []struct {
		name    string
		lb      *lbv1.LoadBalancer
		wantErr bool
		opsLen  int
	}{
		{
			name: "health check mutatored case",
			lb: &lbv1.LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "test",
				},
				Spec: lbv1.LoadBalancerSpec{
					Listeners: []lbv1.Listener{
						{Name: "a", BackendPort: 80, Protocol: corev1.ProtocolTCP},
						{Name: "b", BackendPort: 32, Protocol: corev1.ProtocolUDP},
					},
					HealthCheck: &lbv1.HealthCheck{Port: 80, SuccessThreshold: 0, FailureThreshold: 1, PeriodSeconds: 1, TimeoutSeconds: 1},
				},
			},
			wantErr: false,
			opsLen:  2,
		},
		{
			name: "health check right case: valid parameters",
			lb: &lbv1.LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "test",
				},
				Spec: lbv1.LoadBalancerSpec{
					Listeners: []lbv1.Listener{
						{Name: "a", BackendPort: 80, Protocol: corev1.ProtocolTCP},
						{Name: "b", BackendPort: 32, Protocol: corev1.ProtocolUDP},
					},
					HealthCheck: &lbv1.HealthCheck{Port: 80, SuccessThreshold: 1, FailureThreshold: 1, PeriodSeconds: 1, TimeoutSeconds: 1},
				},
			},
			wantErr: false,
			opsLen:  1,
		},
		{
			name: "health check right case: no health check",
			lb: &lbv1.LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "test",
				},
				Spec: lbv1.LoadBalancerSpec{
					Listeners: []lbv1.Listener{
						{Name: "a", BackendPort: 80, Protocol: corev1.ProtocolTCP},
						{Name: "b", BackendPort: 32, Protocol: corev1.ProtocolUDP},
					},
				},
			},
			wantErr: false,
			opsLen:  1,
		},
	}

	for _, test := range tests {
		if project, err := m.findProject(test.namespace); err != nil {
			t.Error(err)
		} else if project != test.wantProject {
			t.Errorf("want project %s through namespace %s, got %s", test.wantProject, test.namespace, project)
		}
	}

	for _, test := range testsHealthCheckMutatored {
		if pt, err := m.Create(nil, test.lb); (err != nil) != test.wantErr {
			t.Error(err)
		} else if len(pt) != test.opsLen {
			// return 2 ops
			// [{Op:replace Path:/metadata/annotations Value:map[loadbalancer.harvesterhci.io/namespace:default loadbalancer.harvesterhci.io/network: loadbalancer.harvesterhci.io/project:local/p-abcde]}
			// {Op:replace Path:/spec/healthCheck Value:{Port:80 SuccessThreshold:2 FailureThreshold:1 PeriodSeconds:1 TimeoutSeconds:1}}]
			t.Errorf("create test %v return patchOps len %v != %v, %+v", test.name, len(pt), test.opsLen, pt)
		}

		if pt, err := m.Update(nil, nil, test.lb); (err != nil) != test.wantErr {
			t.Error(err)
		} else if len(pt) != test.opsLen {
			t.Errorf("update test %v return patchOps len %v != %v, %+v", test.name, len(pt), test.opsLen, pt)
		}
	}
}

// TestFindNetwork tests the function findNetwork
func TestFindNetwork(t *testing.T) {
	vmis, err := utils.ParseFromFile(filepath.Join(mutatorCaseDirectory + "vmi.yaml"))
	if err != nil {
		t.Error(err)
	}

	clientset := fake.NewSimpleClientset(vmis...)

	m := &mutator{
		vmiCache: fakeclients.VirtualMachineInstanceCache(clientset.KubevirtV1().VirtualMachineInstances),
	}

	tests := []struct {
		namespace       string
		clusterName     string
		wantNetworkName string
	}{
		{
			namespace:       "default",
			clusterName:     "rke2",
			wantNetworkName: "default/mgmt-untagged",
		},
	}

	for _, test := range tests {
		if network, err := m.findNetwork(test.namespace, test.clusterName); err != nil {
			t.Error(err)
		} else if network != test.wantNetworkName {
			t.Errorf("want network %s through namespace %s and cluster %s, got %s",
				test.wantNetworkName, test.namespace, test.clusterName, network)
		}
	}
}
