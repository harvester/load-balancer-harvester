package vm

import (
	"strings"
	"testing"

	lbv1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	kubevirtv1 "kubevirt.io/api/core/v1"

	"github.com/harvester/harvester-load-balancer/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/harvester-load-balancer/pkg/utils"
	"github.com/harvester/harvester-load-balancer/pkg/utils/fakeclients"
)

const (
	testNameSpace        = "test"
	testName             = "test"
	testGuestClusterName = "test"
)

func Test_ClearLBWhenGuestClusterIsOnRemove(t *testing.T) {
	tests := []struct {
		name       string
		lb         *lbv1.LoadBalancer
		vm         *kubevirtv1.VirtualMachine
		wantErr    bool
		errorKey   string
		finalLbCnt int
	}{
		{
			name: "vm is not guest cluster related, no action",
			lb: &lbv1.LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNameSpace,
					Name:      testName,
				},
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: lbv1.Cluster,
				},
			},
			vm: &kubevirtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNameSpace,
					Name:      testName,
				},
			},
			wantErr:    false,
			errorKey:   "",
			finalLbCnt: 1,
		},
		{
			name: "vm is guest cluster related, but not on remove, no action",
			lb: &lbv1.LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNameSpace,
					Name:      testName,
					Labels: map[string]string{
						utils.LabelKeyGuestClusterNameOnLB: testGuestClusterName,
					},
				},
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: lbv1.Cluster,
				},
			},
			vm: &kubevirtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNameSpace,
					Name:      testName,
					Labels: map[string]string{
						utils.LabelKeyHarvesterCreator:     utils.GuestClusterHarvesterNodeDriver,
						utils.LabelKeyGuestClusterNameOnVM: testGuestClusterName,
					},
				},
			},
			wantErr:    false,
			errorKey:   "",
			finalLbCnt: 1,
		},
		{
			name: "vm is guest cluster related, is on remove, the guest lb is removed in turn",
			lb: &lbv1.LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNameSpace,
					Name:      testName,
					Labels: map[string]string{
						utils.LabelKeyGuestClusterNameOnLB: testGuestClusterName,
					},
				},
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: lbv1.Cluster,
				},
			},
			vm: &kubevirtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNameSpace,
					Name:      testName,
					Labels: map[string]string{
						utils.LabelKeyHarvesterCreator:     utils.GuestClusterHarvesterNodeDriver,
						utils.LabelKeyGuestClusterNameOnVM: testGuestClusterName,
					},
					Annotations: map[string]string{
						utils.AnnotationKeyGuestClusterOnRemove: "true",
					},
				},
			},
			wantErr:    false,
			errorKey:   "",
			finalLbCnt: 0,
		},
		{
			name: "vm is guest cluster related, is on remove, the lb of another guest cluster on same namespace is NOT removed",
			lb: &lbv1.LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNameSpace,
					Name:      testName,
					Labels: map[string]string{
						utils.LabelKeyGuestClusterNameOnLB: "not-related",
					},
				},
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: lbv1.Cluster,
				},
			},
			vm: &kubevirtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNameSpace,
					Name:      testName,
					Labels: map[string]string{
						utils.LabelKeyHarvesterCreator:     utils.GuestClusterHarvesterNodeDriver,
						utils.LabelKeyGuestClusterNameOnVM: testGuestClusterName,
					},
					Annotations: map[string]string{
						utils.AnnotationKeyGuestClusterOnRemove: "true",
					},
				},
			},
			wantErr:    false,
			errorKey:   "",
			finalLbCnt: 1,
		},
		{
			name: "vm is guest cluster related, is on remove, the lb type of non-Cluster is NOT removed",
			lb: &lbv1.LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNameSpace,
					Name:      testName,
				},
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: lbv1.VM,
				},
			},
			vm: &kubevirtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNameSpace,
					Name:      testName,
					Labels: map[string]string{
						utils.LabelKeyHarvesterCreator:     utils.GuestClusterHarvesterNodeDriver,
						utils.LabelKeyGuestClusterNameOnVM: testGuestClusterName,
					},
					Annotations: map[string]string{
						utils.AnnotationKeyGuestClusterOnRemove: "true",
					},
				},
			},
			wantErr:    false,
			errorKey:   "",
			finalLbCnt: 1,
		},
		{
			name: "vm is guest cluster related, is on remove, but guest cluster name is missing, a WARN message is shown",
			lb: &lbv1.LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNameSpace,
					Name:      testName,
				},
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: lbv1.VM,
				},
			},
			vm: &kubevirtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNameSpace,
					Name:      testName,
					Labels: map[string]string{
						utils.LabelKeyHarvesterCreator: utils.GuestClusterHarvesterNodeDriver,
					},
					Annotations: map[string]string{
						utils.AnnotationKeyGuestClusterOnRemove: "true",
					},
				},
			},
			wantErr:    false,
			errorKey:   "",
			finalLbCnt: 1,
		},
	}

	for _, tt := range tests {
		objs := []runtime.Object{tt.vm, tt.lb}
		clientset := fake.NewSimpleClientset(objs...)

		lbClient := fakeclients.LoadBalancerClient(clientset.LoadbalancerV1beta1().LoadBalancers)
		lbCache := fakeclients.LoadBalancerCache(clientset.LoadbalancerV1beta1().LoadBalancers)
		h := &Handler{
			lbClient: lbClient,
			lbCache:  lbCache,
		}

		_, err := h.CleanGuestClusterLBs(tt.vm.Name, tt.vm)
		if (err != nil) != tt.wantErr {
			t.Errorf("%q. CleanGuestClusterLBs() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
		if tt.wantErr && tt.errorKey != "" && !strings.Contains(err.Error(), tt.errorKey) {
			t.Errorf("%q, the return error %v does not include the keyword '%s'", tt.name, err, tt.errorKey)
		}
		lbs, err := lbCache.List(tt.vm.Namespace, labels.Everything())
		if err != nil {
			t.Errorf("%q fail to list load balancers, error: %v", tt.name, err.Error())
		}
		if len(lbs) != tt.finalLbCnt {
			t.Errorf("%q expect final lbs %v but got %v", tt.name, tt.finalLbCnt, len(lbs))
		}
	}
}
