package recommender

import (
	"fmt"
	"github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func (c *Controller) enqueuePodScaleAdded(new interface{}) {
	podScale, _ := new.(v1beta1.PodScale)
	klog.Info("Add PodScale ", podScale.Name, " in namespace ", podScale.Namespace)
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(podScale); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.podScalesAdded.Add(key)
}

func (c *Controller) enqueuePodScaleUpdated(old, new interface{}) {
	oldPodScale := old.(v1beta1.PodScale)
	newPodScale := new.(v1beta1.PodScale)
	// To avoid race condition when unnecessary
	if oldPodScale.Spec.PodRef.Name != newPodScale.Spec.PodRef.Name {
		c.podScalesAdded.Add(new)
		c.podScalesDeleted.Add(old)
	}
}

func (c *Controller) enqueuePodScaleDeleted(old interface{}) {
	podScale, _ := old.(v1beta1.PodScale)
	klog.Info("Deleted PodScale ", podScale.Name, " in namespace ", podScale.Namespace)
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(podScale); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.podScalesDeleted.Add(key)
}

//
func (c *Controller) processPodScalesAdded() bool {
	obj, shutdown := c.podScalesAdded.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		// Signals to the queue that the element has been processed
		defer c.podScalesAdded.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the form namespace/name.
		if key, ok = obj.(string); !ok {
			// If the item is invalid
			c.podScalesAdded.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// podScale resource to be synced.
		if err := c.syncPodScalesAdded(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.podScalesAdded.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.podScalesAdded.Forget(obj)
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
		defer c.podScalesDeleted.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the form namespace/name.
		if key, ok = obj.(string); !ok {
			// If the item is invalid
			c.podScalesDeleted.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// podScale resource to be synced.
		if err := c.syncPodScalesDeleted(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.podScalesDeleted.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.podScalesDeleted.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}
