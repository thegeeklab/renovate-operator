package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:validation:Enum=github;gitea
type PlatformType string

//nolint:revive
const (
	PlatformType_GITHUB = "github"
	PlatformType_GITEA  = "gitea"
)

type PlatformSpec struct {
	Type     PlatformType        `json:"type"`
	Endpoint string              `json:"endpoint"`
	Token    corev1.EnvVarSource `json:"token"`
}

// +kubebuilder:validation:Enum=extract;lookup;full
type DryRun string

//nolint:revive
const (
	DryRun_EXTRACT = "extract"
	DryRun_LOOKUP  = "lookup"
	DryRun_FULL    = "full"
)

type RenovateSpec struct {
	// Name of the container image, supporting both tags (`<image>:<tag>`)
	// and digests for deterministic and repeatable deployments
	// (`<image>:<tag>@sha256:<digestValue>`)
	Image string `json:"image,omitempty"`

	// Image pull policy.
	// One of `Always`, `Never` or `IfNotPresent`.
	// If not defined, it defaults to `IfNotPresent`.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/containers/images#updating-images
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	Platform PlatformSpec `json:"platform"`
	DryRun   DryRun       `json:"dryRun,omitempty"`
	// +kubebuilder:default:=true
	Onboarding *bool `json:"onboarding,omitempty"`
	// OnBoardingConfig object `json:"onBoardingConfig,omitempty,inline"`
	// +kubebuilder:default:=10
	PrHourlyLimit int      `json:"prHourlyLimit,omitempty"`
	AddLabels     []string `json:"addLabels,omitempty"`

	GithubToken *corev1.EnvVarSource `json:"githubToken,omitempty"`
}

// +kubebuilder:validation:Enum=trace;debug;info;warn;error;fatal
type LogLevel string

//nolint:revive
const (
	LogLevel_TRACE = "trace"
	LogLevel_DEBUG = "debug"
	LogLevel_INFO  = "info"
	LogLevel_WARN  = "warn"
	LogLevel_ERROR = "error"
	LogLevel_FATAL = "fatal"
)

type LoggingSpec struct {
	// +kubebuilder:default=info
	Level LogLevel `json:"level"`
}

type RunnerStrategy string

//nolint:revive
const (
	// RunnerStrategy_NONE A single batch be created and no parallelization will take place.
	RunnerStrategy_NONE = "none"
	// RunnerStrategy_BATCH Create batches based on number of repositories. If 30 repositories have been found and size
	// is defined as 10, then 3 batches will be created.
	RunnerStrategy_BATCH = "batch"
)

type RunnerSpec struct {
	// +kubebuilder:validation:Enum=none;batch
	// +kubebuilder:default:="none"
	Strategy RunnerStrategy `json:"strategy,omitempty"`

	// MaxRunners Maximum number of parallel runners to start. A single runner will only process a single batch.
	// +kubebuilder:default:=1
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	Instances int32 `json:"instances"`

	// BatchSize Number of repositories per batch. Only used when strategy is 'batch'.
	// If not specified, defaults to a reasonable size based on the number of repositories and instances.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1000
	BatchSize int `json:"batchSize,omitempty"`
}

type DiscoverySpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=false
	Suspend *bool `json:"suspend"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="0 */2 * * *"
	Schedule string `json:"schedule"`

	Filter []string `json:"filter,omitempty"`
}

// RenovatorSpec defines the desired state of Renovator.
type RenovatorSpec struct {
	// Name of the container image, supporting both tags (`<image>:<tag>`)
	// and digests for deterministic and repeatable deployments
	// (`<image>:<tag>@sha256:<digestValue>`)
	Image string `json:"image,omitempty"`

	// Image pull policy.
	// One of `Always`, `Never` or `IfNotPresent`.
	// If not defined, it defaults to `IfNotPresent`.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/containers/images#updating-images
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	Renovate RenovateSpec `json:"renovate"`

	Discovery DiscoverySpec `json:"discovery"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=false
	Suspend *bool `json:"suspend"`

	Schedule string `json:"schedule"`

	// +kubebuilder:validation:Optional
	Logging LoggingSpec `json:"logging"`

	// +kubebuilder:validation:Optional
	Runner RunnerSpec `json:"runner"`
}

// RenovatorStatus defines the observed state of Renovator.
//
//nolint:lll
type RenovatorStatus struct {
	Ready        bool               `json:"ready"`
	Failed       int                `json:"failed,omitempty"`
	Conditions   []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	SpecHash     string             `json:"specHash,omitempty"`
	Repositories []string           `json:"repositories,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Renovator is the Schema for the renovators API.
type Renovator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RenovatorSpec   `json:"spec,omitempty"`
	Status RenovatorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RenovatorList contains a list of Renovator.
type RenovatorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Renovator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Renovator{}, &RenovatorList{})
}
