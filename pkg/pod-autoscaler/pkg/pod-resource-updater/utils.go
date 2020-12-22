package resourceupdater

import (
	"fmt"

	"github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func syncPod(pod v1.Pod, podScale v1beta1.PodScale) (*v1.Pod, error) {

	newPod := pod.DeepCopy()

	// TODO: we should be handle pod with multiple containers
	if len(newPod.Spec.Containers) != 1 {
		return nil, fmt.Errorf("the pod must have only 1 container. containers: %v", newPod.Spec.Containers)
	}

	if newPod.Status.QOSClass != v1.PodQOSGuaranteed {
		return nil, fmt.Errorf("the pod has %v but it should have 'guaranteed' QOS class", newPod.Status.QOSClass)
	}

	if podScale.Status.ActualResources.Cpu().MilliValue() <= 0 {
		return nil, fmt.Errorf("pod scale must have positive cpu resource value, actual value: %v", podScale.Status.ActualResources.Cpu().ScaledValue(resource.Milli))
	}

	if podScale.Status.ActualResources.Memory().MilliValue() <= 0 {
		return nil, fmt.Errorf("pod scale must have positive memory resource value, actual value: %v", podScale.Status.ActualResources.Memory().ScaledValue(resource.Mega))
	}

	newPod.Spec.Containers[0].Resources.Requests = podScale.Status.ActualResources
	newPod.Spec.Containers[0].Resources.Limits = podScale.Status.ActualResources

	// TODO: I should check that the QOS class is 'GUARANTEED'
	// klog.Info(newPod.Status.QOSClass)
	return newPod, nil

}
