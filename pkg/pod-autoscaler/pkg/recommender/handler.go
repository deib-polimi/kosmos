package recommender

import (
	"github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
)

func (c *Controller) enqueuePodScaleAdded(new interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(new); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.podScalesAddedQueue.Queue.AddRateLimited(key)
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
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(old); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.podScalesDeletedQueue.Queue.AddRateLimited(key)
}
