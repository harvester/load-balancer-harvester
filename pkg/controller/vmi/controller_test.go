package vmi

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
	testNameSpace = "test"
	testName      = "test"
)

func Test_ClearLBWhenGuestClusterIsOnRemove(t *testing.T) {
	tests := []struct {
		name       string
		lb         *lbv1.LoadBalancer
		vmi        *kubevirtv1.VirtualMachineInstance
		wantErr    bool
		errorKey   string
		finalLbCnt int
	}{
		{
			name: "vmi is not guest cluster related, no action",
			lb: &lbv1.LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNameSpace,
					Name:      testName,
				},
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: lbv1.Cluster,
				},
			},
			vmi: &kubevirtv1.VirtualMachineInstance{
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
			name: "vmi is guest cluster related, but not on remove, no action",
			lb: &lbv1.LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNameSpace,
					Name:      testName,
				},
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: lbv1.Cluster,
				},
			},
			vmi: &kubevirtv1.VirtualMachineInstance{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNameSpace,
					Name:      testName,
					Labels: map[string]string{
						utils.LabelKeyHarvesterCreator: utils.GuestClusterHarvesterNodeDriver,
					},
				},
			},
			wantErr:    false,
			errorKey:   "",
			finalLbCnt: 1,
		},
		{
			name: "vmi is guest cluster related, is on remove, the guest lb is removed in turn",
			lb: &lbv1.LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNameSpace,
					Name:      testName,
				},
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: lbv1.Cluster,
				},
			},
			vmi: &kubevirtv1.VirtualMachineInstance{
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
			finalLbCnt: 0,
		},
		{
			name: "vmi is guest cluster related, is on remove, the VM type lb is NOT removed",
			lb: &lbv1.LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNameSpace,
					Name:      testName,
				},
				Spec: lbv1.LoadBalancerSpec{
					WorkloadType: lbv1.VM,
				},
			},
			vmi: &kubevirtv1.VirtualMachineInstance{
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
		objs := []runtime.Object{tt.vmi, tt.lb}
		clientset := fake.NewSimpleClientset(objs...)

		lbClient := fakeclients.LoadBalancerClient(clientset.LoadbalancerV1beta1().LoadBalancers)
		lbCache := fakeclients.LoadBalancerCache(clientset.LoadbalancerV1beta1().LoadBalancers)
		h := &Handler{
			lbController: nil,
			lbClient:     lbClient,
			lbCache:      lbCache,
		}

		_, err := h.CleanGuestClusterLBs(tt.vmi.Name, tt.vmi)
		if (err != nil) != tt.wantErr {
			t.Errorf("%q. CleanGuestClusterLBs() error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
		if tt.wantErr && tt.errorKey != "" && !strings.Contains(err.Error(), tt.errorKey) {
			t.Errorf("%q, the return error %v does not include the keyword '%s'", tt.name, err, tt.errorKey)
		}
		lbs, err := lbCache.List(tt.vmi.Namespace, labels.Everything())
		if err != nil {
			t.Errorf("fail to list load balancers, error: %v", err.Error())
		}
		if len(lbs) != tt.finalLbCnt {
			t.Errorf("expect final lbs %v but got %v", tt.finalLbCnt, len(lbs))
		}
	}
}
