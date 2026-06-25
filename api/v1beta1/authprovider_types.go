package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	api_meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AuthProviderSpec defines the desired state of AuthProvider.
type AuthProviderSpec struct {
	// Type is the platform type.
	Type PlatformType `json:"type"`

	// Endpoint is the base URL of the Git platform.
	Endpoint string `json:"endpoint"`

	// ClientID is the OAuth2 client ID.
	ClientID string `json:"clientId"`

	// ClientSecret is a reference to the Secret containing the OAuth2 client secret.
	ClientSecret corev1.SecretKeySelector `json:"clientSecret"`

	// RedirectURL is the OAuth2 callback URL.
	RedirectURL string `json:"redirectUrl"`

	// ForgeURL is the API URL for the Git platform. Defaults to Endpoint.
	// +kubebuilder:validation:Optional
	ForgeURL string `json:"forgeUrl,omitempty"`

	// AuthURL is a custom OAuth2 authorization URL. Derived from Type and Endpoint if not specified.
	// +kubebuilder:validation:Optional
	AuthURL string `json:"authUrl,omitempty"`

	// Insecure disables TLS verification (not recommended for production).
	// +kubebuilder:validation:Optional
	Insecure bool `json:"insecure,omitempty"`

	// DisplayName is a human-readable name shown in the login UI.
	// Derived from the Endpoint hostname when empty.
	// +kubebuilder:validation:Optional
	DisplayName string `json:"displayName,omitempty"`

	// IconURL is the URL of an icon shown next to the provider in the login UI.
	// Defaults to the Endpoint's favicon.ico when empty.
	// +kubebuilder:validation:Optional
	IconURL string `json:"iconUrl,omitempty"`
}

// AuthProviderStatus defines the observed state of AuthProvider.
type AuthProviderStatus struct {
	// Conditions represent the latest available observations of the AuthProvider's state.
	// +kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// Registered indicates whether the provider is currently registered with the auth manager.
	// +kubebuilder:validation:Optional
	Registered bool `json:"registered,omitempty"`

	// SecretResourceVersion tracks the resource version of the referenced secret.
	// This allows the controller to detect when the secret has been rotated.
	// +kubebuilder:validation:Optional
	SecretResourceVersion string `json:"secretResourceVersion,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// AuthProvider is the Schema for the auth providers API.
type AuthProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AuthProviderSpec   `json:"spec,omitempty"`
	Status AuthProviderStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AuthProviderList contains a list of AuthProvider.
type AuthProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AuthProvider `json:"items"`
}

func (a *AuthProvider) SetCondition(
	conditionType string,
	status metav1.ConditionStatus,
	reason, message string,
) {
	api_meta.SetStatusCondition(&a.Status.Conditions, metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: a.Generation,
	})
}

func (a *AuthProvider) GetCondition(conditionType string) *metav1.Condition {
	return api_meta.FindStatusCondition(a.Status.Conditions, conditionType)
}

func (a *AuthProvider) RemoveCondition(conditionType string) {
	api_meta.RemoveStatusCondition(&a.Status.Conditions, conditionType)
}
