package v1beta1

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// JobPhase represents the phase of a RenovatorJob
// +kubebuilder:validation:Enum=Pending;Running;Succeeded;Failed
type JobPhase string

const (
	JobPhasePending   JobPhase = "Pending"
	JobPhaseRunning   JobPhase = "Running"
	JobPhaseSucceeded JobPhase = "Succeeded"
	JobPhaseFailed    JobPhase = "Failed"
)

// RenovatorJobSpec defines the desired state of RenovatorJob.
type RenovatorJobSpec struct {
	// RenovatorName is the name of the parent Renovator CR
	RenovatorName string `json:"renovatorName"`

	// Repositories is the list of repositories to process in this job
	Repositories []string `json:"repositories"`

	// JobSpec is the job specification to use for the runner
	JobSpec batchv1.JobSpec `json:"jobSpec"`

	// BatchID is an identifier for this batch of repositories
	// +optional
	BatchID string `json:"batchId,omitempty"`

	// Priority defines the priority of this job (higher values = higher priority)
	// +kubebuilder:default:=0
	// +optional
	Priority int32 `json:"priority,omitempty"`

	// TTLSecondsAfterFinished is the TTL for automatic deletion of finished jobs
	// +optional
	TTLSecondsAfterFinished *int32 `json:"ttlSecondsAfterFinished,omitempty"`
}

// RenovatorJobStatus defines the observed state of RenovatorJob.
type RenovatorJobStatus struct {
	// Phase represents the current phase of the job
	Phase JobPhase `json:"phase,omitempty"`

	// StartTime is when the job started processing
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is when the job completed processing
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// JobRef is a reference to the Kubernetes Job running this RenovatorJob
	// +optional
	JobRef *corev1.LocalObjectReference `json:"jobRef,omitempty"`

	// ProcessedRepositories tracks which repositories have been processed
	// +optional
	ProcessedRepositories []string `json:"processedRepositories,omitempty"`

	// FailedRepositories tracks which repositories failed to process
	// +optional
	FailedRepositories []string `json:"failedRepositories,omitempty"`

	// Conditions represent the latest available observations of the job's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Message provides additional information about the current status
	// +optional
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Repositories",type="integer",JSONPath=".spec.repositories[*]",priority=1
// +kubebuilder:printcolumn:name="Start Time",type="date",JSONPath=".status.startTime"
// +kubebuilder:printcolumn:name="Completion Time",type="date",JSONPath=".status.completionTime",priority=1
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// RenovatorJob is the Schema for the renovatorjobs API.
type RenovatorJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RenovatorJobSpec   `json:"spec,omitempty"`
	Status RenovatorJobStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RenovatorJobList contains a list of RenovatorJob
type RenovatorJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RenovatorJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RenovatorJob{}, &RenovatorJobList{})
}
