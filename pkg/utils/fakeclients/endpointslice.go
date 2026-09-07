package fakeclients

import (
	"context"

	discoveryv1type "github.com/harvester/harvester-load-balancer/pkg/generated/clientset/versioned/typed/discovery.k8s.io/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	v1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
)

type EndpointSliceClient func(namespace string) discoveryv1type.EndpointSliceInterface

func (c EndpointSliceClient) Create(endpointSlice *v1.EndpointSlice) (*v1.EndpointSlice, error) {
	return c(endpointSlice.Namespace).Create(context.TODO(), endpointSlice, metav1.CreateOptions{})
}

func (c EndpointSliceClient) Update(endpointSlice *v1.EndpointSlice) (*v1.EndpointSlice, error) {
	return c(endpointSlice.Namespace).Update(context.TODO(), endpointSlice, metav1.UpdateOptions{})
}

func (c EndpointSliceClient) UpdateStatus(_ *v1.EndpointSlice) (*v1.EndpointSlice, error) {
	panic("implement me")
}

func (c EndpointSliceClient) Delete(namespace, name string, opts *metav1.DeleteOptions) error {
	deleteOpts := metav1.DeleteOptions{}
	if opts != nil {
		deleteOpts = *opts
	}
	return c(namespace).Delete(context.TODO(), name, deleteOpts)
}

func (c EndpointSliceClient) Get(namespace, name string, opts metav1.GetOptions) (*v1.EndpointSlice, error) {
	return c(namespace).Get(context.TODO(), name, opts)
}

func (c EndpointSliceClient) List(namespace string, opts metav1.ListOptions) (*v1.EndpointSliceList, error) {
	return c(namespace).List(context.TODO(), opts)
}

func (c EndpointSliceClient) Watch(_ string, _ metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

func (c EndpointSliceClient) Patch(namespace, name string, pt types.PatchType, data []byte, _ ...string) (result *v1.EndpointSlice, err error) {
	return c(namespace).Patch(context.TODO(), name, pt, data, metav1.PatchOptions{})
}

func (c EndpointSliceClient) WithImpersonation(_ rest.ImpersonationConfig) (generic.ClientInterface[*v1.EndpointSlice, *v1.EndpointSliceList], error) {
	panic("implement me")
}

type EndpointSliceCache func(namespace string) discoveryv1type.EndpointSliceInterface

func (c EndpointSliceCache) Get(namespace, name string) (*v1.EndpointSlice, error) {
	return c(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

func (c EndpointSliceCache) List(namespace string, selector labels.Selector) ([]*v1.EndpointSlice, error) {
	list, err := c(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, err
	}
	result := make([]*v1.EndpointSlice, 0, len(list.Items))
	for i := range list.Items {
		result = append(result, &list.Items[i])
	}
	return result, nil
}

func (c EndpointSliceCache) AddIndexer(_ string, _ generic.Indexer[*v1.EndpointSlice]) {
	panic("implement me")
}

func (c EndpointSliceCache) GetByIndex(_, _ string) ([]*v1.EndpointSlice, error) {
	panic("implement me")
}
