package recommender

import (
	"github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
)

func (c *Controller) enqueuePodScaleAdded(new interface{}) {
	c.podScalesAddedQueue.Enqueue(new)
}

func (c *Controller) enqueuePodScaleUpdated(old, new interface{}) {
	oldPodScale := old.(*v1beta1.PodScale)
	newPodScale := new.(*v1beta1.PodScale)
	if oldPodScale.Spec.PodRef.Name == newPodScale.Spec.PodRef.Name &&
		oldPodScale.Spec.PodRef.Namespace == newPodScale.Spec.PodRef.Namespace {
		return
	}
	c.enqueuePodScaleAdded(new)
	c.enqueuePodScaleDeleted(old)
}

func (c *Controller) enqueuePodScaleDeleted(old interface{}) {
	c.podScalesDeletedQueue.Enqueue(old)
}
