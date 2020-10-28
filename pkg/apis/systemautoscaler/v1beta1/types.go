package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO: decide whether to keep these or use `register.go`
// const (
// 	GroupName string = "systemautoscaler.polimi.it"
// 	Kind      string = "SystemAutoscaler"
// 	Version   string = "v1beta1"
// 	Plural    string = "systemautoscalers"
// 	Singluar  string = "systemautoscaler"
// 	ShortName string = "sysaut"
// 	Name      string = Plural + "." + GroupName
// )

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SystemAutoscaler is a specification for a SystemAutoscaler resource
type SystemAutoscaler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SystemAutoscalerSpec   `json:"spec"`
	Status SystemAutoscalerStatus `json:"status"`
}

// SystemAutoscalerSpec is the spec for a SystemAutoscaler resource
type SystemAutoscalerSpec struct {
	DesiredResources Resources `json:"desired"`
	ActualResources  Resources `json:"actual"`
	Replicas         *int32    `json:"replicas"`
}

// Resources is used to keep track the amount of resources assigned during the
// autoscaling process.
type Resources struct {
	CPU    *int32 `json:"cpu"`
	Memory *int32 `json:"memory"`
}

// TODO: Decide if useful or not

// SystemAutoscalerStatus is the status for a SystemAutoscaler resource
type SystemAutoscalerStatus struct {
	CurrentResources Resources `json:"current"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SystemAutoscalerList is a list of SystemAutoscaler resources
type SystemAutoscalerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []SystemAutoscaler `json:"items"`
}
