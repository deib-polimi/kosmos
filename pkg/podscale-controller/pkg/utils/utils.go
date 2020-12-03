package utils

import (
	"github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

// StateDiff wraps the changes to apply in the namespace to make it coherent with
// the declared state.
type StateDiff struct {
	AddList    []*corev1.Pod
	DeleteList []*v1beta1.PodScale
}

// DiffPods returns `Pods` that does not already have an associated
// `PodScale` resource and the old `PodScale` resources to delete.
func DiffPods(pods []*corev1.Pod, podScales []*v1beta1.PodScale) (result StateDiff) {
	blueprint := make(map[string]bool)

	for _, podscale := range podScales {
		blueprint[podscale.Spec.PodRef.Name] = true
	}

	for _, pod := range pods {
		if !blueprint[pod.Name] {
			result.AddList = append(result.AddList, pod)
		}
	}

	blueprint = make(map[string]bool)

	for _, pod := range pods {
		blueprint[pod.Name] = true
	}

	for _, podscale := range podScales {
		if !blueprint[podscale.Spec.PodRef.Name] {
			result.DeleteList = append(result.DeleteList, podscale)
		}
	}

	return result
}

// ContainsService looks for a given element inside a Service list
func ContainsService(list []*corev1.Service, element *corev1.Service) bool {
	for _, e := range list {
		if e == element {
			return true
		}
	}
	return false
}
