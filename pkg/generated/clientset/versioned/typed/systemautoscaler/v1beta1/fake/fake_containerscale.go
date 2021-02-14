// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1beta1 "github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeContainerScales implements ContainerScaleInterface
type FakeContainerScales struct {
	Fake *FakeSystemautoscalerV1beta1
	ns   string
}

var containerscalesResource = schema.GroupVersionResource{Group: "systemautoscaler.polimi.it", Version: "v1beta1", Resource: "containerscales"}

var containerscalesKind = schema.GroupVersionKind{Group: "systemautoscaler.polimi.it", Version: "v1beta1", Kind: "ContainerScale"}

// Get takes name of the containerScale, and returns the corresponding containerScale object, and an error if there is any.
func (c *FakeContainerScales) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1beta1.ContainerScale, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(containerscalesResource, c.ns, name), &v1beta1.ContainerScale{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ContainerScale), err
}

// List takes label and field selectors, and returns the list of ContainerScales that match those selectors.
func (c *FakeContainerScales) List(ctx context.Context, opts v1.ListOptions) (result *v1beta1.ContainerScaleList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(containerscalesResource, containerscalesKind, c.ns, opts), &v1beta1.ContainerScaleList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1beta1.ContainerScaleList{ListMeta: obj.(*v1beta1.ContainerScaleList).ListMeta}
	for _, item := range obj.(*v1beta1.ContainerScaleList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested containerScales.
func (c *FakeContainerScales) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(containerscalesResource, c.ns, opts))

}

// Create takes the representation of a containerScale and creates it.  Returns the server's representation of the containerScale, and an error, if there is any.
func (c *FakeContainerScales) Create(ctx context.Context, containerScale *v1beta1.ContainerScale, opts v1.CreateOptions) (result *v1beta1.ContainerScale, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(containerscalesResource, c.ns, containerScale), &v1beta1.ContainerScale{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ContainerScale), err
}

// Update takes the representation of a containerScale and updates it. Returns the server's representation of the containerScale, and an error, if there is any.
func (c *FakeContainerScales) Update(ctx context.Context, containerScale *v1beta1.ContainerScale, opts v1.UpdateOptions) (result *v1beta1.ContainerScale, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(containerscalesResource, c.ns, containerScale), &v1beta1.ContainerScale{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ContainerScale), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeContainerScales) UpdateStatus(ctx context.Context, containerScale *v1beta1.ContainerScale, opts v1.UpdateOptions) (*v1beta1.ContainerScale, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(containerscalesResource, "status", c.ns, containerScale), &v1beta1.ContainerScale{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ContainerScale), err
}

// Delete takes name of the containerScale and deletes it. Returns an error if one occurs.
func (c *FakeContainerScales) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(containerscalesResource, c.ns, name), &v1beta1.ContainerScale{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeContainerScales) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(containerscalesResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1beta1.ContainerScaleList{})
	return err
}

// Patch applies the patch and returns the patched containerScale.
func (c *FakeContainerScales) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1beta1.ContainerScale, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(containerscalesResource, c.ns, name, pt, data, subresources...), &v1beta1.ContainerScale{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ContainerScale), err
}