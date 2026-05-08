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

// Test_findProject tests the function findProject
func Test_findProject(t *testing.T) {
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
					WorkloadType: lbv1.VM,
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
					WorkloadType: lbv1.VM,
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
					WorkloadType: lbv1.VM,
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

// Test_findNetwork tests the function findNetwork
func Test_findNetwork(t *testing.T) {
	vmis, err := utils.ParseFromFile(filepath.Join(mutatorCaseDirectory, "vmi.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	clientset := fake.NewSimpleClientset(vmis...)

	m := &mutator{
		vmiCache: fakeclients.VirtualMachineInstanceCache(clientset.KubevirtV1().VirtualMachineInstances),
	}

	tests := []struct {
		name            string
		lb              *lbv1.LoadBalancer
		clusterName     string
		wantNetworkName string
	}{
		{
			name: "Priority 1: Explicit Network exists",
			lb: &lbv1.LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Annotations: map[string]string{
						utils.AnnotationKeyGuestClusterNetworkNameOnLB: "custom/explicit-net",
					},
				},
			},
			clusterName:     "any-cluster",
			wantNetworkName: "custom/explicit-net",
		},
		{
			name: "Priority 2: Fallthrough when Priority 1 is empty string",
			lb: &lbv1.LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Annotations: map[string]string{
						utils.AnnotationKeyGuestClusterNetworkNameOnLB:       "",
						utils.AnnotationKeyGuestClusterManagementNetworkOnLB: "mgmt/mgmt-net",
					},
				},
			},
			clusterName:     "any-cluster",
			wantNetworkName: "mgmt/mgmt-net",
		},
		{
			name: "Step 3: Modern Discovery (Label-based)",
			lb: &lbv1.LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
			},
			clusterName:     "modern-cluster", // Matches guestcluster.harvesterhci.io/name label
			wantNetworkName: "default/modern-net",
		},
		{
			name: "Step 4: Legacy Discovery (Prefix-based fallback)",
			lb: &lbv1.LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
			},
			clusterName:     "rke2-pool1", // Matches name prefix of legacy VMI
			wantNetworkName: "default/mgmt-untagged",
		},
		{
			name: "Namespace Mismatch: Valid clusterName but wrong namespace returns empty",
			lb: &lbv1.LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "wrong-namespace",
				},
			},
			clusterName:     "modern-cluster",
			wantNetworkName: "",
		},
		{
			name: "No Match: Correct namespace but wrong clusterName returns empty",
			lb: &lbv1.LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
			},
			clusterName:     "non-existent-cluster",
			wantNetworkName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			network, err := m.findNetwork(tt.lb, tt.clusterName)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if network != tt.wantNetworkName {
				t.Errorf("findNetwork() got = %q, want %q", network, tt.wantNetworkName)
			}
		})
	}
}
