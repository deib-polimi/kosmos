package resourceupdater

import (
	"fmt"
	"testing"

	"github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSyncPod(t *testing.T) {

	// TODO: The test case should be modified in future in order to handle more granularity.
	// Instead of pod resource values, we should insert cpu and mem values for each container.
	testcases := []struct {
		description            string
		podQOS                 v1.PodQOSClass
		podNumberOfContainers  int64
		podCPUValue            int64
		podMemValue            int64
		podScaleCPUActualValue int64
		podScaleMemActualValue int64
		success                bool
	}{
		{
			description:            "successfully increased the resources of a pod",
			podQOS:                 v1.PodQOSGuaranteed,
			podNumberOfContainers:  1,
			podCPUValue:            100,
			podMemValue:            100,
			podScaleCPUActualValue: 1000,
			podScaleMemActualValue: 1000,
			success:                true,
		},
		{
			description:            "successfully decreased the resources of a pod",
			podQOS:                 v1.PodQOSGuaranteed,
			podNumberOfContainers:  1,
			podCPUValue:            100,
			podMemValue:            100,
			podScaleCPUActualValue: 1000,
			podScaleMemActualValue: 1000,
			success:                true,
		},
		{
			description:            "fail to update a pod with negative cpu resource value",
			podQOS:                 v1.PodQOSGuaranteed,
			podNumberOfContainers:  1,
			podCPUValue:            100,
			podMemValue:            100,
			podScaleCPUActualValue: -1,
			podScaleMemActualValue: 1000,
			success:                false,
		},
		{
			description:            "fail to update a pod with negative memory resource value",
			podQOS:                 v1.PodQOSGuaranteed,
			podNumberOfContainers:  1,
			podCPUValue:            100,
			podMemValue:            100,
			podScaleCPUActualValue: 1000,
			podScaleMemActualValue: -1,
			success:                false,
		},
		{
			description:            "fail to update a pod that has BE QOS",
			podQOS:                 v1.PodQOSBestEffort,
			podNumberOfContainers:  1,
			podCPUValue:            100,
			podMemValue:            100,
			podScaleCPUActualValue: 1000,
			podScaleMemActualValue: 1000,
			success:                false,
		},
		{
			description:            "fail to update a pod that has BU QOS",
			podQOS:                 v1.PodQOSBurstable,
			podNumberOfContainers:  1,
			podCPUValue:            100,
			podMemValue:            100,
			podScaleCPUActualValue: 1000,
			podScaleMemActualValue: 1000,
			success:                false,
		},
		//{
		//	// TODO: this test should be changed once we are able to update multiple containers
		//	description:            "fail to update a pod that has multiple containers",
		//	podQOS:                 v1.PodQOSGuaranteed,
		//	podNumberOfContainers:  2,
		//	podCPUValue:            100,
		//	podMemValue:            100,
		//	podScaleCPUActualValue: 1000,
		//	podScaleMemActualValue: 1000,
		//	success:                false,
		//},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {
			// Instantiate the containers
			containers := make([]v1.Container, 0)
			for i := 0; i < int(tt.podNumberOfContainers); i++ {
				container := v1.Container{
					Name:  fmt.Sprint("container-n-", i),
					Image: "gcr.io/distroless/static:nonroot",
					Resources: v1.ResourceRequirements{
						Limits: v1.ResourceList{
							v1.ResourceCPU:    *resource.NewScaledQuantity(tt.podCPUValue, resource.Milli),
							v1.ResourceMemory: *resource.NewScaledQuantity(tt.podMemValue, resource.Mega),
						},
						Requests: v1.ResourceList{
							v1.ResourceCPU:    *resource.NewScaledQuantity(tt.podCPUValue, resource.Milli),
							v1.ResourceMemory: *resource.NewScaledQuantity(tt.podMemValue, resource.Mega),
						},
					},
				}
				containers = append(containers, container)
			}
			// Instantiate the pod
			pod := v1.Pod{
				TypeMeta: metav1.TypeMeta{
					APIVersion: v1.SchemeGroupVersion.String(),
					Kind:       "pods",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-name",
					Namespace: "default",
				},
				Spec: v1.PodSpec{
					Containers: containers,
				},
				Status: v1.PodStatus{
					QOSClass: tt.podQOS,
				},
			}
			// Instantiate the pod scale
			podScale := v1beta1.PodScale{
				TypeMeta: metav1.TypeMeta{
					Kind:       "podscales",
					APIVersion: v1beta1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "podscale-name",
					Namespace: "default",
				},
				Spec: v1beta1.PodScaleSpec{
					Pod:       "pod-name",
					Namespace: "default",
					DesiredResources: v1.ResourceList{
						v1.ResourceCPU:    *resource.NewScaledQuantity(tt.podScaleCPUActualValue, resource.Milli),
						v1.ResourceMemory: *resource.NewScaledQuantity(tt.podScaleMemActualValue, resource.Mega),
					},
					Container: "container-n-0",
				},
				Status: v1beta1.PodScaleStatus{
					ActualResources: v1.ResourceList{
						v1.ResourceCPU:    *resource.NewScaledQuantity(tt.podScaleCPUActualValue, resource.Milli),
						v1.ResourceMemory: *resource.NewScaledQuantity(tt.podScaleMemActualValue, resource.Mega),
					},
				},
			}
			newPod, err := syncPod(&pod, podScale)
			if tt.success {
				require.Nil(t, err, "Do not expect error")
				require.Equal(t, newPod.Spec.Containers[0].Resources.Limits.Cpu().ScaledValue(resource.Milli), tt.podScaleCPUActualValue)
				require.Equal(t, newPod.Spec.Containers[0].Resources.Requests.Cpu().ScaledValue(resource.Milli), tt.podScaleCPUActualValue)
				require.Equal(t, newPod.Spec.Containers[0].Resources.Limits.Memory().ScaledValue(resource.Mega), tt.podScaleMemActualValue)
				require.Equal(t, newPod.Spec.Containers[0].Resources.Requests.Memory().ScaledValue(resource.Mega), tt.podScaleMemActualValue)
				require.Equal(t, newPod.Status.QOSClass, v1.PodQOSGuaranteed)
			} else {
				require.Error(t, err, "expected error")
			}
		})
	}
}
