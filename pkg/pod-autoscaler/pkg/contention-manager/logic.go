package contentionmanager

import (
	"context"
	"fmt"

	"github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"

	"github.com/lterrac/system-autoscaler/pkg/podscale-controller/pkg/types"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

// solverFn is responsible of solving resource contentions on the node.
type solverFn func(desired, totalDesired, totalAvailable int64) int64

// proportional is the default policy to handle resource contentions.
// It divides the node resources based on the amount of resources requested
// with respect to the total amount and it adjust them according to
// the actual node capacity.
func proportional(desired, totalDesired, totalAvailable int64) int64 {
	quota := float64(desired) / float64(totalDesired)
	return int64(quota * float64(totalAvailable))
}

// processNextNode adjust the resources of all the pods scheduled on a node
func (c *Controller) processNextNode(podscalesInfos <-chan types.NodeScales) bool {

	for info := range podscalesInfos {

		node, err := c.nodeLister.Get(info.Node)

		if err != nil {
			utilruntime.HandleError(fmt.Errorf("error while getting node: %#v", err))
			return true
		}

		nodeResources := node.Status.Capacity

		desiredResourcesCPU := &resource.Quantity{}
		desiredResourcesMemory := &resource.Quantity{}

		for _, podscale := range info.PodScales {
			desiredResourcesCPU.Add(*podscale.Spec.DesiredResources.Cpu())
			desiredResourcesMemory.Add(*podscale.Spec.DesiredResources.Memory())
		}

		if desiredResourcesCPU.Value() > nodeResources.Cpu().Value() {
			err := solveCPUResourceContentions(info.PodScales, desiredResourcesCPU.Value(), nodeResources.Cpu().Value(), proportional)

			if err != nil {
				utilruntime.HandleError(fmt.Errorf("error while solving cpu contentions for node: %#v \n error: %#v", node.GetName(), err))
				return true
			}
		}

		if desiredResourcesMemory.Value() > nodeResources.Memory().Value() {
			err := solveMemoryResourceContentions(info.PodScales, desiredResourcesMemory.Value(), nodeResources.Memory().Value(), proportional)

			if err != nil {
				utilruntime.HandleError(fmt.Errorf("error while solving memory contentions for node: %#v \n error: %#v", node.GetName(), err))
				return true
			}
		}

		for _, podscale := range info.PodScales {
			_, err = c.podScalesClientset.SystemautoscalerV1beta1().PodScales(podscale.GetNamespace()).UpdateStatus(context.TODO(), podscale, metav1.UpdateOptions{})
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("error while updating podscale: %#v \n error: %#v", podscale, err))
				return true
			}
		}

	}

	return true
}

func solveCPUResourceContentions(podScales []*v1beta1.PodScale, desired int64, capacity int64, solver solverFn) error {
	for _, p := range podScales {
		p.Status.ActualResources.Cpu().SetScaled(
			solver(
				p.Spec.DesiredResources.Cpu().MilliValue(),
				desired,
				capacity,
			),
			resource.Milli,
		)
	}
	return nil
}

func solveMemoryResourceContentions(podScales []*v1beta1.PodScale, desired int64, capacity int64, solver solverFn) error {
	for _, p := range podScales {
		p.Status.ActualResources.Memory().SetScaled(
			solver(
				p.Spec.DesiredResources.Memory().MilliValue(),
				desired,
				capacity,
			),
			resource.Mega,
		)
	}
	return nil
}
