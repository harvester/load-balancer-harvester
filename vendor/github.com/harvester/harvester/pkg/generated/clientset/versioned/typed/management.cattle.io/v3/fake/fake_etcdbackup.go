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

// FakeEtcdBackups implements EtcdBackupInterface
type FakeEtcdBackups struct {
	Fake *FakeManagementV3
	ns   string
}

var etcdbackupsResource = schema.GroupVersionResource{Group: "management.cattle.io", Version: "v3", Resource: "etcdbackups"}

var etcdbackupsKind = schema.GroupVersionKind{Group: "management.cattle.io", Version: "v3", Kind: "EtcdBackup"}

// Get takes name of the etcdBackup, and returns the corresponding etcdBackup object, and an error if there is any.
func (c *FakeEtcdBackups) Get(ctx context.Context, name string, options v1.GetOptions) (result *v3.EtcdBackup, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(etcdbackupsResource, c.ns, name), &v3.EtcdBackup{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v3.EtcdBackup), err
}

// List takes label and field selectors, and returns the list of EtcdBackups that match those selectors.
func (c *FakeEtcdBackups) List(ctx context.Context, opts v1.ListOptions) (result *v3.EtcdBackupList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(etcdbackupsResource, etcdbackupsKind, c.ns, opts), &v3.EtcdBackupList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v3.EtcdBackupList{ListMeta: obj.(*v3.EtcdBackupList).ListMeta}
	for _, item := range obj.(*v3.EtcdBackupList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested etcdBackups.
func (c *FakeEtcdBackups) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(etcdbackupsResource, c.ns, opts))

}

// Create takes the representation of a etcdBackup and creates it.  Returns the server's representation of the etcdBackup, and an error, if there is any.
func (c *FakeEtcdBackups) Create(ctx context.Context, etcdBackup *v3.EtcdBackup, opts v1.CreateOptions) (result *v3.EtcdBackup, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(etcdbackupsResource, c.ns, etcdBackup), &v3.EtcdBackup{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v3.EtcdBackup), err
}

// Update takes the representation of a etcdBackup and updates it. Returns the server's representation of the etcdBackup, and an error, if there is any.
func (c *FakeEtcdBackups) Update(ctx context.Context, etcdBackup *v3.EtcdBackup, opts v1.UpdateOptions) (result *v3.EtcdBackup, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(etcdbackupsResource, c.ns, etcdBackup), &v3.EtcdBackup{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v3.EtcdBackup), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeEtcdBackups) UpdateStatus(ctx context.Context, etcdBackup *v3.EtcdBackup, opts v1.UpdateOptions) (*v3.EtcdBackup, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(etcdbackupsResource, "status", c.ns, etcdBackup), &v3.EtcdBackup{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v3.EtcdBackup), err
}

// Delete takes name of the etcdBackup and deletes it. Returns an error if one occurs.
func (c *FakeEtcdBackups) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(etcdbackupsResource, c.ns, name, opts), &v3.EtcdBackup{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeEtcdBackups) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(etcdbackupsResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v3.EtcdBackupList{})
	return err
}

// Patch applies the patch and returns the patched etcdBackup.
func (c *FakeEtcdBackups) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v3.EtcdBackup, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(etcdbackupsResource, c.ns, name, pt, data, subresources...), &v3.EtcdBackup{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v3.EtcdBackup), err
}
