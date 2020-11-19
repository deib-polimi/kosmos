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

	podscales := []*v1beta1.PodScale{
		{
			Spec: v1beta1.PodScaleSpec{
				PodRef: v1beta1.PodRef{
					Name: "foo",
					Namespace: "default",
				},
			},
		},
		{
			Spec: v1beta1.PodScaleSpec{
				PodRef: v1beta1.PodRef{
					Name: "bar",
					Namespace: "default",
				},
			},
		},
		{
			Spec: v1beta1.PodScaleSpec{
				PodRef: v1beta1.PodRef{
					Name: "foobar",
					Namespace: "default",
				},
			},
		},
		{
			Spec: v1beta1.PodScaleSpec{
				PodRef: v1beta1.PodRef{
					Name: "foobarfoo",
					Namespace: "default",
				},
			},
		},
	}

	testcases := []struct {
		description string
		pods        []*corev1.Pod
		podscales   []*v1beta1.PodScale
		expected    StateDiff
	}{
		{
			description: "add all pods if there are no podscales",
			pods:        pods,
			podscales:   []*v1beta1.PodScale{},
			expected: StateDiff{
				AddList: pods,
			},
		},
		{
			description: "add only pods without the corresponding podscales",
			pods:        pods,
			podscales:   podscales[2:],
			expected: StateDiff{
				AddList: pods[:2],
			},
		},
		{
			description: "delete podscales if there are no pods",
			pods:        []*corev1.Pod{},
			podscales:   podscales,
			expected: StateDiff{
				DeleteList: podscales,
			},
		},
		{
			description: "delete podscales if the corresponding pod no longer exists",
			pods:        pods[2:],
			podscales:   podscales,
			expected: StateDiff{
				DeleteList: podscales[:2],
			},
		},
		{
			description: "statediff should be empty if pod and podscales coincide",
			pods:        pods,
			podscales:   podscales,
			expected:    StateDiff{},
		},
		{
			description: "miscellanea test",
			pods:        pods[1:],
			podscales:   podscales[:3],
			expected: StateDiff{
				AddList:    pods[3:],
				DeleteList: podscales[:1],
			},
		},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {
			actual := DiffPods(tt.pods, tt.podscales)
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
