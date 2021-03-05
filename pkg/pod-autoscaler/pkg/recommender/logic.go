package recommender

import (
	"fmt"
	"math"

	metricsv1beta2 "k8s.io/metrics/pkg/apis/custom_metrics/v1beta2"

	"github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"
)

// Logic is the logic with which the recommender suggests new resources
type Logic interface {
	computePodScale(pod *v1.Pod, podScale *v1beta1.PodScale, sla *v1beta1.ServiceLevelAgreement, metric *metricsv1beta2.MetricValue) (*v1beta1.PodScale, error)
}

// ControlTheoryLogic is the logic that apply control theory in order to recommendContainer new resources
type ControlTheoryLogic struct {
	xcprec    float64
	cores     float64
	prevError float64
}

// newControlTheoryLogic returns a new control theory logic
func newControlTheoryLogic(podScale *v1beta1.PodScale) *ControlTheoryLogic {
	return &ControlTheoryLogic{
		xcprec:    float64(containerScale.Status.ActualResources.Cpu().MilliValue()),
		cores:     float64(containerScale.Status.ActualResources.Cpu().MilliValue()),
		prevError: 0.0,
	}
}

const (
	// Control theory constants
	maxScaleOut = 3
	minCPU      = 5
	BC          = 25
	DC          = 50
	minError    = -20000.0
	maxError    = 20000.0
)

// computePodScale computes a new pod scale for a given pod.
// It also requires the old pod scale, the service level agreement and the pod metrics.
func (logic *ControlTheoryLogic) computePodScale(pod *v1.Pod, podScale *v1beta1.PodScale, sla *v1beta1.ServiceLevelAgreement, metric *metricsv1beta2.MetricValue) (*v1beta1.PodScale, error) {

	container, err := ContainerToScale(*pod, sla.Spec.Service.Container)

	if err != nil {
		klog.Info(err)
		return nil, err
	}

	// Compute the cpu and memory value for the pod
	desiredCPU := logic.computeCPUResource(container, sla, metric)
	desiredMemory := logic.computeMemoryResource(container, podScale, sla, metric)

	desiredResources := make(v1.ResourceList)
	desiredResources[v1.ResourceCPU] = *desiredCPU
	desiredResources[v1.ResourceMemory] = *desiredMemory

	cappedResources := make(v1.ResourceList)
	cappedCPU, _ := applyBounds(desiredCPU, sla.Spec.MinResources.Cpu(), sla.Spec.MaxResources.Cpu(), sla.Spec.MinResources != nil, sla.Spec.MaxResources != nil)
	cappedMemory, _ := applyBounds(desiredMemory, sla.Spec.MinResources.Memory(), sla.Spec.MaxResources.Memory(), sla.Spec.MinResources != nil, sla.Spec.MaxResources != nil)
	cappedResources[v1.ResourceCPU] = *cappedCPU
	cappedResources[v1.ResourceMemory] = *cappedMemory

	// Copy the current PodScale and edit the desired value
	newPodScale := podScale.DeepCopy()
	newPodScale.Spec.DesiredResources = desiredResources
	newPodScale.Status.CappedResources = cappedResources

	return newPodScale, nil
}

// computeMemoryResource computes memory resources for a given pod.
func (logic *ControlTheoryLogic) computeMemoryResource(container v1.Container, podScale *v1beta1.PodScale, sla *v1beta1.ServiceLevelAgreement, metric *metricsv1beta2.MetricValue) *resource.Quantity {

	// Retrieve the value of actual and desired cpu resources
	// TODO: maybe can be deleted
	desiredResource := podScale.Spec.DesiredResources.Memory()
	//actualResource := podScale.Status.ActualResources.Memory()

	// Compute the new desired value
	newDesiredResource := resource.NewMilliQuantity(desiredResource.MilliValue(), resource.BinarySI)

	// For logging purpose
	//klog.Info("Computing memory resource for Pod: ", pod.GetName(), ", actual value: ", actualResource, ", desired value: ", desiredResource, ", new value: ", newDesiredResource)

	return newDesiredResource
}

// computeMemoryResource computes memory resources for a given pod.
func (logic *ControlTheoryLogic) computeCPUResource(container v1.Container, sla *v1beta1.ServiceLevelAgreement, metric *metricsv1beta2.MetricValue) *resource.Quantity {

	actualCpu := containerScale.Status.ActualResources.Cpu().MilliValue()
	logic.cores = float64(actualCpu)
	logic.xcprec = logic.cores - BC*logic.prevError

	// Compute the new desired value
	//result := metric.Value.MilliValue()
	//if !ok {
	//	klog.Info(`response_time cannot be casted to Int64. response_time is:`, metric)
	//	return resource.NewMilliQuantity(container.Resources.Requests.Cpu().MilliValue(), resource.BinarySI)
	//}

	responseTime := float64(metric.Value.MilliValue()) / 1000
	// The response time is in seconds
	setPoint := float64(sla.Spec.Metric.ResponseTime.MilliValue()) / 1000
	e := 1/setPoint - 1/responseTime
	e = math.Min(math.Max(e, minError), maxError)
	xc := float64(logic.xcprec + BC*e)
	oldcores := logic.cores
	cores := math.Min(math.Max(minCPU, xc+DC*e), oldcores*maxScaleOut)

	newDesiredResource := resource.NewMilliQuantity(int64(cores), resource.BinarySI)

	klog.Info("xc is: ", xc, ", e is: ", e, ", xcprex is: ", logic.xcprec)
	// Store the value in logic
	logic.cores = cores
	logic.xcprec = logic.cores - BC*e

	// For logging purpose
	klog.Info("BC: ", BC, ", DC: ", DC)
	klog.Info("response time is: ", responseTime, ", set point is: ", setPoint, " and error is: ", e)
	klog.Info("xc is: ", xc, ", cores is: ", cores, ", xcprex is: ", logic.xcprec)
	//klog.Info("Computing CPU resource for Pod: ", pod.GetName(), ", actual value: ", actualResource, ", desired value: ", desiredResource, ", new value: ", newDesiredResource)

	return newDesiredResource
}

// ContainerToScale returns the desired container from the given pod
func ContainerToScale(pod v1.Pod, container string) (v1.Container, error) {
	for _, c := range pod.Spec.Containers {
		if c.Name == container {
			return c, nil
		}
	}

	return v1.Container{}, fmt.Errorf("the container %s does not exists within the pod %s", container, pod.Name)
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
