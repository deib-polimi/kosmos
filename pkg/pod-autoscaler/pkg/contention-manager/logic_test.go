package contentionmanager

import (
	"testing"

	"github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	"github.com/lterrac/system-autoscaler/pkg/containerscale-controller/pkg/types"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestProportional(t *testing.T) {

	testcases := []struct {
		description    string
		desired        int64
		desiredTotal   int64
		totalAvailable int64
		expected       int64
	}{
		{
			description:    "should get half of the resources",
			desired:        2,
			desiredTotal:   4,
			totalAvailable: 2,
			expected:       1,
		},
		{
			description:    "should get all the resources",
			desired:        2,
			desiredTotal:   2,
			totalAvailable: 1,
			expected:       1,
		},
		{
			description:    "should get no the resources",
			desired:        0,
			desiredTotal:   2,
			totalAvailable: 1,
			expected:       0,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {
			actual := proportional(tt.desired, tt.desiredTotal, tt.totalAvailable)
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestNewContentionManager(t *testing.T) {

	firstName := "foo"
	firstNamespace := "foo"
	secondName := "bar"
	secondNamespace := "bar"
	nodeName := "foobar"

	testcases := []struct {
		description string
		nodeScale   types.NodeScales
		node        *corev1.Node
		pods        []corev1.Pod
		solver      solverFn
		asserts     func(*testing.T, *ContentionManager, types.NodeScales, *corev1.Node, []corev1.Pod)
	}{
		{
			description: "should create the manager with the full node capacity allocatable",
			nodeScale: types.NodeScales{
				Node: nodeName,
			},
			pods: []corev1.Pod{},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeName,
				},
				Status: corev1.NodeStatus{
					Capacity: corev1.ResourceList{
						corev1.ResourceCPU:    *resource.NewScaledQuantity(100, resource.Milli),
						corev1.ResourceMemory: *resource.NewScaledQuantity(100, resource.Mega),
					},
				},
			},
			solver: proportional,
			asserts: func(t *testing.T, cm *ContentionManager, ns types.NodeScales, n *corev1.Node, p []corev1.Pod) {
				require.Equal(t, n.Status.Capacity.Cpu().MilliValue(), cm.CPUCapacity.MilliValue())
				require.Equal(t, n.Status.Capacity.Memory().MilliValue(), cm.MemoryCapacity.MilliValue())
			},
		},
		{
			description: "should not consume resources requested by external pods",
			nodeScale: types.NodeScales{
				Node: nodeName,
			},
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      firstName,
						Namespace: firstNamespace,
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    *resource.NewScaledQuantity(25, resource.Milli),
										corev1.ResourceMemory: *resource.NewScaledQuantity(25, resource.Mega),
									},
								},
							},
							{
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    *resource.NewScaledQuantity(25, resource.Milli),
										corev1.ResourceMemory: *resource.NewScaledQuantity(25, resource.Mega),
									},
								},
							},
						},
						NodeName: nodeName,
					},
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeName,
				},
				Status: corev1.NodeStatus{
					Capacity: corev1.ResourceList{
						corev1.ResourceCPU:    *resource.NewScaledQuantity(100, resource.Milli),
						corev1.ResourceMemory: *resource.NewScaledQuantity(100, resource.Mega),
					},
				},
			},
			asserts: func(t *testing.T, cm *ContentionManager, ns types.NodeScales, n *corev1.Node, p []corev1.Pod) {
				require.Equal(t, resource.NewScaledQuantity(50, resource.Milli).MilliValue(), cm.CPUCapacity.MilliValue())
				require.Equal(t, resource.NewScaledQuantity(50, resource.Mega).MilliValue(), cm.MemoryCapacity.MilliValue())
			},
		},
		{
			description: "should not have negative allocatable resources",
			nodeScale: types.NodeScales{
				Node: nodeName,
			},
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      firstName,
						Namespace: firstNamespace,
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    *resource.NewScaledQuantity(75, resource.Milli),
										corev1.ResourceMemory: *resource.NewScaledQuantity(75, resource.Mega),
									},
								},
							},
							{
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    *resource.NewScaledQuantity(75, resource.Milli),
										corev1.ResourceMemory: *resource.NewScaledQuantity(75, resource.Mega),
									},
								},
							},
						},
						NodeName: nodeName,
					},
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeName,
				},
				Status: corev1.NodeStatus{
					Capacity: corev1.ResourceList{
						corev1.ResourceCPU:    *resource.NewScaledQuantity(100, resource.Milli),
						corev1.ResourceMemory: *resource.NewScaledQuantity(100, resource.Mega),
					},
				},
			},
			asserts: func(t *testing.T, cm *ContentionManager, ns types.NodeScales, n *corev1.Node, p []corev1.Pod) {
				require.Nil(t, cm)
			},
		},
		{
			description: "should not consider pods with QOS classes not equal to guaranteed",
			nodeScale: types.NodeScales{
				Node: nodeName,
				ContainerScales: []*v1beta1.ContainerScale{
					{
						Spec: v1beta1.ContainerScaleSpec{
							PodRef: v1beta1.PodRef{
								Name:      firstName,
								Namespace: firstNamespace,
							},
						},
					},
					{
						Spec: v1beta1.ContainerScaleSpec{
							PodRef: v1beta1.PodRef{
								Name:      secondName,
								Namespace: secondNamespace,
							},
						},
					},
				},
			},
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secondName,
						Namespace: secondNamespace,
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    *resource.NewScaledQuantity(25, resource.Milli),
										corev1.ResourceMemory: *resource.NewScaledQuantity(25, resource.Mega),
									},
								},
							},
						},
						NodeName: nodeName,
					},
					Status: corev1.PodStatus{
						QOSClass: corev1.PodQOSBurstable,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      firstName,
						Namespace: firstNamespace,
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    *resource.NewScaledQuantity(25, resource.Milli),
										corev1.ResourceMemory: *resource.NewScaledQuantity(25, resource.Mega),
									},
								},
							},
						},
						NodeName: nodeName,
					},
					Status: corev1.PodStatus{
						QOSClass: corev1.PodQOSBestEffort,
					},
				},
			},
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeName,
				},
				Status: corev1.NodeStatus{
					Capacity: corev1.ResourceList{
						corev1.ResourceCPU:    *resource.NewScaledQuantity(100, resource.Milli),
						corev1.ResourceMemory: *resource.NewScaledQuantity(100, resource.Mega),
					},
				},
			},
			asserts: func(t *testing.T, cm *ContentionManager, ns types.NodeScales, n *corev1.Node, p []corev1.Pod) {
				require.Equal(t, resource.NewScaledQuantity(50, resource.Milli).MilliValue(), cm.CPUCapacity.MilliValue())
				require.Equal(t, resource.NewScaledQuantity(50, resource.Mega).MilliValue(), cm.MemoryCapacity.MilliValue())
			},
		},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {

			cm := NewContentionManager(tt.node, tt.nodeScale, tt.pods, tt.solver)
			tt.asserts(t, cm, tt.nodeScale, tt.node, tt.pods)
		})
	}
}

func TestSolve(t *testing.T) {

	testcases := []struct {
		description string
		ContentionManager
		expected []*v1beta1.ContainerScale
		asserts  func(*testing.T, []*v1beta1.ContainerScale, []*v1beta1.ContainerScale)
	}{
		{
			description: "should get the desired capped resources",
			ContentionManager: ContentionManager{
				solverFn:       proportional,
				CPUCapacity:    resource.NewScaledQuantity(100, resource.Milli),
				MemoryCapacity: resource.NewScaledQuantity(100, resource.Mega),
				ContainerScales: []*v1beta1.ContainerScale{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "",
							Namespace: "",
						},
						Spec: v1beta1.ContainerScaleSpec{
							DesiredResources: corev1.ResourceList{
								corev1.ResourceCPU:    *resource.NewScaledQuantity(50, resource.Milli),
								corev1.ResourceMemory: *resource.NewScaledQuantity(50, resource.Mega),
							},
						},
						Status: v1beta1.ContainerScaleStatus{
							ActualResources: corev1.ResourceList{
								corev1.ResourceCPU:    *resource.NewScaledQuantity(50, resource.Milli),
								corev1.ResourceMemory: *resource.NewScaledQuantity(50, resource.Mega),
							},
							CappedResources: corev1.ResourceList{
								corev1.ResourceCPU:    *resource.NewScaledQuantity(50, resource.Milli),
								corev1.ResourceMemory: *resource.NewScaledQuantity(50, resource.Mega),
							},
						},
					},
				},
			},
			expected: []*v1beta1.ContainerScale{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "",
						Namespace: "",
					},
					Spec: v1beta1.ContainerScaleSpec{
						DesiredResources: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewScaledQuantity(50, resource.Milli),
							corev1.ResourceMemory: *resource.NewScaledQuantity(50, resource.Mega),
						},
					},
					Status: v1beta1.ContainerScaleStatus{
						ActualResources: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewScaledQuantity(50, resource.Milli),
							corev1.ResourceMemory: *resource.NewScaledQuantity(50, resource.Mega),
						},
						CappedResources: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewScaledQuantity(50, resource.Milli),
							corev1.ResourceMemory: *resource.NewScaledQuantity(50, resource.Mega),
						},
					},
				},
			},
			asserts: func(t *testing.T, expected []*v1beta1.ContainerScale, actual []*v1beta1.ContainerScale) {
				for i := range expected {
					require.Equal(t, 0, expected[i].Status.ActualResources.Cpu().Cmp(*actual[i].Status.ActualResources.Cpu()))
					require.Equal(t, 0, expected[i].Status.ActualResources.Memory().Cmp(*actual[i].Status.ActualResources.Memory()))
				}
			},
		},
		{
			description: "should get the half of desired capped resources",
			ContentionManager: ContentionManager{
				solverFn:       proportional,
				CPUCapacity:    resource.NewScaledQuantity(100, resource.Milli),
				MemoryCapacity: resource.NewScaledQuantity(100, resource.Mega),
				ContainerScales: []*v1beta1.ContainerScale{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "",
							Namespace: "",
						},
						Spec: v1beta1.ContainerScaleSpec{
							DesiredResources: corev1.ResourceList{
								corev1.ResourceCPU:    *resource.NewScaledQuantity(100, resource.Milli),
								corev1.ResourceMemory: *resource.NewScaledQuantity(100, resource.Mega),
							},
						},
						Status: v1beta1.ContainerScaleStatus{
							ActualResources: corev1.ResourceList{
								corev1.ResourceCPU:    *resource.NewScaledQuantity(100, resource.Milli),
								corev1.ResourceMemory: *resource.NewScaledQuantity(100, resource.Mega),
							},
							CappedResources: corev1.ResourceList{
								corev1.ResourceCPU:    *resource.NewScaledQuantity(100, resource.Milli),
								corev1.ResourceMemory: *resource.NewScaledQuantity(100, resource.Mega),
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "",
							Namespace: "",
						},
						Spec: v1beta1.ContainerScaleSpec{
							DesiredResources: corev1.ResourceList{
								corev1.ResourceCPU:    *resource.NewScaledQuantity(100, resource.Milli),
								corev1.ResourceMemory: *resource.NewScaledQuantity(100, resource.Mega),
							},
						},
						Status: v1beta1.ContainerScaleStatus{
							ActualResources: corev1.ResourceList{
								corev1.ResourceCPU:    *resource.NewScaledQuantity(100, resource.Milli),
								corev1.ResourceMemory: *resource.NewScaledQuantity(100, resource.Mega),
							},
							CappedResources: corev1.ResourceList{
								corev1.ResourceCPU:    *resource.NewScaledQuantity(100, resource.Milli),
								corev1.ResourceMemory: *resource.NewScaledQuantity(100, resource.Mega),
							},
						},
					},
				},
			},
			expected: []*v1beta1.ContainerScale{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "",
						Namespace: "",
					},
					Spec: v1beta1.ContainerScaleSpec{
						DesiredResources: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewScaledQuantity(100, resource.Milli),
							corev1.ResourceMemory: *resource.NewScaledQuantity(100, resource.Mega),
						},
					},
					Status: v1beta1.ContainerScaleStatus{
						ActualResources: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewScaledQuantity(50, resource.Milli),
							corev1.ResourceMemory: *resource.NewScaledQuantity(50, resource.Mega),
						},
						CappedResources: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewScaledQuantity(100, resource.Milli),
							corev1.ResourceMemory: *resource.NewScaledQuantity(100, resource.Mega),
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "",
						Namespace: "",
					},
					Spec: v1beta1.ContainerScaleSpec{
						DesiredResources: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewScaledQuantity(100, resource.Milli),
							corev1.ResourceMemory: *resource.NewScaledQuantity(100, resource.Mega),
						},
					},
					Status: v1beta1.ContainerScaleStatus{
						ActualResources: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewScaledQuantity(50, resource.Milli),
							corev1.ResourceMemory: *resource.NewScaledQuantity(50, resource.Mega),
						},
						CappedResources: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewScaledQuantity(100, resource.Milli),
							corev1.ResourceMemory: *resource.NewScaledQuantity(100, resource.Mega),
						},
					},
				},
			},
			asserts: func(t *testing.T, expected []*v1beta1.ContainerScale, actual []*v1beta1.ContainerScale) {
				for i := range expected {
					require.Equal(t, 0, expected[i].Status.ActualResources.Cpu().Cmp(*actual[i].Status.ActualResources.Cpu()))
					require.Equal(t, 0, expected[i].Status.ActualResources.Memory().Cmp(*actual[i].Status.ActualResources.Memory()))
				}
			},
		},
	}
	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {
			containerscales := tt.ContentionManager.Solve()

			totalCPU := resource.Quantity{}
			totalMemory := resource.Quantity{}

			for _, p := range containerscales {
				totalCPU.Add(*p.Status.ActualResources.Cpu())
				totalMemory.Add(*p.Status.ActualResources.Memory())
			}

			require.GreaterOrEqual(t, tt.ContentionManager.CPUCapacity.MilliValue(), totalCPU.MilliValue())
			require.GreaterOrEqual(t, tt.ContentionManager.MemoryCapacity.MilliValue(), totalMemory.MilliValue())

			tt.asserts(t, tt.expected, containerscales)
		})
	}
}
