package fakeclients

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	lbv1beta1 "github.com/harvester/harvester-load-balancer/pkg/apis/loadbalancer.harvesterhci.io/v1beta1"
	lbv1 "github.com/harvester/harvester-load-balancer/pkg/generated/clientset/versioned/typed/loadbalancer.harvesterhci.io/v1beta1"
	ctllbv1beta1 "github.com/harvester/harvester-load-balancer/pkg/generated/controllers/loadbalancer.harvesterhci.io/v1beta1"
)

type IPPoolCache func() lbv1.IPPoolInterface

func (i IPPoolCache) Get(name string) (*lbv1beta1.IPPool, error) {
	return i().Get(context.TODO(), name, metav1.GetOptions{})
}

func (i IPPoolCache) List(selector labels.Selector) ([]*lbv1beta1.IPPool, error) {
	list, err := i().List(context.TODO(), metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, err
	}
	result := make([]*lbv1beta1.IPPool, 0, len(list.Items))
	for i := range list.Items {
		result = append(result, &list.Items[i])
	}
	return result, err
}

func (i IPPoolCache) AddIndexer(indexName string, indexer ctllbv1beta1.IPPoolIndexer) {
	panic("implement me")
}

func (i IPPoolCache) GetByIndex(indexName, key string) ([]*lbv1beta1.IPPool, error) {
	panic("implement me")
}
