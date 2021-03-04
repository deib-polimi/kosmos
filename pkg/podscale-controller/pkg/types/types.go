package types

import (
	"fmt"

	"github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
)

// NodeScales is used to group podscales by node.
type NodeScales struct {
	Node      string
	PodScales []*v1beta1.PodScale
}

func (n *NodeScales) Contains(name, namespace string) bool {
	for _, podscale := range n.PodScales {
		if podscale.Spec.Namespace == namespace &&
			podscale.Spec.Pod == name {
			return true
		}
	}
	return false
}

func (n *NodeScales) Remove(name, namespace string) (*v1beta1.PodScale, error) {
	for i, podscale := range n.PodScales {
		if podscale.Spec.Namespace == namespace &&
			podscale.Spec.Pod == name {
			n.PodScales = append(n.PodScales[:i], n.PodScales[i+1:]...)
			return podscale, nil
		}
	}
	return nil, fmt.Errorf("error: missing %#v-%#v in node %#v", namespace, name, n.Node)
}
