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

// FixedGainControlLogic is the logic that apply control theory in order to recommendContainer new resources
type FixedGainControlLogic struct {
	xcprec    float64
	cores     float64
	prevError float64
}

// newFixedGainControlLogic returns a new control theory logic
func newFixedGainControlLogic(podScale *v1beta1.PodScale) *FixedGainControlLogic {
	return &FixedGainControlLogic{
		xcprec:    float64(podScale.Status.ActualResources.Cpu().MilliValue()),
		cores:     float64(podScale.Status.ActualResources.Cpu().MilliValue()),
		prevError: 0.0,
	}
}

const (
	// Control theory constants
	maxScaleOut = 1.5
	minCPU      = 5
	BC          = 40
	DC          = 80
	minError    = -10
	maxError    = 10
	minBC       = 10
	minDC       = 15
	maxBC       = 100
	maxDC       = 150
)

// computePodScale computes a new pod scale for a given pod.
// It also requires the old pod scale, the service level agreement and the pod metrics.
func (logic *FixedGainControlLogic) computePodScale(pod *v1.Pod, podScale *v1beta1.PodScale, sla *v1beta1.ServiceLevelAgreement, metric *metricsv1beta2.MetricValue) (*v1beta1.PodScale, error) {

	container, err := ContainerToScale(*pod, sla.Spec.Service.Container)

	if err != nil {
		klog.Info(err)
		return nil, err
	}

	// Compute the cpu and memory value for the pod
	desiredCPU := logic.computeCPUResource(container, podScale, sla, metric)
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
func (logic *FixedGainControlLogic) computeMemoryResource(container v1.Container, podScale *v1beta1.PodScale, sla *v1beta1.ServiceLevelAgreement, metric *metricsv1beta2.MetricValue) *resource.Quantity {

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
func (logic *FixedGainControlLogic) computeCPUResource(container v1.Container, podScale *v1beta1.PodScale, sla *v1beta1.ServiceLevelAgreement, metric *metricsv1beta2.MetricValue) *resource.Quantity {

	actualCpu := podScale.Status.ActualResources.Cpu().MilliValue()
	logic.cores = float64(actualCpu)
	logic.xcprec = logic.cores - BC*logic.prevError

	responseTime := float64(metric.Value.MilliValue()) / 1000
	// The response time is in seconds
	setPoint := float64(sla.Spec.Metric.ResponseTime.MilliValue()) / 1000
	e := 1/setPoint - 1/responseTime
	e = math.Min(math.Max(e, minError), maxError)
	logic.prevError = e
	xc := float64(logic.xcprec + BC*e)
	oldcores := logic.cores
	cores := math.Min(math.Max(minCPU, xc+DC*e), oldcores*maxScaleOut)

	newDesiredResource := resource.NewMilliQuantity(int64(cores), resource.BinarySI)

	klog.Info("xc is: ", xc, ", e is: ", e, ", xcprex is: ", logic.xcprec)

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

// AdaptiveGain is the logic that apply an adaptive gain feedback controller to recommend new resources
type AdaptiveGainControlLogic struct {
	xcprec    float64
	cores     float64
	prevError float64
	bc        float64
	dc        float64
}

// newAdaptiveGainControlLogic returns a new adaptive gain feedback controller
func newAdaptiveGainControlLogic(podScale *v1beta1.PodScale) *AdaptiveGainControlLogic {
	return &AdaptiveGainControlLogic{
		xcprec:    float64(podScale.Status.ActualResources.Cpu().MilliValue()),
		cores:     float64(podScale.Status.ActualResources.Cpu().MilliValue()),
		prevError: 0.0,
		bc:        BC,
		dc:        DC,
	}
}

// computePodScale computes a new pod scale for a given pod.
// It also requires the old pod scale, the service level agreement and the pod metrics.
func (logic *AdaptiveGainControlLogic) computePodScale(pod *v1.Pod, podScale *v1beta1.PodScale, sla *v1beta1.ServiceLevelAgreement, metric *metricsv1beta2.MetricValue) (*v1beta1.PodScale, error) {

	container, err := ContainerToScale(*pod, sla.Spec.Service.Container)

	if err != nil {
		klog.Info(err)
		return nil, err
	}

	// Compute the cpu and memory value for the pod
	desiredCPU := logic.computeCPUResource(container, podScale, sla, metric)
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
func (logic *AdaptiveGainControlLogic) computeMemoryResource(container v1.Container, podScale *v1beta1.PodScale, sla *v1beta1.ServiceLevelAgreement, metric *metricsv1beta2.MetricValue) *resource.Quantity {

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
func (logic *AdaptiveGainControlLogic) computeCPUResource(container v1.Container, podScale *v1beta1.PodScale, sla *v1beta1.ServiceLevelAgreement, metric *metricsv1beta2.MetricValue) *resource.Quantity {

	actualCpu := podScale.Status.ActualResources.Cpu().MilliValue()
	logic.cores = float64(actualCpu)
	logic.xcprec = logic.cores - logic.bc*logic.prevError

	responseTime := float64(metric.Value.MilliValue()) / 1000
	// The response time is in seconds
	setPoint := float64(sla.Spec.Metric.ResponseTime.MilliValue()) / 1000
	e := 1/setPoint - 1/responseTime
	e = math.Min(math.Max(e, minError), maxError)
	logic.prevError = e
	xc := float64(logic.xcprec + logic.bc*e)
	oldcores := logic.cores
	cores := math.Min(math.Max(minCPU, xc+logic.dc*e), oldcores*maxScaleOut)

	// Adapt the gains
	logic.bc = math.Min(math.Max(logic.bc*math.Sqrt(math.Abs(e)*2), minBC), maxBC)
	logic.dc = math.Min(math.Max(logic.bc*DC/BC, minDC), maxDC)

	newDesiredResource := resource.NewMilliQuantity(int64(cores), resource.BinarySI)
	klog.Infof("error is  %v,  bc is %v, dc is %v", e, logic.bc, logic.dc)
	klog.Infof("response_time is %v, old cores are %v, new cores are %v", responseTime, oldcores, cores)
	return newDesiredResource
}
