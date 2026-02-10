package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RenovateConfigSpec defines the desired state of RenovateConfig.
type RenovateConfigSpec struct {
	ImageSpec `json:",inline"`

	// +kubebuilder:validation:Optional
	Logging *LoggingSpec `json:"logging,omitempty"`

	Platform PlatformSpec `json:"platform"`
	// +kubebuilder:validation:Optional
	DryRun DryRun `json:"dryRun,omitempty"`
	// +kubebuilder:validation:Optional
	Onboarding *bool `json:"onboarding,omitempty"`
	// OnBoardingConfig object `json:"onBoardingConfig,omitempty,inline"`
	// +kubebuilder:validation:Optional
	PrHourlyLimit int `json:"prHourlyLimit,omitempty"`
	// +kubebuilder:validation:Optional
	AddLabels []string `json:"addLabels,omitempty"`

	// +kubebuilder:validation:Optional
	GithubToken *corev1.EnvVarSource `json:"githubToken,omitempty"`
}

// RenovateConfigStatus defines the observed state of RenovateConfig.
//
//nolint:lll
type RenovateConfigStatus struct {
	Ready      bool               `json:"ready"`
	Failed     int                `json:"failed,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// RenovateConfig is the Schema for the renovateconfigs API.
type RenovateConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RenovateConfigSpec   `json:"spec,omitempty"`
	Status RenovateConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RenovateConfigList contains a list of RenovateConfig.
type RenovateConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RenovateConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RenovateConfig{}, &RenovateConfigList{})
}
