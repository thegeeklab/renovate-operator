package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	api_meta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	RenovatorConditionDiscoveryReady      = "DiscoveryReady"
	RenovatorConditionRunnerReady         = "RunnerReady"
	RenovatorConditionRenovateConfigReady = "RenovateConfigReady"
)

// +kubebuilder:validation:Enum=github;gitea
type PlatformType string

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

	DefaultOperatorContainerImage       = "docker.io/thegeeklab/renovate-operator:latest"
	DefaultRenovateContainerImage       = "ghcr.io/renovatebot/renovate:latest"
	DefaultSchedule                     = "0 */2 * * *"
	DefaultSuccessLimit           int32 = 3
	DefaultFailedLimit            int32 = 1
	DefaultBackoffLimit           int32 = 0
	DefaultScratchVolumePath            = "/tmp/renovate"
)

type LoggingSpec struct {
	Level LogLevel `json:"level"`
}

// ImageSpec defines the container image specification.
type ImageSpec struct {
	// Name of the container image, supporting both tags (`<image>:<tag>`)
	// and digests for deterministic and repeatable deployments
	// (`<image>:<tag>@sha256:<digestValue>`)
	// +kubebuilder:validation:Optional
	Image string `json:"image,omitempty"`

	// Image pull policy.
	// One of `Always`, `Never` or `IfNotPresent`.
	// If not defined, it defaults to `IfNotPresent`.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/containers/images#updating-images
	// +kubebuilder:validation:Optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// ImagePullSecrets is a list of references to secrets in the same namespace to use for pulling the image.
	// More info: https://kubernetes.io/docs/concepts/containers/images/#specifying-imagepullsecrets-on-a-pod
	// +kubebuilder:validation:Optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
}

type JobSpec struct {
	// Suspend specifies whether the job execution and scheduling should be paused.
	// +kubebuilder:validation:Optional
	Suspend *bool `json:"suspend,omitempty"`

	// Schedule specifies the cron-formatted schedule on which the job should run.
	// +kubebuilder:validation:Optional
	Schedule string `json:"schedule,omitempty"`

	// Timezone specifies the IANA timezone name (e.g., "America/New_York", "Europe/Berlin")
	// to use when evaluating the cron schedule. If not set, the operator's local timezone is used.
	// +kubebuilder:validation:Optional
	Timezone string `json:"timezone,omitempty"`

	// SuccessLimit specifies the number of successful finished jobs to retain for history.
	// +kubebuilder:validation:Optional
	SuccessLimit *int32 `json:"successLimit,omitempty"`

	// FailedLimit specifies the number of failed finished jobs to retain for history.
	// +kubebuilder:validation:Optional
	FailedLimit *int32 `json:"failedLimit,omitempty"`

	// BackoffLimit specifies the number of retries before marking this job as failed.
	// +kubebuilder:validation:Optional
	BackoffLimit *int32 `json:"backoffLimit,omitempty"`

	// TTLSecondsAfterFinished limits the lifetime of a Job that has finished execution
	// (either Complete or Failed). If this field is set, the job and its pods will be
	// automatically deleted after the specified number of seconds.
	// +kubebuilder:validation:Optional
	TTLSecondsAfterFinished *int32 `json:"ttlSecondsAfterFinished,omitempty"`
}

// ScratchVolumeSpec configures a scratch volume for RENOVATE_BASE_DIR.
type ScratchVolumeSpec struct {
	// Path is the mount path for the scratch volume (RENOVATE_BASE_DIR).
	// Must be an absolute path.
	Path string `json:"path,omitempty"`

	// Ephemeral uses a Kubernetes generic ephemeral volume for scratch (volume.ephemeral).
	// When set, Medium and SizeLimit are ignored.
	Ephemeral *corev1.EphemeralVolumeSource `json:"ephemeral,omitempty"`

	// Medium specifies the storage medium.
	Medium corev1.StorageMedium `json:"medium,omitempty"`

	// SizeLimit is the maximum size of the volume.
	SizeLimit *resource.Quantity `json:"sizeLimit,omitempty"`
}

// PodSpec defines pod-level scheduling configuration.
type PodSpec struct {
	// NodeSelector specifies the node selector for scheduling the renovate pod.
	// +kubebuilder:validation:Optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Affinity specifies the affinity settings for scheduling the renovate pod.
	// +kubebuilder:validation:Optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Tolerations specifies the tolerations for scheduling the renovate pod.
	// +kubebuilder:validation:Optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// TopologySpreadConstraints specifies the topology spread constraints for the renovate pod.
	// +kubebuilder:validation:Optional
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`

	// Resources specifies the resource requirements for the renovate container.
	// +kubebuilder:validation:Optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// SecurityContext specifies the security context for the renovate container.
	// +kubebuilder:validation:Optional
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty"`

	// ExtraEnv specifies additional environment variables for the renovate container.
	// +kubebuilder:validation:Optional
	ExtraEnv []corev1.EnvVar `json:"extraEnv,omitempty"`

	// ExtraVolumes specifies additional volumes for the renovate pod.
	// +kubebuilder:validation:Optional
	ExtraVolumes []corev1.Volume `json:"extraVolumes,omitempty"`

	// RuntimeClassName specifies the runtime class for the renovate pod.
	// +kubebuilder:validation:Optional
	RuntimeClassName *string `json:"runtimeClassName,omitempty"`

	// PodAnnotations specifies additional annotations for the renovate pod.
	// +kubebuilder:validation:Optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`

	// ScratchVolume configures a scratch volume for RENOVATE_BASE_DIR.
	// +kubebuilder:validation:Optional
	ScratchVolume *ScratchVolumeSpec `json:"scratchVolume,omitempty"`
}

// RenovatorSpec defines the desired state of Renovator.
type RenovatorSpec struct {
	ImageSpec `json:",inline"`

	// +kubebuilder:validation:Optional
	Logging LoggingSpec `json:"logging,omitempty"`

	JobSpec `json:",inline"`

	PodSpec `json:",inline"`

	Discovery DiscoverySpec `json:"discovery"`

	// +kubebuilder:validation:Optional
	Runner RunnerSpec `json:"runner"`

	Renovate RenovateConfigSpec `json:"renovate,omitempty"`

	// AuthProviderRef is a reference to an AuthProvider resource in the same namespace.
	// When set, this Renovator will use the referenced AuthProvider for authentication.
	// Multiple Renovators can reference the same AuthProvider to share authentication configuration.
	// +kubebuilder:validation:Optional
	AuthProviderRef string `json:"authProviderRef,omitempty"`
}

// RenovatorStatus defines the observed state of Renovator.
//
//nolint:lll
type RenovatorStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
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

func (r *Renovator) SetCondition(
	conditionType string,
	status metav1.ConditionStatus,
	reason, message string,
) {
	api_meta.SetStatusCondition(&r.Status.Conditions, metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: r.Generation,
	})
}

func (r *Renovator) GetCondition(conditionType string) *metav1.Condition {
	return api_meta.FindStatusCondition(r.Status.Conditions, conditionType)
}

func (r *Renovator) RemoveCondition(conditionType string) {
	api_meta.RemoveStatusCondition(&r.Status.Conditions, conditionType)
}
