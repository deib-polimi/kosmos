package resourceupdater

import (
	"fmt"

	"github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func syncPod(pod *v1.Pod, podScale v1beta1.PodScale) (*v1.Pod, error) {

	newPod := pod.DeepCopy()

	if newPod.Status.QOSClass != v1.PodQOSGuaranteed {
		return nil, fmt.Errorf("the pod has %v but it should have 'guaranteed' QOS class", newPod.Status.QOSClass)
	}

	if podScale.Status.ActualResources.Cpu().MilliValue() <= 0 {
		return nil, fmt.Errorf("pod scale must have positive cpu resource value, actual value: %v", podScale.Status.ActualResources.Cpu().ScaledValue(resource.Milli))
	}

	if podScale.Status.ActualResources.Memory().MilliValue() <= 0 {
		return nil, fmt.Errorf("pod scale must have positive memory resource value, actual value: %v", podScale.Status.ActualResources.Memory().ScaledValue(resource.Mega))
	}

	for i, container := range newPod.Spec.Containers {
		if container.Name == podScale.Spec.Container {
			container.Resources.Requests = podScale.Status.ActualResources
			container.Resources.Limits = podScale.Status.ActualResources
			newPod.Spec.Containers[i] = container
			break
		}
	}

	return newPod, nil

}
