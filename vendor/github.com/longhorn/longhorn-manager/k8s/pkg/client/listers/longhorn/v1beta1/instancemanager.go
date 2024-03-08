/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by lister-gen. DO NOT EDIT.

package v1beta1

import (
	v1beta1 "github.com/longhorn/longhorn-manager/k8s/pkg/apis/longhorn/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// InstanceManagerLister helps list InstanceManagers.
type InstanceManagerLister interface {
	// List lists all InstanceManagers in the indexer.
	List(selector labels.Selector) (ret []*v1beta1.InstanceManager, err error)
	// InstanceManagers returns an object that can list and get InstanceManagers.
	InstanceManagers(namespace string) InstanceManagerNamespaceLister
	InstanceManagerListerExpansion
}

// instanceManagerLister implements the InstanceManagerLister interface.
type instanceManagerLister struct {
	indexer cache.Indexer
}

// NewInstanceManagerLister returns a new InstanceManagerLister.
func NewInstanceManagerLister(indexer cache.Indexer) InstanceManagerLister {
	return &instanceManagerLister{indexer: indexer}
}

// List lists all InstanceManagers in the indexer.
func (s *instanceManagerLister) List(selector labels.Selector) (ret []*v1beta1.InstanceManager, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.InstanceManager))
	})
	return ret, err
}

// InstanceManagers returns an object that can list and get InstanceManagers.
func (s *instanceManagerLister) InstanceManagers(namespace string) InstanceManagerNamespaceLister {
	return instanceManagerNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// InstanceManagerNamespaceLister helps list and get InstanceManagers.
type InstanceManagerNamespaceLister interface {
	// List lists all InstanceManagers in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*v1beta1.InstanceManager, err error)
	// Get retrieves the InstanceManager from the indexer for a given namespace and name.
	Get(name string) (*v1beta1.InstanceManager, error)
	InstanceManagerNamespaceListerExpansion
}

// instanceManagerNamespaceLister implements the InstanceManagerNamespaceLister
// interface.
type instanceManagerNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all InstanceManagers in the indexer for a given namespace.
func (s instanceManagerNamespaceLister) List(selector labels.Selector) (ret []*v1beta1.InstanceManager, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.InstanceManager))
	})
	return ret, err
}

// Get retrieves the InstanceManager from the indexer for a given namespace and name.
func (s instanceManagerNamespaceLister) Get(name string) (*v1beta1.InstanceManager, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1beta1.Resource("instancemanager"), name)
	}
	return obj.(*v1beta1.InstanceManager), nil
}
