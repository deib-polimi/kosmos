package contentionmanager

import (
	"context"
	"fmt"
	"k8s.io/klog/v2"

	"github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	"github.com/lterrac/system-autoscaler/pkg/podscale-controller/pkg/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

// TODO wrap it into a structure/interface to enable multiple solver policies
// solverFn solves resource contentions on the node.
type solverFn func(desired, totalDesired, totalAvailable int64) int64

// proportional is the default policy to handle resource contentions.
// It divides the node resources based on the amount of resources requested
// with respect to the total amount and it adjust them according to
// the actual node capacity.
func proportional(desired, totalDesired, totalAvailable int64) int64 {
	quota := float64(desired) / float64(totalDesired)
	return int64(quota * float64(totalAvailable))
}

// ContentionManager embeds the contention resolution logic on a given Node.
type ContentionManager struct {
	solverFn
	CPUCapacity    *resource.Quantity
	MemoryCapacity *resource.Quantity
	PodScales      []*v1beta1.PodScale
}

// NewContentionManager returns a new ContentionManager instance
func NewContentionManager(n *corev1.Node, ns types.NodeScales, p []corev1.Pod, solver solverFn) *ContentionManager {
	// exclude from the computation the resources allocated to pod not tracked by System Autoscaler
	var err error
	untrackedCPU := &resource.Quantity{}
	untrackedMemory := &resource.Quantity{}

	for _, pod := range p {
		if !ns.Contains(pod.Name, pod.Namespace) {
			for _, c := range pod.Spec.Containers {
				untrackedCPU.Add(*c.Resources.Requests.Cpu())
				untrackedMemory.Add(*c.Resources.Requests.Memory())
			}
		}

		// This must never happen. In place resource update could not work properly if requests and
		// limits do not coincide.
		// TODO remove this when webhook server is implemented
		// TODO Discuss about QOS behaviour for external pods
		if ns.Contains(pod.Name, pod.Namespace) && pod.Status.QOSClass != corev1.PodQOSGuaranteed {
			_, err = ns.Remove(pod.Name, pod.Namespace)
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("error while creating the contention manager: %#v", err))
				return nil
			}
			for _, c := range pod.Spec.Containers {
				untrackedCPU.Add(*c.Resources.Requests.Cpu())
				untrackedMemory.Add(*c.Resources.Requests.Memory())
			}
		}
	}

	allocatableCPU := n.Status.Capacity.Cpu()
	klog.Info("Node total capacity is:", allocatableCPU)
	untrackedCPU.Neg()
	klog.Info("Pod desired resources is:", untrackedCPU)
	allocatableCPU.Add(*untrackedCPU)
	klog.Info("Node allocatable capacity is:", allocatableCPU)

	allocatableMemory := n.Status.Capacity.Memory()
	untrackedMemory.Neg()
	allocatableMemory.Add(*untrackedMemory)

	if allocatableCPU.Sign() < 0 || allocatableMemory.Sign() < 0 {
		utilruntime.HandleError(fmt.Errorf("error while creating the contention manager: allocatable resources shouldn't be negative. CPU: %#v Memory: %#v", allocatableCPU.MilliValue(), allocatableMemory.MilliValue()))
		return nil
	}

	return &ContentionManager{
		solverFn:       solver,
		CPUCapacity:    allocatableCPU,
		MemoryCapacity: allocatableMemory,
		PodScales:      ns.PodScales,
	}
}

// Solve resolves the contentions between the podscales
func (m *ContentionManager) Solve() []*v1beta1.PodScale {
	desiredCPU := &resource.Quantity{}
	desiredMemory := &resource.Quantity{}

	for _, podscale := range m.PodScales {
		desiredCPU.Add(*podscale.Spec.DesiredResources.Cpu())
		desiredMemory.Add(*podscale.Spec.DesiredResources.Memory())
	}

	var actualCPU *resource.Quantity
	var actualMemory *resource.Quantity

	for _, p := range m.PodScales {

		if desiredCPU.Cmp(*m.CPUCapacity) == 1 {
			actualCPU = resource.NewScaledQuantity(m.solverFn(
				p.Spec.DesiredResources.Cpu().MilliValue(),
				desiredCPU.MilliValue(),
				m.CPUCapacity.MilliValue(),
			),
				resource.Milli,
			)
		} else {
			actualCPU = resource.NewScaledQuantity(
				p.Spec.DesiredResources.Cpu().MilliValue(), resource.Milli,
			)
		}

		if desiredMemory.Cmp(*m.MemoryCapacity) == 1 {
			actualMemory = resource.NewScaledQuantity(
				m.solverFn(
					p.Spec.DesiredResources.Memory().MilliValue(),
					desiredMemory.MilliValue(),
					m.MemoryCapacity.MilliValue(),
				),
				resource.Milli,
			)
		} else {
			actualMemory = resource.NewScaledQuantity(
				p.Spec.DesiredResources.Memory().MilliValue(),
				resource.Milli,
			)
		}

		p.Status.ActualResources = corev1.ResourceList{
			corev1.ResourceCPU:    *actualCPU,
			corev1.ResourceMemory: *actualMemory,
		}
	}

	return m.PodScales
}

// processNextNode adjust the resources of all the pods scheduled on a node
// according to the actual capacity. Resources not tracked by System Autoscaler
// are not considered.
func (c *Controller) processNextNode(podscalesInfos <-chan types.NodeScales) bool {
	for podscalesInfo := range podscalesInfos {

		node, err := c.listers.NodeLister.Get(podscalesInfo.Node)

		if err != nil {
			utilruntime.HandleError(fmt.Errorf("error while getting node: %#v", err))
			return true
		}

		pods, err := c.kubeClientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{
			FieldSelector: fields.SelectorFromSet(map[string]string{
				"spec.nodeName": node.Name,
			}).String(),
		})


		if err != nil {
			utilruntime.HandleError(fmt.Errorf("error while getting node pods: %#v", err))
			return true
		}

		for _, pod := range pods.Items {
			klog.Info("Node: ", node.Name, ",pod: ", pod.Name)
		}

		cm := NewContentionManager(node, podscalesInfo, pods.Items, proportional)

		nodeScale := cm.Solve()

		podscalesInfo.PodScales = nodeScale

		c.out <- podscalesInfo
	}

	return true
}
