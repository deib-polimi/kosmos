package recommender

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
)

// Synchronize the pod scale
// Whenever a new one is added, updates the current set of pods contained in podsMap.
func (c *Controller) syncPodScalesAdded(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the PodScale resource with this namespace/name
	podScale, err := c.podScalesLister.PodScales(namespace).Get(name)
	if err != nil {
		// The PodScale resource may no longer exist, in which case we stop processing.
		if errors.IsNotFound(err) {
			// PodScale cleanup is achieved via OwnerReferences
			utilruntime.HandleError(fmt.Errorf("PodScale '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	// Get the pod associated with the pod scale
	pod, err := c.kubernetesClientset.CoreV1().Pods(podScale.Namespace).Get(context.TODO(), podScale.Name, v1.GetOptions{})

	// Store the pod in the internal map
	c.podsMap.Store(key, pod)

	return nil
}

// Synchronize the pod scale
// Whenever a pod scale is deleted, updates the current set of pods contained in podsMap.
func (c *Controller) syncPodScalesDeleted(key string) error {
	// Delete the element
	c.podsMap.Delete(key)
	return nil
}
