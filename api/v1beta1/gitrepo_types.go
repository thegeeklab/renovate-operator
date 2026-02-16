package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GitRepoSpec defines the desired state of GitRepo.
type GitRepoSpec struct {
	Name string `json:"name"`

	//+kubebuilder:validation:Optional
	ConfigRef string `json:"configRef,omitempty"`

	// +kubebuilder:validation:Optional
	WebhookID string `json:"webhookId,omitempty"`
}

// GitRepoStatus defines the observed state of GitRepo.
//
//nolint:lll
type GitRepoStatus struct {
	Ready      bool               `json:"ready"`
	Failed     int                `json:"failed,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	SpecHash   string             `json:"specHash,omitempty"`
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

func init() {
	SchemeBuilder.Register(&GitRepo{}, &GitRepoList{})
}
