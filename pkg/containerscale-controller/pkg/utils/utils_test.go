package utils

import (
	"testing"

	"github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDiffPods(t *testing.T) {
	pods := []*corev1.Pod{
		{
			ObjectMeta: v1.ObjectMeta{
				Name: "foo",
			},
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Name: "bar",
			},
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Name: "foobar",
			},
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Name: "foobarfoo",
			},
		},
	}

	containerscales := []*v1beta1.ContainerScale{
		{
			Spec: v1beta1.ContainerScaleSpec{
				PodRef: v1beta1.PodRef{
					Name:      "foo",
					Namespace: "default",
				},
			},
		},
		{
			Spec: v1beta1.ContainerScaleSpec{
				PodRef: v1beta1.PodRef{
					Name:      "bar",
					Namespace: "default",
				},
			},
		},
		{
			Spec: v1beta1.ContainerScaleSpec{
				PodRef: v1beta1.PodRef{
					Name:      "foobar",
					Namespace: "default",
				},
			},
		},
		{
			Spec: v1beta1.ContainerScaleSpec{
				PodRef: v1beta1.PodRef{
					Name:      "foobarfoo",
					Namespace: "default",
				},
			},
		},
	}

	testcases := []struct {
		description     string
		pods            []*corev1.Pod
		containerscales []*v1beta1.ContainerScale
		expected        StateDiff
	}{
		{
			description:     "add all pods if there are no containerscales",
			pods:            pods,
			containerscales: []*v1beta1.ContainerScale{},
			expected: StateDiff{
				AddList: pods,
			},
		},
		{
			description:     "add only pods without the corresponding containerscales",
			pods:            pods,
			containerscales: containerscales[2:],
			expected: StateDiff{
				AddList: pods[:2],
			},
		},
		{
			description:     "delete containerscales if there are no pods",
			pods:            []*corev1.Pod{},
			containerscales: containerscales,
			expected: StateDiff{
				DeleteList: containerscales,
			},
		},
		{
			description:     "delete containerscales if the corresponding pod no longer exists",
			pods:            pods[2:],
			containerscales: containerscales,
			expected: StateDiff{
				DeleteList: containerscales[:2],
			},
		},
		{
			description:     "statediff should be empty if pod and containerscales coincide",
			pods:            pods,
			containerscales: containerscales,
			expected:        StateDiff{},
		},
		{
			description:     "miscellanea test",
			pods:            pods[1:],
			containerscales: containerscales[:3],
			expected: StateDiff{
				AddList:    pods[3:],
				DeleteList: containerscales[:1],
			},
		},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {
			actual := DiffPods(tt.pods, tt.containerscales)
			require.Equal(t, tt.expected, actual, "StateDiff should coincide")
		})
	}
}

func TestContainsService(t *testing.T) {
	services := []*corev1.Service{
		{
			ObjectMeta: v1.ObjectMeta{
				Name: "foo",
			},
		},
	}

	testcases := []struct {
		description string
		services    []*corev1.Service
		element     *corev1.Service
		expected    bool
	}{
		{
			description: "elemnt contained in services",
			services:    services,
			element:     services[0],
			expected:    true,
		},
		{
			description: "elemnt contained in services",
			services:    services,
			element: &corev1.Service{
				ObjectMeta: v1.ObjectMeta{
					Name: "notexists",
				},
			},
			expected: false,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {
			actual := ContainsService(tt.services, tt.element)
			require.Equal(t, tt.expected, actual)
		})
	}
}
