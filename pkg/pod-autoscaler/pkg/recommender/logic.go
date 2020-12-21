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
	computePodScale(pod *v1.Pod, podScale *v1beta1.PodScale, sla *v1beta1.ServiceLevelAgreement, metricMap map[string]interface{}) *v1beta1.PodScale
}

// ControlTheoryLogic is the logic that apply control theory in order to recommendPod new resources
type ControlTheoryLogic struct {
	xcprec float64
	cores  float64
}

// newControlTheoryLogic returns a new control theory logic
func newControlTheoryLogic(podScale *v1beta1.PodScale) *ControlTheoryLogic {
	return &ControlTheoryLogic{
		xcprec: float64(podScale.Status.ActualResources.Cpu().ScaledValue(cpuDefaultScale)),
		cores:  float64(podScale.Status.ActualResources.Cpu().ScaledValue(cpuDefaultScale)),
	}
}

const (
	// TODO: for now, default values are MegaBytes for memory and millicpu per seconds for cpu.
	cpuDefaultScale    = resource.Milli
	memoryDefaultScale = resource.Mega

	// Control theory constants
	maxScaleOut = 3
	// TODO: sometime this constraint does not work. Check why.
	// the min value sometimes has conflict with max scale out.
	// For example if CPU is 50m, and max scaleout is 3, then the maximum value is 150m, but it is still less than minCPU
	minCPU = 50
	BC     = 100
	DC     = 100
)

// computePodScale computes a new pod scale for a given pod.
// It also requires the old pod scale, the service level agreement and the pod metrics.
func (logic *ControlTheoryLogic) computePodScale(pod *v1.Pod, podScale *v1beta1.PodScale, sla *v1beta1.ServiceLevelAgreement, metricMap map[string]interface{}) *v1beta1.PodScale {

	// Compute the cpu and memory value for the pod
	cpuResource := logic.computeCPUResource(pod, podScale, sla, metricMap)
	memoryResource := logic.computeMemoryResource(pod, podScale, sla, metricMap)
	desiredResource := make(v1.ResourceList)
	desiredResource[v1.ResourceCPU] = *cpuResource
	desiredResource[v1.ResourceMemory] = *memoryResource

	// Copy the current PodScale and edit the desired value
	newPodScale := podScale.DeepCopy()
	newPodScale.Spec.DesiredResources = desiredResource

	return newPodScale
}

// computeMemoryResource computes memory resources for a given pod.
func (logic *ControlTheoryLogic) computeMemoryResource(pod *v1.Pod, podScale *v1beta1.PodScale, sla *v1beta1.ServiceLevelAgreement, metricMap map[string]interface{}) *resource.Quantity {

	// Retrieve the value of actual and desired cpu resources
	desiredResource := podScale.Spec.DesiredResources.Memory()
	actualResource := podScale.Status.ActualResources.Memory()

	// Compute the new desired value
	newDesiredResource := resource.NewScaledQuantity(desiredResource.ScaledValue(memoryDefaultScale), memoryDefaultScale)
	newDesiredResource = applyBounds(newDesiredResource, sla.Spec.MinResources.Memory(), sla.Spec.MaxResources.Memory())

	// For logging purpose
	klog.Info("Computing memory resource for Pod: ", pod.GetName(), ", actual value: ", actualResource, ", desired value: ", desiredResource, ", new value: ", newDesiredResource)

	return newDesiredResource
}

// computeMemoryResource computes cpu resources for a given pod.
func (logic *ControlTheoryLogic) computeCPUResource(pod *v1.Pod, podScale *v1beta1.PodScale, sla *v1beta1.ServiceLevelAgreement, metricMap map[string]interface{}) *resource.Quantity {

	// Retrieve the value of actual and desired cpu resources
	desiredResource := podScale.Spec.DesiredResources.Cpu()
	actualResource := podScale.Status.ActualResources.Cpu()

	// Compute the new desired value
	result, ok := metricMap["response_time"]
	if !ok {
		klog.Info(`"response_time" was not in metrics. Metrics are:`, metricMap)
		return desiredResource
	}
	responseTime := result.(float64)
	// TODO: decide if to use milliseconds or seconds as default unit
	setPoint := float64(*sla.Spec.Metric.ResponseTime) / 1000
	e := 1/setPoint - 1/responseTime
	xc := float64(logic.xcprec + BC*e)
	oldcores := logic.cores
	logic.cores = math.Min(math.Max(minCPU, xc+DC*e), oldcores*maxScaleOut)
	logic.xcprec = float64(logic.cores - BC*e)

	newDesiredResource := resource.NewScaledQuantity(int64(logic.cores), cpuDefaultScale)
	newDesiredResource = applyBounds(newDesiredResource, sla.Spec.MinResources.Cpu(), sla.Spec.MaxResources.Cpu())

	// For logging purpose
	klog.Info("Computing CPU resource for Pod: ", pod.GetName(), ", actual value: ", actualResource, ", desired value: ", desiredResource, ", new value: ", newDesiredResource)

	return newDesiredResource
}

func applyBounds(value *resource.Quantity, min *resource.Quantity, max *resource.Quantity) *resource.Quantity{
	if value.Value() > max.Value() {
		return max
	} else if value.Value() < min.Value() {
		return min
	} else {
		return value
	}
}
