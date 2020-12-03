package recommender

import (
	"fmt"
	"github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func (c *Controller) enqueuePodScaleAdded(new interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(new); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.podScalesAddedQueue.Add(key)
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
	c.podScalesDeletedQueue.Add(key)
}

//
func (c *Controller) processPodScalesAdded() bool {
	obj, shutdown := c.podScalesAddedQueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		// Signals to the queue that the element has been processed
		defer c.podScalesAddedQueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the form namespace/name.
		if key, ok = obj.(string); !ok {
			// If the item is invalid
			c.podScalesAddedQueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// podScale resource to be synced.
		if err := c.syncPodScalesAdded(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.podScalesAddedQueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.podScalesAddedQueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) processPodScalesDeleted() bool {
	obj, shutdown := c.podScalesDeleted.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		// Signals to the queue that the element has been processed
		defer c.podScalesDeletedQueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the form namespace/name.
		if key, ok = obj.(string); !ok {
			// If the item is invalid
			c.podScalesDeletedQueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// podScale resource to be synced.
		if err := c.syncPodScalesDeleted(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.podScalesDeletedQueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.podScalesDeletedQueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}


func (c *Controller) processRecommendNode() bool {
	obj, shutdown := c.recommendNodeQueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		// Signals to the queue that the element has been processed
		defer c.recommendNodeQueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the form namespace/name.
		if key, ok = obj.(string); !ok {
			// If the item is invalid
			c.recommendNodeQueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// podScale resource to be synced.
		if err := c.recommendNode(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.recommendNodeQueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.recommendNodeQueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}
