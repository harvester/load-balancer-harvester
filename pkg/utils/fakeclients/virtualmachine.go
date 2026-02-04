package fakeclients

import (
	"context"

	"github.com/rancher/wrangler/v3/pkg/generic"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	kubevirtv1api "kubevirt.io/api/core/v1"

	kubevirtv1 "github.com/harvester/harvester-load-balancer/pkg/generated/clientset/versioned/typed/kubevirt.io/v1"
)

type VirtualMachineClient func(string) kubevirtv1.VirtualMachineInterface

func (c VirtualMachineClient) Update(vm *kubevirtv1api.VirtualMachine) (*kubevirtv1api.VirtualMachine, error) {
	return c(vm.Namespace).Update(context.TODO(), vm, metav1.UpdateOptions{})
}

func (c VirtualMachineClient) Get(namespace, name string, options metav1.GetOptions) (*kubevirtv1api.VirtualMachine, error) {
	return c(namespace).Get(context.TODO(), name, options)
}

func (c VirtualMachineClient) Create(vm *kubevirtv1api.VirtualMachine) (*kubevirtv1api.VirtualMachine, error) {
	return c(vm.Namespace).Create(context.TODO(), vm, metav1.CreateOptions{})
}

func (c VirtualMachineClient) Delete(_, _ string, _ *metav1.DeleteOptions) error {
	panic("implement me")
}

func (c VirtualMachineClient) List(_ string, _ metav1.ListOptions) (*kubevirtv1api.VirtualMachineList, error) {
	panic("implement me")
}

func (c VirtualMachineClient) UpdateStatus(*kubevirtv1api.VirtualMachine) (*kubevirtv1api.VirtualMachine, error) {
	panic("implement me")
}

func (c VirtualMachineClient) Watch(_ string, _ metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

func (c VirtualMachineClient) Patch(_, _ string, _ types.PatchType, _ []byte, _ ...string) (result *kubevirtv1api.VirtualMachine, err error) {
	panic("implement me")
}

func (c VirtualMachineClient) WithImpersonation(_ rest.ImpersonationConfig) (generic.ClientInterface[*kubevirtv1api.VirtualMachine, *kubevirtv1api.VirtualMachineList], error) {
	panic("implement me")
}

type VirtualMachineCache func(string) kubevirtv1.VirtualMachineInterface

func (c VirtualMachineCache) Get(namespace, name string) (*kubevirtv1api.VirtualMachine, error) {
	return c(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

func (c VirtualMachineCache) List(namespace string, selector labels.Selector) ([]*kubevirtv1api.VirtualMachine, error) {
	list, err := c(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, err
	}
	result := make([]*kubevirtv1api.VirtualMachine, 0, len(list.Items))
	for i := range list.Items {
		result = append(result, &list.Items[i])
	}
	return result, err
}

func (c VirtualMachineCache) AddIndexer(_ string, _ generic.Indexer[*kubevirtv1api.VirtualMachine]) {
	panic("implement me")
}

func (c VirtualMachineCache) GetByIndex(_, _ string) ([]*kubevirtv1api.VirtualMachine, error) {
	panic("implement me")
}
