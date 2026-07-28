package v1beta1

import (
	api_meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// DiscoveryConditionDiscoveryRunning indicates whether a discovery job is currently running.
	DiscoveryConditionDiscoveryRunning = "DiscoveryRunning"
	// DiscoveryConditionDiscoveryCompleted indicates whether the last discovery job completed successfully.
	DiscoveryConditionDiscoveryCompleted = "DiscoveryCompleted"
	// DiscoveryConditionDiscoveryFailed indicates whether the last discovery job failed.
	DiscoveryConditionDiscoveryFailed = "DiscoveryFailed"
)

// DiscoverySpec defines the desired state of Discovery.
type DiscoverySpec struct {
	ImageSpec `json:",inline"`

	// +kubebuilder:validation:Optional
	Logging *LoggingSpec `json:"logging,omitempty"`

	//+kubebuilder:validation:Optional
	ConfigRef string `json:"configRef,omitempty"`

	JobSpec `json:",inline"`

	PodSpec `json:",inline"`

	// +kubebuilder:validation:Optional
	Filter []string `json:"filter,omitempty"`

	// SkipForks excludes forked repositories from autodiscovery when set to true.
	// +kubebuilder:validation:Optional
	SkipForks *bool `json:"skipForks,omitempty"`

	// Topics filters autodiscovery to repositories matching all specified topics.
	// +kubebuilder:validation:Optional
	Topics []string `json:"topics,omitempty"`

	// Webhooks configures webhook management for the repositories discovered
	// by this Discovery. Propagated to the child GitRepo resources.
	// +kubebuilder:validation:Optional
	Webhooks WebhooksSpec `json:"webhooks,omitempty"`
}

// DiscoveryStatus defines the observed state of Discovery.
//
//nolint:lll
type DiscoveryStatus struct {
	Conditions           []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	LastScheduleTime     *metav1.Time       `json:"lastScheduleTime,omitempty"`
	LastDiscoveryTime    *metav1.Time       `json:"lastDiscoveryTime,omitempty"`
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

// GetSchedule returns the cron schedule string.
func (d *Discovery) GetSchedule() string {
	return d.Spec.Schedule
}

// GetTimezone returns the IANA timezone name for schedule evaluation.
func (d *Discovery) GetTimezone() string {
	return d.Spec.Timezone
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

// GetLastDiscoveryTime returns the creation timestamp of the most recently
// completed discovery job for which metrics have been emitted.
func (d *Discovery) GetLastDiscoveryTime() *metav1.Time {
	return d.Status.LastDiscoveryTime
}

// SetLastDiscoveryTime updates the creation timestamp of the most recently
// completed discovery job for which metrics have been emitted.
func (d *Discovery) SetLastDiscoveryTime(t *metav1.Time) {
	d.Status.LastDiscoveryTime = t
}

// GetSuccessLimit returns the history limit for successful jobs.
func (d *Discovery) GetSuccessLimit() int {
	return int(*d.Spec.SuccessLimit)
}

// GetFailedLimit returns the history limit for failed jobs.
func (d *Discovery) GetFailedLimit() int {
	return int(*d.Spec.FailedLimit)
}

// GetSkipForks returns true if forked repositories should be excluded from autodiscovery.
func (d *Discovery) GetSkipForks() bool {
	if d.Spec.SkipForks == nil {
		return false
	}

	return *d.Spec.SkipForks
}

// GetTopics returns the list of topics to filter repositories by.
func (d *Discovery) GetTopics() []string {
	return d.Spec.Topics
}

func (d *Discovery) SetCondition(
	conditionType string,
	status metav1.ConditionStatus,
	reason, message string,
) {
	api_meta.SetStatusCondition(&d.Status.Conditions, metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: d.Generation,
	})
}

func (d *Discovery) GetCondition(conditionType string) *metav1.Condition {
	return api_meta.FindStatusCondition(d.Status.Conditions, conditionType)
}

func (d *Discovery) RemoveCondition(conditionType string) {
	api_meta.RemoveStatusCondition(&d.Status.Conditions, conditionType)
}
