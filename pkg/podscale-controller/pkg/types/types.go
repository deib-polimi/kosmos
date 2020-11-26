package types

import "github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"

type NodeScales struct {
	Node      string
	PodScales []v1beta1.PodScale
}
