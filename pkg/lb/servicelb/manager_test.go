package servicelb

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
	"github.com/harvester/harvester-load-balancer/pkg/generated/clientset/versioned/fake"
	discoveryv1type "github.com/harvester/harvester-load-balancer/pkg/generated/clientset/versioned/typed/discovery.k8s.io/v1"
	"github.com/harvester/harvester-load-balancer/pkg/utils/fakeclients"
	kubevirtv1 "kubevirt.io/api/core/v1"
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

func TestConstructDummyEndpointSlice(t *testing.T) {
	portName := "http"
	proto := corev1.ProtocolTCP
	var backendPort int32 = 8080

	lb := &lbv1.LoadBalancer{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "loadbalancer.harvesterhci.io/v1beta1",
			Kind:       "LoadBalancer",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-lb",
			Namespace: "default",
			UID:       "uid-1234",
		},
		Spec: lbv1.LoadBalancerSpec{
			Listeners: []lbv1.Listener{
				{
					Name:        portName,
					Protocol:    proto,
					BackendPort: backendPort,
				},
			},
		},
	}

	// lb controller name is unknown
	m := &Manager{}

	eps := m.constructDummyEndpointSlice(lb)

	if eps.Namespace != "default" {
		t.Errorf("expected namespace 'default', got '%s'", eps.Namespace)
	}
	if eps.Name != "test-lb" {
		t.Errorf("expected name 'test-lb', got '%s'", eps.Name)
	}
	if eps.AddressType != discoveryv1.AddressTypeIPv4 {
		t.Errorf("expected addressType IPv4, got '%s'", eps.AddressType)
	}

	// Verify OwnerReference
	if len(eps.OwnerReferences) != 1 {
		t.Fatalf("expected 1 OwnerReference, got %d", len(eps.OwnerReferences))
	}
	if eps.OwnerReferences[0].Name != lb.Name {
		t.Errorf("expected OwnerReference name '%s', got '%s'", lb.Name, eps.OwnerReferences[0].Name)
	}
	if eps.OwnerReferences[0].UID != lb.UID {
		t.Errorf("expected OwnerReference UID '%s', got '%s'", lb.UID, eps.OwnerReferences[0].UID)
	}

	// Verify Labels
	if svcName := eps.Labels[KeyServiceName]; svcName != lb.Name {
		t.Errorf("expected service name label '%s', got '%s'", lb.Name, svcName)
	}

	// Verify Ports
	if len(eps.Ports) != 1 {
		t.Fatalf("expected 1 port, got %d", len(eps.Ports))
	}
	if eps.Ports[0].Name == nil || *eps.Ports[0].Name != portName {
		t.Errorf("expected port name '%s', got '%v'", portName, eps.Ports[0].Name)
	}
	if eps.Ports[0].Protocol == nil || *eps.Ports[0].Protocol != proto {
		t.Errorf("expected protocol '%s', got '%v'", proto, eps.Ports[0].Protocol)
	}
	if eps.Ports[0].Port == nil || *eps.Ports[0].Port != backendPort {
		t.Errorf("expected port '%d', got '%v'", backendPort, eps.Ports[0].Port)
	}
}

func TestEnsureDummyEndpointsliceIfNotExist(t *testing.T) {
	lb := &lbv1.LoadBalancer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-lb",
			Namespace: "default",
		},
	}

	existingEPS := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-lb",
			Namespace: "default",
		},
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"test"},
			},
		},
	}

	existingEmptyEPS := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-lb",
			Namespace: "default",
		},
	}

	tests := []struct {
		name          string
		existingObjs  []runtime.Object
		expectCreated bool
		expectUpdated bool
		expectErr     bool
	}{
		{
			name:          "EndpointSlice already exists in cache",
			existingObjs:  []runtime.Object{existingEPS},
			expectCreated: false,
			expectErr:     false,
		},
		{
			name:          "EndpointSlice does not exist, create successfully",
			existingObjs:  []runtime.Object{},
			expectCreated: true,
			expectErr:     false,
		},
		{
			name:          "EndpointSlice exists but with empty endpoints, update successfully",
			existingObjs:  []runtime.Object{existingEmptyEPS},
			expectUpdated: true,
			expectErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewSimpleClientset(tt.existingObjs...)

			epsClient := fakeclients.EndpointSliceClient(func(namespace string) discoveryv1type.EndpointSliceInterface {
				return fakeClient.DiscoveryV1().EndpointSlices(namespace)
			})

			epsCache := fakeclients.EndpointSliceCache(func(namespace string) discoveryv1type.EndpointSliceInterface {
				return fakeClient.DiscoveryV1().EndpointSlices(namespace)
			})

			m := &Manager{
				endpointSliceClient: epsClient,
				endpointSliceCache:  epsCache,
			}

			err := m.ensureDummyEndpointSliceIfNotExist(lb)

			if tt.expectErr && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Filter actions for 'create', 'update' verb specifically
			var createActions []clientgotesting.Action
			var updateActions []clientgotesting.Action
			for _, action := range fakeClient.Actions() {
				if action.GetVerb() == "create" {
					createActions = append(createActions, action)
				} else if action.GetVerb() == "update" {
					updateActions = append(updateActions, action)
				}
			}

			if tt.expectCreated && len(createActions) != 1 {
				t.Fatalf("expected 1 'create' API action, got %d", len(createActions))
			}
			if tt.expectUpdated && len(updateActions) != 1 {
				t.Fatalf("expected 1 'update' API action, got %d", len(updateActions))
			}
		})
	}
}
