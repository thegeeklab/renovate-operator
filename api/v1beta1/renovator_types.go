package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RenovatorStatusPhase string

const (
	RenovatorStatusPhasePending     RenovatorStatusPhase = "Pending"
	RenovatorStatusPhaseCreating    RenovatorStatusPhase = "Creating"
	RenovatorStatusPhaseCreated     RenovatorStatusPhase = "Created"
	RenovatorStatusPhaseTerminating RenovatorStatusPhase = "Terminating"
	RenovatorStatusPhaseDeleted     RenovatorStatusPhase = "Deleted"
	RenovatorStatusPhaseError       RenovatorStatusPhase = "Error"
)

// +kubebuilder:validation:Enum=github;gitea
type PlatformTypes string

//nolint:revive,stylecheck
const (
	PlatformType_GITHUB = "github"
	PlatformType_GITEA  = "gitea"
)

type Platform struct {
	PlatformType PlatformTypes       `json:"type"`
	Endpoint     string              `json:"endpoint"`
	Token        corev1.EnvVarSource `json:"token"`
}

type RenovateConfig struct {
	Platform Platform `json:"platform"`
	// +kubebuilder:default:="27.15.0"
	RenovateVersion string `json:"version,omitempty"`
	// +kubebuilder:default:=false
	DryRun *bool `json:"dryRun,omitempty"`
	// +kubebuilder:default:=true
	OnBoarding *bool `json:"onBoarding,omitempty"`
	// OnBoardingConfig object `json:"onBoardingConfig,omitempty,inline"`
	// +kubebuilder:default:=10
	PrHourlyLimit       int                 `json:"prHourlyLimit,omitempty"`
	AddLabels           []string            `json:"addLabels,omitempty"`
	GithubTokenSelector corev1.EnvVarSource `json:"githubToken,omitempty"`
}
type Discovery struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="0 */2 * * *"
	Schedule string `json:"schedule"`
}

// +kubebuilder:validation:Enum=trace;debug;info;warn;error;fatal
type LogLevel string

//nolint:revive,stylecheck
const (
	LogLevel_TRACE = "trace"
	LogLevel_DEBUG = "debug"
	LogLevel_INFO  = "info"
	LogLevel_WARN  = "warn"
	LogLevel_ERROR = "error"
	LogLevel_FATAL = "fatal"
)

type Logging struct {
	// +kubebuilder:default=info
	Level LogLevel `json:"level"`
}

type ScalingStrategy string

//nolint:revive,stylecheck
const (
	// ScalingStrategy_NONE A single batch be created and no parallelization will take place.
	ScalingStrategy_NONE = "none"
	// ScalingStrategy_SIZE Create batches based on number of repositories. If 30 repositories have been found and size
	// is defined as 10, then 3 batches will be created.
	ScalingStrategy_SIZE = "size"
)

type Scaling struct {
	// +kubebuilder:validation:Enum=none;size
	// +kubebuilder:default:="none"
	Strategy ScalingStrategy `json:"strategy,omitempty"`

	// MaxWorkers Maximum number of parallel workers to start. A single worker will only process a single batch at maximum.
	// +kubebuilder:default:=1
	MaxWorkers int32 `json:"maxWorkers"`

	Size int `json:"size,omitempty"`
}

// RenovatorSpec defines the desired state of Renovator.
type RenovatorSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

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

	RenovateConfig RenovateConfig `json:"renovate"`

	Discovery Discovery `json:"discovery,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=false
	Suspend *bool `json:"suspend"`

	Schedule string `json:"schedule"`

	// +kubebuilder:validation:Optional
	Logging Logging `json:"logging"`

	// +kubebuilder:validation:Optional
	Scaling Scaling `json:"scaling"`
}

// RenovatorStatus defines the observed state of Renovator.
//
//nolint:lll
type RenovatorStatus struct {
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
