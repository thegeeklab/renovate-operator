package renovator

import (
	"context"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/metadata"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"

	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *Reconciler) reconcileDiscovery(ctx context.Context) (*ctrl.Result, error) {
	discovery := &renovatev1beta1.Discovery{ObjectMeta: metadata.GenericMetadata(r.req)}

	_, err := k8s.CreateOrUpdate(ctx, r.Client, discovery, r.instance, func() error {
		return r.updateDiscovery(discovery)
	})

	return &ctrl.Result{}, err
}

func (r *Reconciler) updateDiscovery(discovery *renovatev1beta1.Discovery) error {
	spec := r.instance.Spec
	discoverySpec := r.instance.Spec.Discovery

	discovery.Spec.ConfigRef = discoverySpec.ConfigRef
	discovery.Spec.Filter = discoverySpec.Filter

	discovery.Spec.Image = spec.Image
	if discoverySpec.Image != "" {
		discovery.Spec.Image = discoverySpec.Image
	}

	discovery.Spec.ImagePullPolicy = spec.ImagePullPolicy
	if discoverySpec.ImagePullPolicy != "" {
		discovery.Spec.ImagePullPolicy = discoverySpec.ImagePullPolicy
	}

	discovery.Spec.ImagePullSecrets = spec.ImagePullSecrets
	if discoverySpec.ImagePullSecrets != nil {
		discovery.Spec.ImagePullSecrets = discoverySpec.ImagePullSecrets
	}

	discovery.Spec.Suspend = spec.Suspend
	if discoverySpec.Suspend != nil {
		discovery.Spec.Suspend = discoverySpec.Suspend
	}

	discovery.Spec.Schedule = spec.Schedule
	if discoverySpec.Schedule != "" {
		discovery.Spec.Schedule = discoverySpec.Schedule
	}

	discovery.Spec.Timezone = spec.Timezone
	if discoverySpec.Timezone != "" {
		discovery.Spec.Timezone = discoverySpec.Timezone
	}

	discovery.Spec.SuccessLimit = spec.SuccessLimit
	if discoverySpec.SuccessLimit != nil {
		discovery.Spec.SuccessLimit = discoverySpec.SuccessLimit
	}

	discovery.Spec.FailedLimit = spec.FailedLimit
	if discoverySpec.FailedLimit != nil {
		discovery.Spec.FailedLimit = discoverySpec.FailedLimit
	}

	discovery.Spec.BackoffLimit = spec.BackoffLimit
	if discoverySpec.BackoffLimit != nil {
		discovery.Spec.BackoffLimit = discoverySpec.BackoffLimit
	}

	discovery.Spec.TTLSecondsAfterFinished = spec.TTLSecondsAfterFinished
	if discoverySpec.TTLSecondsAfterFinished != nil {
		discovery.Spec.TTLSecondsAfterFinished = discoverySpec.TTLSecondsAfterFinished
	}

	discovery.Spec.NodeSelector = spec.NodeSelector
	if discoverySpec.NodeSelector != nil {
		discovery.Spec.NodeSelector = discoverySpec.NodeSelector
	}

	discovery.Spec.Affinity = spec.Affinity
	if discoverySpec.Affinity != nil {
		discovery.Spec.Affinity = discoverySpec.Affinity
	}

	discovery.Spec.Tolerations = spec.Tolerations
	if discoverySpec.Tolerations != nil {
		discovery.Spec.Tolerations = discoverySpec.Tolerations
	}

	discovery.Spec.TopologySpreadConstraints = spec.TopologySpreadConstraints
	if discoverySpec.TopologySpreadConstraints != nil {
		discovery.Spec.TopologySpreadConstraints = discoverySpec.TopologySpreadConstraints
	}

	discovery.Spec.Resources = spec.Resources
	if discoverySpec.Resources.Limits != nil || discoverySpec.Resources.Requests != nil {
		discovery.Spec.Resources = discoverySpec.Resources
	}

	discovery.Spec.SecurityContext = spec.SecurityContext
	if discoverySpec.SecurityContext != nil {
		discovery.Spec.SecurityContext = discoverySpec.SecurityContext
	}

	discovery.Spec.ExtraEnv = spec.ExtraEnv
	if discoverySpec.ExtraEnv != nil {
		discovery.Spec.ExtraEnv = discoverySpec.ExtraEnv
	}

	discovery.Spec.ExtraVolumes = spec.ExtraVolumes
	if discoverySpec.ExtraVolumes != nil {
		discovery.Spec.ExtraVolumes = discoverySpec.ExtraVolumes
	}

	discovery.Spec.RuntimeClassName = spec.RuntimeClassName
	if discoverySpec.RuntimeClassName != nil {
		discovery.Spec.RuntimeClassName = discoverySpec.RuntimeClassName
	}

	discovery.Spec.PodAnnotations = spec.PodAnnotations
	if discoverySpec.PodAnnotations != nil {
		discovery.Spec.PodAnnotations = discoverySpec.PodAnnotations
	}

	discovery.Spec.ScratchVolume = spec.ScratchVolume
	if discoverySpec.ScratchVolume != nil {
		discovery.Spec.ScratchVolume = discoverySpec.ScratchVolume
	}

	discovery.Spec.SkipForks = discoverySpec.SkipForks
	discovery.Spec.Topics = discoverySpec.Topics

	discovery.Spec.Webhooks.Enabled = spec.Webhooks.Enabled
	if discoverySpec.Webhooks.Enabled != nil {
		discovery.Spec.Webhooks.Enabled = discoverySpec.Webhooks.Enabled
	}

	logging := &spec.Logging
	if discoverySpec.Logging != nil {
		logging = discoverySpec.Logging
	}

	discovery.Spec.Logging = logging

	if discovery.Labels == nil {
		discovery.Labels = make(map[string]string)
	}

	discovery.Labels[renovatev1beta1.LabelRenovator] = string(r.instance.UID)

	if HasRenovatorOperationDiscover(r.instance.Annotations) {
		if discovery.Annotations == nil {
			discovery.Annotations = make(map[string]string)
		}

		discovery.Annotations[renovatev1beta1.RenovatorOperation] = renovatev1beta1.OperationDiscover
	}

	return nil
}
