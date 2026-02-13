package fakeclients

import (
	"context"

	"github.com/rancher/wrangler/v3/pkg/generic"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"

	lbv1beta1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
	lbv1 "github.com/harvester/harvester-load-balancer/pkg/generated/clientset/versioned/typed/loadbalancer.harvesterhci.io/v1beta1"
)

type LoadBalancerCache func(string) lbv1.LoadBalancerInterface

func (l LoadBalancerCache) Get(namespace, name string) (*lbv1beta1.LoadBalancer, error) {
	return l(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

func (l LoadBalancerCache) List(namespace string, selector labels.Selector) ([]*lbv1beta1.LoadBalancer, error) {
	list, err := l(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, err
	}
	result := make([]*lbv1beta1.LoadBalancer, 0, len(list.Items))
	for i := range list.Items {
		result = append(result, &list.Items[i])
	}
	return result, err
}

func (l LoadBalancerCache) AddIndexer(indexName string, indexer generic.Indexer[*lbv1beta1.LoadBalancer]) {
	panic("implement me")
}

func (l LoadBalancerCache) GetByIndex(indexName, key string) ([]*lbv1beta1.LoadBalancer, error) {
	panic("implement me")
}

type LoadBalancerClient func(string) lbv1.LoadBalancerInterface

func (c LoadBalancerClient) Update(lb *lbv1beta1.LoadBalancer) (*lbv1beta1.LoadBalancer, error) {
	return c(lb.Namespace).Update(context.TODO(), lb, metav1.UpdateOptions{})
}

func (c LoadBalancerClient) Get(namespace, name string, options metav1.GetOptions) (*lbv1beta1.LoadBalancer, error) {
	return c(namespace).Get(context.TODO(), name, options)
}

func (c LoadBalancerClient) Create(lb *lbv1beta1.LoadBalancer) (*lbv1beta1.LoadBalancer, error) {
	return c(lb.Namespace).Create(context.TODO(), lb, metav1.CreateOptions{})
}

func (c LoadBalancerClient) Delete(namespace, name string, _ *metav1.DeleteOptions) error {
	return c(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
}

func (c LoadBalancerClient) List(_ string, _ metav1.ListOptions) (*lbv1beta1.LoadBalancerList, error) {
	panic("implement me")
}

func (c LoadBalancerClient) Patch(_, _ string, _ types.PatchType, _ []byte, _ ...string) (*lbv1beta1.LoadBalancer, error) {
	panic("implement me")
}

func (c LoadBalancerClient) UpdateStatus(*lbv1beta1.LoadBalancer) (*lbv1beta1.LoadBalancer, error) {
	panic("implement me")
}

func (c LoadBalancerClient) Watch(_ string, _ metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

func (c LoadBalancerClient) WithImpersonation(_ rest.ImpersonationConfig) (generic.ClientInterface[*lbv1beta1.LoadBalancer, *lbv1beta1.LoadBalancerList], error) {
	panic("implement me")
}
