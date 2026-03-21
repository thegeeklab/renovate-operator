package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RunnerSpec defines the desired state of Runner.
type RunnerSpec struct {
	ImageSpec `json:",inline"`

	// +kubebuilder:validation:Optional
	Logging *LoggingSpec `json:"logging,omitempty"`

	//+kubebuilder:validation:Optional
	ConfigRef string `json:"configRef,omitempty"`

	JobSpec `json:",inline"`
}

// RunnerStatus defines the observed state of Runner.
//
//nolint:lll
type RunnerStatus struct {
	Conditions       []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	LastScheduleTime *metav1.Time       `json:"lastScheduleTime,omitempty"`
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

// GetSchedule returns the cron schedule string.
func (r *Runner) GetSchedule() string {
	return r.Spec.Schedule
}

// GetSuspend returns true if the schedule is suspended.
func (r *Runner) GetSuspend() bool {
	if r.Spec.Suspend == nil {
		return false
	}

	return *r.Spec.Suspend
}

// GetLastScheduleTime returns the time of the last execution.
func (r *Runner) GetLastScheduleTime() *metav1.Time {
	return r.Status.LastScheduleTime
}

// SetLastScheduleTime updates the time of the last execution.
func (r *Runner) SetLastScheduleTime(t *metav1.Time) {
	r.Status.LastScheduleTime = t
}

// GetSuccessLimit returns the history limit for successful jobs.
func (r *Runner) GetSuccessLimit() int {
	return int(*r.Spec.SuccessLimit)
}

// GetFailedLimit returns the history limit for failed jobs.
func (r *Runner) GetFailedLimit() int {
	return int(*r.Spec.FailedLimit)
}
