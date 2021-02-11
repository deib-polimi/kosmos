package resourceupdater

import (
	"fmt"
	"github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func syncPod(pod *v1.Pod, containerScale v1beta1.ContainerScale) (*v1.Pod, error) {

	newPod := pod.DeepCopy()

	if newPod.Status.QOSClass != v1.PodQOSGuaranteed {
		return nil, fmt.Errorf("the pod has %v but it should have 'guaranteed' QOS class", newPod.Status.QOSClass)
	}

	if containerScale.Status.ActualResources.Cpu().MilliValue() <= 0 {
		return nil, fmt.Errorf("pod scale must have positive cpu resource value, actual value: %v", containerScale.Status.ActualResources.Cpu().ScaledValue(resource.Milli))
	}

	if containerScale.Status.ActualResources.Memory().MilliValue() <= 0 {
		return nil, fmt.Errorf("pod scale must have positive memory resource value, actual value: %v", containerScale.Status.ActualResources.Memory().ScaledValue(resource.Mega))
	}

	for i, container := range newPod.Spec.Containers {
		if container.Name == containerScale.Spec.Container {
			container.Resources.Requests = containerScale.Status.ActualResources
			container.Resources.Limits = containerScale.Status.ActualResources
			newPod.Spec.Containers[i] = container
			break
		}
	}

	return newPod, nil

}
