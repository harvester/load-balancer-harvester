/*
Copyright 2024 Rancher Labs, Inc.

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

// Code generated by main. DO NOT EDIT.

package fake

import (
	"context"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeClusterTemplates implements ClusterTemplateInterface
type FakeClusterTemplates struct {
	Fake *FakeManagementV3
	ns   string
}

var clustertemplatesResource = schema.GroupVersionResource{Group: "management.cattle.io", Version: "v3", Resource: "clustertemplates"}

var clustertemplatesKind = schema.GroupVersionKind{Group: "management.cattle.io", Version: "v3", Kind: "ClusterTemplate"}

// Get takes name of the clusterTemplate, and returns the corresponding clusterTemplate object, and an error if there is any.
func (c *FakeClusterTemplates) Get(ctx context.Context, name string, options v1.GetOptions) (result *v3.ClusterTemplate, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(clustertemplatesResource, c.ns, name), &v3.ClusterTemplate{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v3.ClusterTemplate), err
}

// List takes label and field selectors, and returns the list of ClusterTemplates that match those selectors.
func (c *FakeClusterTemplates) List(ctx context.Context, opts v1.ListOptions) (result *v3.ClusterTemplateList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(clustertemplatesResource, clustertemplatesKind, c.ns, opts), &v3.ClusterTemplateList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v3.ClusterTemplateList{ListMeta: obj.(*v3.ClusterTemplateList).ListMeta}
	for _, item := range obj.(*v3.ClusterTemplateList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusterTemplates.
func (c *FakeClusterTemplates) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(clustertemplatesResource, c.ns, opts))

}

// Create takes the representation of a clusterTemplate and creates it.  Returns the server's representation of the clusterTemplate, and an error, if there is any.
func (c *FakeClusterTemplates) Create(ctx context.Context, clusterTemplate *v3.ClusterTemplate, opts v1.CreateOptions) (result *v3.ClusterTemplate, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(clustertemplatesResource, c.ns, clusterTemplate), &v3.ClusterTemplate{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v3.ClusterTemplate), err
}

// Update takes the representation of a clusterTemplate and updates it. Returns the server's representation of the clusterTemplate, and an error, if there is any.
func (c *FakeClusterTemplates) Update(ctx context.Context, clusterTemplate *v3.ClusterTemplate, opts v1.UpdateOptions) (result *v3.ClusterTemplate, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(clustertemplatesResource, c.ns, clusterTemplate), &v3.ClusterTemplate{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v3.ClusterTemplate), err
}

// Delete takes name of the clusterTemplate and deletes it. Returns an error if one occurs.
func (c *FakeClusterTemplates) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(clustertemplatesResource, c.ns, name, opts), &v3.ClusterTemplate{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeClusterTemplates) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(clustertemplatesResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v3.ClusterTemplateList{})
	return err
}

// Patch applies the patch and returns the patched clusterTemplate.
func (c *FakeClusterTemplates) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v3.ClusterTemplate, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(clustertemplatesResource, c.ns, name, pt, data, subresources...), &v3.ClusterTemplate{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v3.ClusterTemplate), err
}
