package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// DiscoverySpec defines the desired state of Discovery.
type DiscoverySpec struct {
	ImageSpec `json:",inline"`

	// +kubebuilder:validation:Optional
	Logging *LoggingSpec `json:"logging,omitempty"`

	//+kubebuilder:validation:Optional
	ConfigRef string `json:"configRef,omitempty"`

	JobSpec `json:",inline"`

	// +kubebuilder:validation:Optional
	Filter []string `json:"filter,omitempty"`
}

// DiscoveryStatus defines the observed state of Discovery.
//
//nolint:lll
type DiscoveryStatus struct {
	Ready            bool               `json:"ready,omitempty"`
	Failed           int                `json:"failed,omitempty"`
	Conditions       []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	LastScheduleTime *metav1.Time       `json:"lastScheduleTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=discoveries

// Discovery is the Schema for the discoveries API.
type Discovery struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DiscoverySpec   `json:"spec,omitempty"`
	Status DiscoveryStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DiscoveryList contains a list of Discovery.
type DiscoveryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Discovery `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Discovery{}, &DiscoveryList{})
}

// GetSchedule returns the cron schedule string.
func (d *Discovery) GetSchedule() string {
	return d.Spec.Schedule
}

// GetSuspend returns true if the schedule is suspended.
func (d *Discovery) GetSuspend() bool {
	if d.Spec.Suspend == nil {
		return false
	}

	return *d.Spec.Suspend
}

// GetLastScheduleTime returns the time of the last execution.
func (d *Discovery) GetLastScheduleTime() *metav1.Time {
	return d.Status.LastScheduleTime
}

// SetLastScheduleTime updates the time of the last execution.
func (d *Discovery) SetLastScheduleTime(t *metav1.Time) {
	d.Status.LastScheduleTime = t
}

// GetSuccessLimit returns the history limit for successful jobs.
func (d *Discovery) GetSuccessLimit() int {
	return d.Spec.SuccessLimit
}

// GetFailedLimit returns the history limit for failed jobs.
func (d *Discovery) GetFailedLimit() int {
	return d.Spec.FailedLimit
}
