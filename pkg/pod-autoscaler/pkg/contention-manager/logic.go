package contentionmanager

import (
	"context"
	"fmt"

	"github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"

	"github.com/lterrac/system-autoscaler/pkg/podscale-controller/pkg/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

func (c *Controller) processNextPodscale(podscalesInfos <-chan types.NodeScales) bool {

	for info := range podscalesInfos {

		node, err := c.nodeLister.Get(info.Node)

		if err != nil {
			utilruntime.HandleError(fmt.Errorf("error while getting node: %#v", err))
			return true
		}

		nodeResources := node.Status.Capacity

		var desiredResourcesCPU *resource.Quantity
		var desiredResourcesMemory *resource.Quantity

		for _, podscale := range info.PodScales {
			desiredResourcesCPU.Add(*podscale.Spec.DesiredResources.Cpu())
			desiredResourcesMemory.Add(*podscale.Spec.DesiredResources.Memory())
		}

		if desiredResourcesCPU.Value() > nodeResources.Cpu().Value() {
			err := solveResourceContentions(corev1.ResourceCPU, info.PodScales, desiredResourcesCPU.Value(), nodeResources.Cpu().Value())

			if err != nil {
				utilruntime.HandleError(fmt.Errorf("error while solving cpu contentions for node: %#v \n error: %#v", node.GetName(), err))
				return true
			}
		}

		if desiredResourcesMemory.Value() > nodeResources.Memory().Value() {
			err := solveResourceContentions(corev1.ResourceMemory, info.PodScales, desiredResourcesMemory.Value(), nodeResources.Memory().Value())

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

func solveResourceContentions(resourceName corev1.ResourceName, podScales []*v1beta1.PodScale, desired int64, capacity int64) error {
	return nil
}
