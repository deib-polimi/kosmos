package controller

import (
	"fmt"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

type syncFunc func(key string) error

// processNextQueueItem will read a single item from the workqueue and
// attempt to process it, by calling the syncFunc.
func (c *Controller) processNextQueueItem(queue workqueue.RateLimitingInterface, sync syncFunc) bool {
	obj, shutdown := queue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		defer queue.Done(obj)

		var key string
		var ok bool

		if key, ok = obj.(string); !ok {
			queue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueues but got %#v", obj))
			return nil
		}

		// Run the syncHandler, passing it the namespace/name string of the
		// resource to be synced.
		if err := sync(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			queue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		queue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}
