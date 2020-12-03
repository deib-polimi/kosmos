package recommender

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// Synchronize the pod scale
// Whenever a new one is added, updates the current set of pods contained in podsMap.
func (c *Controller) syncPodScalesAdded(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	klog.Info("Adding pod scale with namespace: ", namespace, " and name: ", name)
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
	pod, err := c.kubernetesClientset.CoreV1().Pods(podScale.Spec.PodRef.Namespace).Get(context.TODO(), podScale.Spec.PodRef.Name, v1.GetOptions{})
	if err != nil {
		return nil
	}

	// Retrieve the node where the pod is deployed
	node := pod.Spec.NodeName

	// Store in the controller state
	result, _ := c.status.nodeMap.LoadOrStore(node, make(map[string]struct{}))
	keys := result.(map[string]struct{})
	keys[key] = sets.Empty{}
	c.status.nodeMap.Store(node, keys)
	c.status.podMap.Store(key, pod)
	c.status.logicMap.Store(key, newControlTheoryLogic())

	return nil
}

// Synchronize the pod scale
// Whenever a pod scale is deleted, updates the current set of pods contained in podsMap.
func (c *Controller) syncPodScalesDeleted(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	klog.Info("Adding pod scale with namespace: ", namespace, " and name: ", name)
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
	pod, err := c.kubernetesClientset.CoreV1().Pods(podScale.Spec.PodRef.Namespace).Get(context.TODO(), podScale.Spec.PodRef.Name, v1.GetOptions{})
	if err != nil {
		return nil
	}

	// Retrieve the node where the pod is deployed
	node := pod.Spec.NodeName

	// Delete the key from controller state
	result, _ := c.status.nodeMap.LoadOrStore(node, make(map[string]struct{}))
	keys := result.(map[string]struct{})
	delete(keys, key)
	if len(keys) == 0 {
		c.status.nodeMap.Delete(node)
	} else {
		c.status.nodeMap.Store(node, keys)
	}
	c.status.podMap.Delete(key)
	c.status.logicMap.Delete(key)

	return nil
}
