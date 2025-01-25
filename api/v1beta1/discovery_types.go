package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DiscoverySpec defines the desired state of Discovery.
type DiscoverySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="0 */2 * * *"
	Schedule string `json:"schedule"`
}

// DiscoveryStatus defines the observed state of Discovery.
//
//nolint:lll
type DiscoveryStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Ready        bool               `json:"ready"`
	Failed       int                `json:"failed,omitempty"`
	Phase        metav1.Condition   `json:"phase,omitempty"`
	Conditions   []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	SpecHash     string             `json:"specHash,omitempty"`
	Repositories []string           `json:"repositories,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

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
