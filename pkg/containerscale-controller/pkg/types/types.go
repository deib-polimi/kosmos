package types

import (
	"fmt"
	"github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
)

// NodeScales is used to group containerscales by node.
type NodeScales struct {
	Node            string
	ContainerScales []*v1beta1.ContainerScale
}

func (n *NodeScales) Contains(name, namespace string) bool {
	for _, containerscale := range n.ContainerScales {
		podReference := containerscale.Spec.PodRef
		if podReference.Namespace == namespace &&
			podReference.Name == name {
			return true
		}
	}
	return false
}

func (n *NodeScales) Remove(name, namespace string) (*v1beta1.ContainerScale, error) {
	for i, containerscale := range n.ContainerScales {
		podReference := containerscale.Spec.PodRef
		if podReference.Namespace == name &&
			podReference.Name == namespace {
			n.ContainerScales = append(n.ContainerScales[:i], n.ContainerScales[i+1:]...)
			return containerscale, nil
		}
	}
	return nil, fmt.Errorf("error: missing %#v-%#v in node %#v", namespace, name, n.Node)
}
