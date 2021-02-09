package recommender

import (
	"math"

	"github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"
)

// Logic is the logic with which the recommender suggests new resources
type Logic interface {
	computeContainerScale(pod *v1.Pod, containerScale *v1beta1.ContainerScale, sla *v1beta1.ServiceLevelAgreement, metricMap map[string]interface{}) *v1beta1.ContainerScale
}

// ControlTheoryLogic is the logic that apply control theory in order to recommendPod new resources
type ControlTheoryLogic struct {
	xcprec float64
	cores  float64
}

// newControlTheoryLogic returns a new control theory logic
func newControlTheoryLogic(containerScale *v1beta1.ContainerScale) *ControlTheoryLogic {
	return &ControlTheoryLogic{
		xcprec: float64(containerScale.Status.ActualResources.Cpu().MilliValue()),
		cores:  float64(containerScale.Status.ActualResources.Cpu().MilliValue()),
	}
}

const (
	// Control theory constants
	maxScaleOut = 3
	minCPU      = 5
	BC          = 5
	DC          = 10
)

// computeContainerScale computes a new pod scale for a given pod.
// It also requires the old pod scale, the service level agreement and the pod metrics.
func (logic *ControlTheoryLogic) computeContainerScale(pod *v1.Pod, containerScale *v1beta1.ContainerScale, sla *v1beta1.ServiceLevelAgreement, metricMap map[string]interface{}) *v1beta1.ContainerScale {

	// Compute the cpu and memory value for the pod
	cpuResource := logic.computeCPUResource(pod, sla, metricMap)
	memoryResource := logic.computeMemoryResource(pod, containerScale, sla, metricMap)
	desiredResource := make(v1.ResourceList)
	desiredResource[v1.ResourceCPU] = *cpuResource
	desiredResource[v1.ResourceMemory] = *memoryResource

	// Copy the current ContainerScale and edit the desired value
	newContainerScale := containerScale.DeepCopy()
	newContainerScale.Spec.DesiredResources = desiredResource

	return newContainerScale
}

// computeMemoryResource computes memory resources for a given pod.
func (logic *ControlTheoryLogic) computeMemoryResource(pod *v1.Pod, containerScale *v1beta1.ContainerScale, sla *v1beta1.ServiceLevelAgreement, metricMap map[string]interface{}) *resource.Quantity {

	// Retrieve the value of actual and desired cpu resources
	// TODO: maybe can be deleted
	desiredResource := containerScale.Spec.DesiredResources.Memory()
	actualResource := containerScale.Status.ActualResources.Memory()

	// Compute the new desired value
	newDesiredResource := resource.NewMilliQuantity(desiredResource.MilliValue(), resource.BinarySI)
	newDesiredResource, _ = applyBounds(newDesiredResource, sla.Spec.MinResources.Memory(), sla.Spec.MaxResources.Memory(), sla.Spec.MinResources != nil, sla.Spec.MaxResources != nil)

	// For logging purpose
	klog.Info("Computing memory resource for Pod: ", pod.GetName(), ", actual value: ", actualResource, ", desired value: ", desiredResource, ", new value: ", newDesiredResource)

	return newDesiredResource
}

// computeMemoryResource computes cpu resources for a given pod.
func (logic *ControlTheoryLogic) computeCPUResource(pod *v1.Pod, sla *v1beta1.ServiceLevelAgreement, metricMap map[string]interface{}) *resource.Quantity {

	// Compute the new desired value
	result, ok := metricMap["response_time"]
	if !ok {
		klog.Info(`"response_time" was not in metrics. Metrics are:`, metricMap)
		//TODO: extract resources from pod
		return nil
	}
	responseTime := result.(float64) / 1000
	// The response time is in seconds
	setPoint := float64(sla.Spec.Metric.ResponseTime.MilliValue()) / 1000
	e := 1/setPoint - 1/responseTime
	xc := float64(logic.xcprec + BC*e)
	oldcores := logic.cores
	cores := math.Min(math.Max(minCPU, xc+DC*e), oldcores*maxScaleOut)

	newDesiredResource := resource.NewMilliQuantity(int64(cores), resource.BinarySI)
	newDesiredResource, bounded := applyBounds(newDesiredResource, sla.Spec.MinResources.Cpu(), sla.Spec.MaxResources.Cpu(), sla.Spec.MinResources != nil, sla.Spec.MaxResources != nil)

	klog.Info("xc is: ", xc, ", e is: ", e, ", xcprex is: ", logic.xcprec)
	// Store the value in logic
	if bounded {
		logic.cores = float64(newDesiredResource.MilliValue())
	} else {
		logic.cores = cores
	}
	logic.xcprec = logic.cores - BC*e

	// For logging purpose
	klog.Info("BC: ", BC, ", DC: ", DC)
	klog.Info("response time is: ", responseTime, ", set point is: ", setPoint, " and error is: ", e)
	klog.Info("xc is: ", xc, ", cores is: ", cores, ", xcprex is: ", logic.xcprec)
	//klog.Info("Computing CPU resource for Pod: ", pod.GetName(), ", actual value: ", actualResource, ", desired value: ", desiredResource, ", new value: ", newDesiredResource)

	return newDesiredResource
}

func applyBounds(value *resource.Quantity, min *resource.Quantity, max *resource.Quantity, checkLower bool, checkUpper bool) (*resource.Quantity, bool) {
	if checkUpper && value.MilliValue() > max.MilliValue() {
		return max, true
	} else if checkLower && value.MilliValue() < min.MilliValue() {
		return min, true
	} else {
		return value, false
	}
}
