package servicelb

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubevirtv1 "kubevirt.io/api/core/v1"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
	"github.com/harvester/harvester-load-balancer/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/harvester-load-balancer/pkg/utils/fakeclients"
)

const (
	testNamespace         = "default"
	testNamespaceMismatch = "mismatch"
	testVMName            = "vm1"
	testLBName            = "lb1"
)

func getTestLB() *lbv1.LoadBalancer {
	return &lbv1.LoadBalancer{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      testVMName,
		},
		Spec: lbv1.LoadBalancerSpec{
			BackendServerSelector: map[string][]string{
				"app": {"test"},
			},
		},
	}
}

func getTestVM(namespace string, interfaces []kubevirtv1.VirtualMachineInstanceNetworkInterface, deletionTimeStamp bool) *kubevirtv1.VirtualMachineInstance {
	vmi := &kubevirtv1.VirtualMachineInstance{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      testLBName,
			Labels: map[string]string{
				"app": "test",
			},
		},
		Spec: kubevirtv1.VirtualMachineInstanceSpec{
			Domain: kubevirtv1.DomainSpec{
				Devices: kubevirtv1.Devices{
					Interfaces: []kubevirtv1.Interface{
						{
							Name: "default",
						},
					},
				},
			},
		},
		Status: kubevirtv1.VirtualMachineInstanceStatus{
			Interfaces: interfaces,
		},
	}
	if deletionTimeStamp {
		vmi.DeletionTimestamp = &metav1.Time{}
	}
	return vmi
}

func TestGetBackendServer(t *testing.T) {

	tests := []struct {
		name                             string
		lb                               *lbv1.LoadBalancer
		vmi                              *kubevirtv1.VirtualMachineInstance
		matchedRunningBackendServerCount int
		withAddressBackendServerCount    int
	}{
		{
			name: "return 1 valid server with valid IPv4",
			lb:   getTestLB(),
			vmi: getTestVM(testNamespace, []kubevirtv1.VirtualMachineInstanceNetworkInterface{
				{
					Name: "eth0",
					IP:   "192.168.100.10", // expect VMI has valid IPv4
				},
			}, false),
			matchedRunningBackendServerCount: 1,
			withAddressBackendServerCount:    1,
		},
		{
			name: "match 0 VM, as LB has no selector",
			lb:   &lbv1.LoadBalancer{}, // empty LB
			vmi: getTestVM(testNamespace, []kubevirtv1.VirtualMachineInstanceNetworkInterface{
				{
					Name: "eth0",
					IP:   "192.168.100.10",
				},
			}, false),
			matchedRunningBackendServerCount: 0,
			withAddressBackendServerCount:    0,
		},
		{
			name: "match 0 VM, as the VM has deletionTimeStamp set",
			lb:   getTestLB(),
			vmi: getTestVM(testNamespace, []kubevirtv1.VirtualMachineInstanceNetworkInterface{
				{
					Name: "eth0",
					IP:   "192.168.100.10",
				},
			}, true),
			matchedRunningBackendServerCount: 0,
			withAddressBackendServerCount:    0,
		},
		{
			name: "match 0 VM, as LB and VM are from different namespace",
			lb:   getTestLB(),
			vmi: getTestVM(testNamespaceMismatch, []kubevirtv1.VirtualMachineInstanceNetworkInterface{
				{
					Name: "eth0",
					IP:   "192.168.100.10",
				},
			}, false),
			matchedRunningBackendServerCount: 0,
			withAddressBackendServerCount:    0,
		},

		{
			name: "match 1 VM, valid 0 VM, as it has no IPv4",
			lb:   getTestLB(),
			vmi: getTestVM(testNamespace, []kubevirtv1.VirtualMachineInstanceNetworkInterface{
				{
					Name: "eth0",
				},
			}, false),
			matchedRunningBackendServerCount: 1,
			withAddressBackendServerCount:    0,
		},
		{
			name: "match 1 VM, valid 0 VM, as it has invalid IPv4",
			lb:   getTestLB(),
			vmi: getTestVM(testNamespace, []kubevirtv1.VirtualMachineInstanceNetworkInterface{
				{
					Name: "eth0",
					IP:   "192.168.100.10.200", // invalid IPv4
				},
			}, false),
			matchedRunningBackendServerCount: 1,
			withAddressBackendServerCount:    0,
		},
		{
			name: "match 1 VM, valid 0 VM, as it has invalid IPv4 (but IPv6)",
			lb:   getTestLB(),
			vmi: getTestVM(testNamespace, []kubevirtv1.VirtualMachineInstanceNetworkInterface{
				{
					Name: "eth0",
					IP:   "::1/128",
				},
			}, false),
			matchedRunningBackendServerCount: 1,
			withAddressBackendServerCount:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset()
			lbManager := Manager{
				vmiCache: fakeclients.VirtualMachineInstanceCache(clientset.KubevirtV1().VirtualMachineInstances),
			}
			if tt.vmi != nil {
				err := clientset.Tracker().Add(tt.vmi)
				if err != nil {
					t.Errorf("mock resource should add into fake controller tracker, got error: %v", err.Error())
				}
			}
			ret, err := lbManager.getServiceBackendServers(tt.lb)
			if err != nil {
				t.Errorf("getServiceBackendServers return error: %v", err.Error())
			}
			if ret.GetMatchedBackendServerCount() != tt.matchedRunningBackendServerCount {
				t.Errorf("matchedRunningBackendServerCount, real %v != expected %v", ret.GetMatchedBackendServerCount(), tt.matchedRunningBackendServerCount)
			}
			if ret.GetWithIPAddressBackendServerCount() != tt.withAddressBackendServerCount {
				t.Errorf("withAddressBackendServerCount, real %v != expected %v", ret.GetWithIPAddressBackendServerCount(), tt.withAddressBackendServerCount)
			}
		})
	}
}
