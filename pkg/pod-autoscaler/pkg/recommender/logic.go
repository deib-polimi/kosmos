package recommender

import (
	"github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"
)

func (c *Controller) computePodScale(pod *v1.Pod, podScale *v1beta1.PodScale, metricMap map[string]string) *v1beta1.PodScale {

	// Compute the cpu and memory value for the pod
	cpuResource := computeCPUResource(pod, podScale, metricMap)
	memoryResource := computeMemoryResource(pod, podScale, metricMap)

	// Copy the current PodScale and edit the desired value
	newPodScale := podScale.DeepCopy()
	newPodScale.Spec.DesiredResources.Cpu().Set(cpuResource.Value())
	newPodScale.Spec.DesiredResources.Memory().Set(memoryResource.Value())

	return newPodScale
}

func computeMemoryResource(pod *v1.Pod, podScale *v1beta1.PodScale, metricMap map[string]string) *resource.Quantity {

	// Retrieve the value of actual and desired cpu resources
	desiredResource := podScale.Spec.DesiredResources.Memory()
	actualResource := podScale.Status.ActualResources.Memory()

	// Compute the new desired value
	newDesiredResource := resource.NewQuantity(desiredResource.Value()+1, resource.BinarySI)

	// For logging purpose
	klog.Infof("Computing memory resource for Pod: %s, actual value: %s, desired value: %s, new value: %s", pod.Name, actualResource.String(), desiredResource.String(), newDesiredResource.String())

	return newDesiredResource
}

func computeCPUResource(pod *v1.Pod, podScale *v1beta1.PodScale, metricMap map[string]string) *resource.Quantity {

	// Retrieve the value of actual and desired cpu resources
	desiredResource := podScale.Spec.DesiredResources.Cpu()
	actualResource := podScale.Status.ActualResources.Cpu()

	// Compute the new desired value
	newDesiredResource := resource.NewQuantity(desiredResource.Value()+1, resource.BinarySI)

	// For logging purpose
	klog.Infof("Computing CPU resource for Pod: %s, actual value: %s, desired value: %s, new value: %s", pod.Name, actualResource.String(), desiredResource.String(), newDesiredResource.String())

	return newDesiredResource
}
