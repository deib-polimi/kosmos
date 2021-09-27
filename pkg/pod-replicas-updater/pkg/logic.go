package replicaupdater

import (
	"github.com/lterrac/system-autoscaler/pkg/metrics-exposer/pkg/metrics"
	metricsgetter "github.com/lterrac/system-autoscaler/pkg/pod-autoscaler/pkg/metrics"
	"k8s.io/klog/v2"
	"math"
	"time"

	"github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

// Logic is the logic the controller uses to suggest new replica values for an application
type Logic interface {
	//computeReplica computes the number of replicas for an application
	computeReplica(sla *v1beta1.ServiceLevelAgreement, pods []*corev1.Pod, podscales []*v1beta1.PodScale, service *corev1.Service, metricClient metricsgetter.MetricGetter, curReplica int32) int32
}

type LogicState string

// Logic states
const (
	ScalingUpState   LogicState = "scaling_up"
	ScalingDownState LogicState = "scaling_down"
	SteadyState      LogicState = "steady"
)

// HPALogic is the logic that emulates the HPA logic
type HPALogic struct {
	startScaleUpTime   time.Time
	startScaleDownTime time.Time
	stabilizeTime      time.Time
	state              LogicState
}

// newHPALogic returns a new HPA logic
func newHPALogic() *HPALogic {
	return &HPALogic{
		startScaleUpTime:   time.Now(),
		startScaleDownTime: time.Now(),
		stabilizeTime:      time.Now(),
		state:              SteadyState,
	}
}

// newCustomLogic returns a new HPA logic
func newCustomLogic(earlyStop bool) *CustomLogic {
	return &CustomLogic{
		startScaleUpTime:   time.Now(),
		startScaleDownTime: time.Now(),
		stabilizeTime:      time.Now(),
		state:              SteadyState,
		earlyStop:          earlyStop,
	}
}

const (
	scaleUpPeriodMillis   = 30000
	scaleDownPeriodMillis = 30000
	stabilizePeriodMillis = 60000
	tolerance             = 1.2
)

//computeReplica computes the number of replicas for a service, given the serviceLevelAgreement
func (logic *HPALogic) computeReplica(sla *v1beta1.ServiceLevelAgreement, pods []*corev1.Pod, podscales []*v1beta1.PodScale, service *corev1.Service, metricClient metricsgetter.MetricGetter, curReplica int32) int32 {

	minReplicas := sla.Spec.MinReplicas
	maxReplicas := sla.Spec.MaxReplicas

	klog.Info("current_state:", logic.state)

	// If the application has recently changed the amount of replicas, it will wait for it to stabilize
	klog.Info(time.Since(logic.stabilizeTime).Milliseconds())
	if time.Since(logic.stabilizeTime).Milliseconds() < stabilizePeriodMillis {
		logic.state = SteadyState
		return curReplica
	}

	// Compute the desired amount of replica
	desiredTarget := float64(sla.Spec.Metric.ResponseTime.MilliValue())
	responseTime, err := metricClient.ServiceMetrics(service, metrics.ResponseTime)

	if err != nil {
		klog.Errorf("failed to retrieve metrics for service with name %s and namespace %s, error: %s", service.Name, service.Namespace, err)
		return curReplica
	}

	actualTarget := float64(responseTime.Value.MilliValue())

	// Apply constraints
	nReplicas := int32(math.Min(float64(maxReplicas), math.Max(float64(minReplicas), math.Round(actualTarget/desiredTarget*float64(curReplica)))))

	// Check tolerance
	// If the new amount of replicas is between the upper bound and the lower bound
	// do no take any action
	toleranceUpperBound := int32(float64(desiredTarget) * tolerance)
	toleranceLowerBound := int32(float64(desiredTarget) * (2 - tolerance))
	if nReplicas < toleranceUpperBound && nReplicas > toleranceLowerBound {
		logic.state = SteadyState
		return curReplica
	}

	// Scale Up
	if nReplicas > curReplica {
		if logic.state == ScalingUpState {
			if time.Since(logic.startScaleUpTime).Milliseconds() > scaleUpPeriodMillis {
				logic.state = SteadyState
				logic.stabilizeTime = time.Now()
				return nReplicas
			}
		} else {
			logic.state = ScalingUpState
			logic.startScaleUpTime = time.Now()
			return curReplica
		}
		// Scale down
	} else if nReplicas < curReplica {
		if logic.state == ScalingDownState {
			if time.Since(logic.startScaleDownTime).Milliseconds() > scaleDownPeriodMillis {
				logic.state = SteadyState
				logic.stabilizeTime = time.Now()
				return nReplicas
			}
		} else {
			logic.state = ScalingDownState
			logic.startScaleDownTime = time.Now()
			return curReplica
		}
	} else {
		logic.state = SteadyState
	}

	return curReplica
}

// TODO: lot of code is duplicated, we should try handle it
// CustomLogic is the logic that emulates the custom logic
type CustomLogic struct {
	startScaleUpTime   time.Time
	startScaleDownTime time.Time
	stabilizeTime      time.Time
	state              LogicState
	earlyStop          bool
}

//computeReplica computes the number of replicas for a service, given the serviceLevelAgreement
func (logic *CustomLogic) computeReplica(sla *v1beta1.ServiceLevelAgreement, pods []*corev1.Pod, podscales []*v1beta1.PodScale, service *corev1.Service, metricClient metricsgetter.MetricGetter, curReplica int32) int32 {


	minReplicas := sla.Spec.MinReplicas
	maxReplicas := sla.Spec.MaxReplicas

	// If the application has recently changed the amount of replicas, it will wait for it to stabilize
	if time.Since(logic.stabilizeTime).Milliseconds() < stabilizePeriodMillis {
		logic.state = SteadyState
		return curReplica
	}

	nReplicas := curReplica

	// Compute the desired amount of replica
	// Check for upscaling
	for _, scale := range podscales {
		if scale.Spec.DesiredResources.Cpu().MilliValue() > scale.Status.ActualResources.Cpu().MilliValue() {
			nReplicas += 1
			if logic.earlyStop {
				break
			}
		}
	}
	// Check for downscaling
	if curReplica == nReplicas {
		desiredTarget := float64(sla.Spec.Metric.ResponseTime.MilliValue())
		responseTime, err := metricClient.ServiceMetrics(service, metrics.ResponseTime)
		if err != nil {
			klog.Errorf("failed to retrieve metrics for service with name %s and namespace %s, error: %s", service.Name, service.Namespace, err)
			return curReplica
		}
		actualTarget := float64(responseTime.Value.MilliValue())
		// Apply constraints
		downscaledReplicas := int32(math.Min(float64(maxReplicas), math.Max(float64(minReplicas), math.Round(actualTarget/desiredTarget*float64(curReplica)))))

		if downscaledReplicas < nReplicas {
			nReplicas = downscaledReplicas
		}
	}

	nReplicas = int32(math.Min(float64(maxReplicas), math.Max(float64(minReplicas), float64(nReplicas))))

	// Scale Up
	if nReplicas > curReplica {
		if logic.state == ScalingUpState {
			if time.Since(logic.startScaleUpTime).Milliseconds() > scaleUpPeriodMillis {
				logic.state = SteadyState
				logic.stabilizeTime = time.Now()
				return nReplicas
			}
		} else {
			logic.state = ScalingUpState
			logic.startScaleUpTime = time.Now()
			return curReplica
		}
		// Scale down
	} else if nReplicas < curReplica {
		if logic.state == ScalingDownState {
			if time.Since(logic.startScaleDownTime).Milliseconds() > scaleDownPeriodMillis {
				logic.state = SteadyState
				logic.stabilizeTime = time.Now()
				return nReplicas
			}
		} else {
			logic.state = ScalingDownState
			logic.startScaleDownTime = time.Now()
			return curReplica
		}
	} else {
		logic.state = SteadyState
	}

	return curReplica

}
