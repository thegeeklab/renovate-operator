package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// GitRepoConditionRenovateRunning indicates whether a renovate job is currently running.
	GitRepoConditionRenovateRunning = "RenovateRunning"
	// GitRepoConditionRenovateCompleted indicates whether the last renovate job completed successfully.
	GitRepoConditionRenovateCompleted = "RenovateCompleted"
	// GitRepoConditionRenovateFailed indicates whether the last renovate job failed.
	GitRepoConditionRenovateFailed = "RenovateFailed"
)

// GitRepoSpec defines the desired state of GitRepo.
type GitRepoSpec struct {
	Name string `json:"name"`

	//+kubebuilder:validation:Optional
	ConfigRef string `json:"configRef,omitempty"`
}

// GitRepoStatus defines the observed state of GitRepo.
//
//nolint:lll
type GitRepoStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`

	// LastRenovateTime is the creation timestamp of the most recently completed
	// renovate job for this GitRepo.
	// This field is managed by the operator and should not be set manually.
	// +kubebuilder:validation:Optional
	LastRenovateTime *metav1.Time `json:"lastRenovateTime,omitempty"`

	// WebhookID is the ID of the webhook registered on the remote Git provider.
	// This field is managed by the operator and should not be set manually.
	// +kubebuilder:validation:Optional
	WebhookID string `json:"webhookId,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=gitrepos

// GitRepo is the Schema for the gitrepos API.
type GitRepo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GitRepoSpec   `json:"spec,omitempty"`
	Status GitRepoStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GitRepoList contains a list of GitRepo.
type GitRepoList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GitRepo `json:"items"`
}

func (g *GitRepo) GetLastRenovateTime() *metav1.Time {
	return g.Status.LastRenovateTime
}

func (g *GitRepo) SetLastRenovateTime(t *metav1.Time) {
	g.Status.LastRenovateTime = t
}

func (g *GitRepo) SetCondition(
	conditionType string,
	status metav1.ConditionStatus,
	reason, message string,
	now metav1.Time,
) {
	for i, cond := range g.Status.Conditions {
		if cond.Type == conditionType {
			if cond.Status != status {
				g.Status.Conditions[i].Status = status
				g.Status.Conditions[i].LastTransitionTime = now
			}

			g.Status.Conditions[i].Reason = reason
			g.Status.Conditions[i].Message = message

			return
		}
	}

	g.Status.Conditions = append(g.Status.Conditions, metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
		ObservedGeneration: g.Generation,
	})
}

func (g *GitRepo) GetCondition(conditionType string) *metav1.Condition {
	for i := range g.Status.Conditions {
		if g.Status.Conditions[i].Type == conditionType {
			return &g.Status.Conditions[i]
		}
	}

	return nil
}

func (g *GitRepo) RemoveCondition(conditionType string) {
	var newConditions []metav1.Condition

	for _, cond := range g.Status.Conditions {
		if cond.Type != conditionType {
			newConditions = append(newConditions, cond)
		}
	}

	g.Status.Conditions = newConditions
}
