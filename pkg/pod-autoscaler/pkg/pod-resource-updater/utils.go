package resourceupdater

import (
	"fmt"

	"github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func syncPod(pod v1.Pod, containerScale v1beta1.ContainerScale) (*v1.Pod, error) {

	newPod := pod.DeepCopy()

	// TODO: we should be handle pod with multiple containers
	//if len(newPod.Spec.Containers) != 1 {
	//	return nil, fmt.Errorf("the pod must have only 1 container. containers: %v", newPod.Spec.Containers)
	//}

	if newPod.Status.QOSClass != v1.PodQOSGuaranteed {
		return nil, fmt.Errorf("the pod has %v but it should have 'guaranteed' QOS class", newPod.Status.QOSClass)
	}

	if containerScale.Status.ActualResources.Cpu().MilliValue() <= 0 {
		return nil, fmt.Errorf("pod scale must have positive cpu resource value, actual value: %v", containerScale.Status.ActualResources.Cpu().ScaledValue(resource.Milli))
	}

	if containerScale.Status.ActualResources.Memory().MilliValue() <= 0 {
		return nil, fmt.Errorf("pod scale must have positive memory resource value, actual value: %v", containerScale.Status.ActualResources.Memory().ScaledValue(resource.Mega))
	}

	newPod.Spec.Containers[0].Resources.Requests = containerScale.Status.ActualResources
	newPod.Spec.Containers[0].Resources.Limits = containerScale.Status.ActualResources

	// TODO: I should check that the QOS class is 'GUARANTEED'
	// klog.Info(newPod.Status.QOSClass)
	return newPod, nil

}
