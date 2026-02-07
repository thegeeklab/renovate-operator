package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RunnerSpec defines the desired state of Runner.
type RunnerSpec struct {
	//+kubebuilder:validation:Optional
	ConfigRef string `json:"configRef"`

	ImageSpec `json:",inline"`

	// +kubebuilder:validation:Optional
	Logging *LoggingSpec `json:"logging,omitempty"`

	JobSpec `json:",inline"`

	// +kubebuilder:validation:Enum=none;batch
	// +kubebuilder:default:="none"
	Strategy RunnerStrategy `json:"strategy,omitempty"`

	// Maximum number of parallel pods to start. One instance will only process a single batch.
	// +kubebuilder:default:=1
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	Instances int32 `json:"instances"`

	// Number of repositories per batch. Only used when strategy is 'batch'.
	// If not specified, defaults to a reasonable size based on the number of repositories and instances.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1000
	BatchSize int `json:"batchSize,omitempty"`
}

// RunnerStatus defines the observed state of Runner.
//
//nolint:lll
type RunnerStatus struct {
	Ready      bool               `json:"ready"`
	Failed     int                `json:"failed,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Runner is the Schema for the runners API.
type Runner struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RunnerSpec   `json:"spec,omitempty"`
	Status RunnerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RunnerList contains a list of Runner.
type RunnerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Runner `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Runner{}, &RunnerList{})
}
