package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SchedulerSpec defines the desired state of Scheduler.
type SchedulerSpec struct {
	//+kubebuilder:validation:Optional
	ConfigRef string `json:"configRef"`

	ImageSpec `json:",inline"`

	// +kubebuilder:validation:Optional
	Logging *LoggingSpec `json:"logging,omitempty"`

	JobSpec `json:",inline"`

	// +kubebuilder:validation:Enum=none;batch
	// +kubebuilder:default:="none"
	Strategy SchedulerStrategy `json:"strategy,omitempty"`

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

// SchedulerStatus defines the observed state of Scheduler.
//
//nolint:lll
type SchedulerStatus struct {
	Ready      bool               `json:"ready"`
	Failed     int                `json:"failed,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Scheduler is the Schema for the schedulers API.
type Scheduler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SchedulerSpec   `json:"spec,omitempty"`
	Status SchedulerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SchedulerList contains a list of Scheduler.
type SchedulerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Scheduler `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Scheduler{}, &SchedulerList{})
}
