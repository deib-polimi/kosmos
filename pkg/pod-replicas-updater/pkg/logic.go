package replicaupdater

import (
	"github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"math"
	"time"
)

// Logic is the logic the controller uses to suggest new replica values for an application
type Logic interface {
	//computeReplica computes the number of replicas for an application
	computeReplica(sla *v1beta1.ServiceLevelAgreement, pods []*corev1.Pod, podscales []*v1beta1.ContainerScale, metrics []map[string]interface{}, curReplica int32) int32
}

type HPALogicState string

// Logic states
const (
	ScalingUpState   HPALogicState = "scaling_up"
	ScalingDownState HPALogicState = "scaling_down"
	SteadyState      HPALogicState = "steady"
)

// HPALogic is the logic that emulates the HPA logic
type HPALogic struct {
	startScaleUpTime   time.Time
	startScaleDownTime time.Time
	stabilizeTime      time.Time
	state              HPALogicState
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

const (
	scaleUpPeriodMillis   = 15000
	scaleDownPeriodMillis = 15000
	stabilizePeriodMillis = 30000
	tolerance             = 1.10
)

//computeReplica computes the number of replicas for a service, given the serviceLevelAgreement
func (logic *HPALogic) computeReplica(sla *v1beta1.ServiceLevelAgreement, pods []*corev1.Pod, podscales []*v1beta1.ContainerScale, metrics []map[string]interface{}, curReplica int32) int32 {

	minReplicas := sla.Spec.MinReplicas
	maxReplicas := sla.Spec.MaxReplicas

	// If the application has recently changed the amount of replicas, it will wait for it to stabilize
	if time.Since(logic.stabilizeTime).Milliseconds() < stabilizePeriodMillis {
		logic.state = SteadyState
		return curReplica
	}

	// Compute the desired amount of replica
	desiredTarget := float64(sla.Spec.Metric.ResponseTime.MilliValue())
	actualTarget := 0.0
	for _, metric := range metrics {
		result, ok := metric["response_time"]
		if !ok {
			klog.Info(`"response_time" was not in metrics. Metrics are:`, metric)
		}
		actualTarget += result.(float64)
	}
	nPods := float64(len(metrics))
	actualTarget = actualTarget / nPods

	// Apply constraints
	nReplicas := int32(math.Min(float64(maxReplicas), math.Max(float64(minReplicas), math.Round(actualTarget/desiredTarget*nPods))))

	// Check tolerance
	// If the new amount of replicas is between the upper bound and the lower bound
	// do no take any action
	toleranceUpperBound := int32(float64(nReplicas) * tolerance)
	toleranceLowerBound := int32(float64(nReplicas) * (tolerance - 1))
	if nReplicas < toleranceUpperBound && nReplicas > toleranceLowerBound {
		logic.state = SteadyState
		return nReplicas
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
