package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// DiscoverySpec defines the desired state of Discovery.
type DiscoverySpec struct {
	ImageSpec `json:",inline"`

	// +kubebuilder:validation:Optional
	Logging *LoggingSpec `json:"logging,omitempty"`

	JobSpec `json:",inline"`

	Filter []string `json:"filter,omitempty"`
}

// DiscoveryStatus defines the observed state of Discovery.
//
//nolint:lll
type DiscoveryStatus struct {
	Ready      bool               `json:"ready"`
	Failed     int                `json:"failed,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
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
