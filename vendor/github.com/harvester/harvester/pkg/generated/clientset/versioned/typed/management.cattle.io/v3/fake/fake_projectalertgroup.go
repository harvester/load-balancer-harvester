/*
Copyright 2023 Rancher Labs, Inc.

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

// FakeProjectAlertGroups implements ProjectAlertGroupInterface
type FakeProjectAlertGroups struct {
	Fake *FakeManagementV3
	ns   string
}

var projectalertgroupsResource = schema.GroupVersionResource{Group: "management.cattle.io", Version: "v3", Resource: "projectalertgroups"}

var projectalertgroupsKind = schema.GroupVersionKind{Group: "management.cattle.io", Version: "v3", Kind: "ProjectAlertGroup"}

// Get takes name of the projectAlertGroup, and returns the corresponding projectAlertGroup object, and an error if there is any.
func (c *FakeProjectAlertGroups) Get(ctx context.Context, name string, options v1.GetOptions) (result *v3.ProjectAlertGroup, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(projectalertgroupsResource, c.ns, name), &v3.ProjectAlertGroup{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v3.ProjectAlertGroup), err
}

// List takes label and field selectors, and returns the list of ProjectAlertGroups that match those selectors.
func (c *FakeProjectAlertGroups) List(ctx context.Context, opts v1.ListOptions) (result *v3.ProjectAlertGroupList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(projectalertgroupsResource, projectalertgroupsKind, c.ns, opts), &v3.ProjectAlertGroupList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v3.ProjectAlertGroupList{ListMeta: obj.(*v3.ProjectAlertGroupList).ListMeta}
	for _, item := range obj.(*v3.ProjectAlertGroupList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested projectAlertGroups.
func (c *FakeProjectAlertGroups) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(projectalertgroupsResource, c.ns, opts))

}

// Create takes the representation of a projectAlertGroup and creates it.  Returns the server's representation of the projectAlertGroup, and an error, if there is any.
func (c *FakeProjectAlertGroups) Create(ctx context.Context, projectAlertGroup *v3.ProjectAlertGroup, opts v1.CreateOptions) (result *v3.ProjectAlertGroup, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(projectalertgroupsResource, c.ns, projectAlertGroup), &v3.ProjectAlertGroup{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v3.ProjectAlertGroup), err
}

// Update takes the representation of a projectAlertGroup and updates it. Returns the server's representation of the projectAlertGroup, and an error, if there is any.
func (c *FakeProjectAlertGroups) Update(ctx context.Context, projectAlertGroup *v3.ProjectAlertGroup, opts v1.UpdateOptions) (result *v3.ProjectAlertGroup, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(projectalertgroupsResource, c.ns, projectAlertGroup), &v3.ProjectAlertGroup{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v3.ProjectAlertGroup), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeProjectAlertGroups) UpdateStatus(ctx context.Context, projectAlertGroup *v3.ProjectAlertGroup, opts v1.UpdateOptions) (*v3.ProjectAlertGroup, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(projectalertgroupsResource, "status", c.ns, projectAlertGroup), &v3.ProjectAlertGroup{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v3.ProjectAlertGroup), err
}

// Delete takes name of the projectAlertGroup and deletes it. Returns an error if one occurs.
func (c *FakeProjectAlertGroups) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(projectalertgroupsResource, c.ns, name, opts), &v3.ProjectAlertGroup{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeProjectAlertGroups) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(projectalertgroupsResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v3.ProjectAlertGroupList{})
	return err
}

// Patch applies the patch and returns the patched projectAlertGroup.
func (c *FakeProjectAlertGroups) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v3.ProjectAlertGroup, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(projectalertgroupsResource, c.ns, name, pt, data, subresources...), &v3.ProjectAlertGroup{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v3.ProjectAlertGroup), err
}