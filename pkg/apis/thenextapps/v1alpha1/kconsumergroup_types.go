package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KconsumerGroupSpec defines the desired state of KconsumerGroup
type KconsumerGroupSpec struct {
	MinReplicas  int32        `json:"minReplicas"`
	Autoscaling  bool         `json:"autoscaling"`
	ConsumerSpec ConsumerSpec `json:"consumerSpec"`
}

// ConsumerSpec defines the consumer's attributes
type ConsumerSpec struct {
	PodName    string `json:"containerName"`
	Image      string `json:"image"`
	Topic      string `json:"topic"`
	Throughput int32  `json:"throughput"`
}

// KconsumerGroupStatus defines the observed state of KconsumerGroup
type KconsumerGroupStatus struct {
	Replicas int32    `json:"replicas"`
	Nodes    []string `json:"nodes"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KconsumerGroup is the Schema for the kconsumergroups API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=kconsumergroups,scope=Namespaced
type KconsumerGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KconsumerGroupSpec   `json:"spec,omitempty"`
	Status KconsumerGroupStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KconsumerGroupList contains a list of KconsumerGroup
type KconsumerGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KconsumerGroup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KconsumerGroup{}, &KconsumerGroupList{})
}
