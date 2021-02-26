package recommender

import (
	"encoding/json"
	"k8s.io/klog/v2"
	"testing"

	"github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta2 "k8s.io/metrics/pkg/apis/custom_metrics/v1beta2"
)

func TestControlTheoryLogic(t *testing.T) {

	testcases := []struct {
		description         string
		currentResponseTime float64
		desiredResponseTime int64
		upperBound          int64
		lowerBound          int64
	}{
		{
			description:         "the lower bound should never exceed",
			currentResponseTime: 50,
			desiredResponseTime: 5000,
			upperBound:          2000,
			lowerBound:          1000,
		},
		{
			description:         "the upper bound should never exceed",
			currentResponseTime: 5000,
			desiredResponseTime: 50,
			upperBound:          2000,
			lowerBound:          1000,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {

			sla := &v1beta1.ServiceLevelAgreement{
				TypeMeta: metav1.TypeMeta{APIVersion: v1beta1.SchemeGroupVersion.String()},
				Spec: v1beta1.ServiceLevelAgreementSpec{
					Metric: v1beta1.MetricRequirement{
						ResponseTime: *resource.NewQuantity(tt.desiredResponseTime, resource.BinarySI),
					},
					MinResources: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    *resource.NewScaledQuantity(tt.lowerBound, resource.Milli),
						corev1.ResourceMemory: *resource.NewScaledQuantity(tt.lowerBound, resource.Milli),
					},
					MaxResources: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    *resource.NewScaledQuantity(tt.upperBound, resource.Milli),
						corev1.ResourceMemory: *resource.NewScaledQuantity(tt.upperBound, resource.Milli),
					},
					Service: &v1beta1.Service{
						Container: "container",
					},
				},
			}

			pod := &corev1.Pod{
				TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "pods"},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "container",
							Resources: corev1.ResourceRequirements{
								Limits: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceCPU:    *resource.NewScaledQuantity((tt.lowerBound+tt.upperBound)/2, resource.Milli),
									corev1.ResourceMemory: *resource.NewScaledQuantity((tt.lowerBound+tt.upperBound)/2, resource.Milli),
								},
								Requests: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceCPU:    *resource.NewScaledQuantity((tt.lowerBound+tt.upperBound)/2, resource.Milli),
									corev1.ResourceMemory: *resource.NewScaledQuantity((tt.lowerBound+tt.upperBound)/2, resource.Milli),
								},
							},
						},
					},
				},
			}

			containerScale := &v1beta1.ContainerScale{
				TypeMeta:   metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{},
				Spec: v1beta1.ContainerScaleSpec{
					DesiredResources: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    *resource.NewScaledQuantity((tt.lowerBound+tt.upperBound)/2, resource.Milli),
						corev1.ResourceMemory: *resource.NewScaledQuantity((tt.lowerBound+tt.upperBound)/2, resource.Milli),
					},
				},
				Status: v1beta1.ContainerScaleStatus{
					ActualResources: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    *resource.NewScaledQuantity((tt.lowerBound+tt.upperBound)/2, resource.Milli),
						corev1.ResourceMemory: *resource.NewScaledQuantity((tt.lowerBound+tt.upperBound)/2, resource.Milli),
					},
					CappedResources: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    *resource.NewScaledQuantity((tt.lowerBound+tt.upperBound)/2, resource.Milli),
						corev1.ResourceMemory: *resource.NewScaledQuantity((tt.lowerBound+tt.upperBound)/2, resource.Milli),
					},
				},
			}

			metricsMap := metricsv1beta2.MetricValue{
				Value: *resource.NewQuantity(int64(tt.currentResponseTime), resource.BinarySI),
			}

			logic := ControlTheoryLogic{
				xcprec: float64(tt.lowerBound+tt.upperBound) / 2,
				cores:  float64(tt.lowerBound+tt.upperBound) / 2,
			}

			for i := 0; i < 200; i++ {
				containerScale, err := logic.computeContainerScale(pod, containerScale, sla, &metricsMap)
				require.Nil(t, err)
				require.GreaterOrEqual(t, containerScale.Status.CappedResources.Cpu().MilliValue(), tt.lowerBound)
				x, _ := json.Marshal(containerScale)
				klog.Info(string(x))
				require.LessOrEqual(t, containerScale.Status.CappedResources.Cpu().MilliValue(), tt.upperBound)
			}

		})
	}
}

func TestBounds(t *testing.T) {

	testcases := []struct {
		description string
		current     int64
		upperBound  int64
		lowerBound  int64
	}{
		{
			description: "applying lower bound",
			current:     15,
			upperBound:  1000,
			lowerBound:  100,
		},
		{
			description: "applying upper bound",
			current:     1200,
			upperBound:  1000,
			lowerBound:  100,
		},
		{
			description: "not applying any bounds",
			current:     500,
			upperBound:  1000,
			lowerBound:  100,
		},
	}

	for _, tt := range testcases {
		currentResource := resource.NewMilliQuantity(tt.current, resource.BinarySI)
		upperBoundResource := resource.NewMilliQuantity(tt.upperBound, resource.BinarySI)
		lowerBoundResource := resource.NewMilliQuantity(tt.lowerBound, resource.BinarySI)
		resultResource, bounded := applyBounds(currentResource, lowerBoundResource, upperBoundResource, true, true)
		require.GreaterOrEqual(t, resultResource.MilliValue(), lowerBoundResource.MilliValue())
		require.LessOrEqual(t, resultResource.MilliValue(), upperBoundResource.MilliValue())
		if tt.current <= tt.upperBound && tt.current >= tt.lowerBound {
			require.Equal(t, resultResource.MilliValue(), currentResource.MilliValue())
			require.False(t, bounded)
		} else {
			require.True(t, bounded)
		}
	}
}
