package renovator

import (
	"context"
	"fmt"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/metadata"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//nolint:dupl
func (r *Reconciler) reconcileRunner(ctx context.Context) (*ctrl.Result, error) {
	runner := &renovatev1beta1.Runner{ObjectMeta: metadata.GenericMetadata(r.req)}

	_, err := k8s.CreateOrUpdate(ctx, r.Client, runner, r.instance, func() error {
		return r.updateRunner(runner)
	})
	if err != nil {
		return &ctrl.Result{}, err
	}

	if HasRenovatorOperationRenovate(r.instance.Annotations) {
		patch := client.MergeFrom(r.instance.DeepCopy())
		r.instance.Annotations = RemoveOperation(r.instance.Annotations, renovatev1beta1.OperationRenovate)

		if err := r.Patch(ctx, r.instance, patch); err != nil {
			return &ctrl.Result{}, fmt.Errorf("remove renovate operation: %w", err)
		}
	}

	return &ctrl.Result{}, nil
}

func (r *Reconciler) updateRunner(runner *renovatev1beta1.Runner) error {
	spec := r.instance.Spec
	runnerSpec := r.instance.Spec.Runner

	runner.Spec.ConfigRef = runnerSpec.ConfigRef

	runner.Spec.Image = spec.Image
	if runnerSpec.Image != "" {
		runner.Spec.Image = runnerSpec.Image
	}

	runner.Spec.ImagePullPolicy = spec.ImagePullPolicy
	if runnerSpec.ImagePullPolicy != "" {
		runner.Spec.ImagePullPolicy = runnerSpec.ImagePullPolicy
	}

	runner.Spec.ImagePullSecrets = spec.ImagePullSecrets
	if runnerSpec.ImagePullSecrets != nil {
		runner.Spec.ImagePullSecrets = runnerSpec.ImagePullSecrets
	}

	runner.Spec.Schedule = spec.Schedule
	if runnerSpec.Schedule != "" {
		runner.Spec.Schedule = runnerSpec.Schedule
	}

	runner.Spec.Timezone = spec.Timezone
	if runnerSpec.Timezone != "" {
		runner.Spec.Timezone = runnerSpec.Timezone
	}

	runner.Spec.Suspend = spec.Suspend
	if runnerSpec.Suspend != nil {
		runner.Spec.Suspend = runnerSpec.Suspend
	}

	runner.Spec.SuccessLimit = spec.SuccessLimit
	if runnerSpec.SuccessLimit != nil {
		runner.Spec.SuccessLimit = runnerSpec.SuccessLimit
	}

	runner.Spec.FailedLimit = spec.FailedLimit
	if runnerSpec.FailedLimit != nil {
		runner.Spec.FailedLimit = runnerSpec.FailedLimit
	}

	runner.Spec.BackoffLimit = spec.BackoffLimit
	if runnerSpec.BackoffLimit != nil {
		runner.Spec.BackoffLimit = runnerSpec.BackoffLimit
	}

	runner.Spec.TTLSecondsAfterFinished = spec.TTLSecondsAfterFinished
	if runnerSpec.TTLSecondsAfterFinished != nil {
		runner.Spec.TTLSecondsAfterFinished = runnerSpec.TTLSecondsAfterFinished
	}

	runner.Spec.NodeSelector = spec.NodeSelector
	if runnerSpec.NodeSelector != nil {
		runner.Spec.NodeSelector = runnerSpec.NodeSelector
	}

	runner.Spec.Affinity = spec.Affinity
	if runnerSpec.Affinity != nil {
		runner.Spec.Affinity = runnerSpec.Affinity
	}

	runner.Spec.Tolerations = spec.Tolerations
	if runnerSpec.Tolerations != nil {
		runner.Spec.Tolerations = runnerSpec.Tolerations
	}

	runner.Spec.TopologySpreadConstraints = spec.TopologySpreadConstraints
	if runnerSpec.TopologySpreadConstraints != nil {
		runner.Spec.TopologySpreadConstraints = runnerSpec.TopologySpreadConstraints
	}

	runner.Spec.Resources = spec.Resources
	if runnerSpec.Resources.Limits != nil || runnerSpec.Resources.Requests != nil {
		runner.Spec.Resources = runnerSpec.Resources
	}

	runner.Spec.SecurityContext = spec.SecurityContext
	if runnerSpec.SecurityContext != nil {
		runner.Spec.SecurityContext = runnerSpec.SecurityContext
	}

	runner.Spec.ExtraEnv = spec.ExtraEnv
	if runnerSpec.ExtraEnv != nil {
		runner.Spec.ExtraEnv = runnerSpec.ExtraEnv
	}

	runner.Spec.ExtraVolumes = spec.ExtraVolumes
	if runnerSpec.ExtraVolumes != nil {
		runner.Spec.ExtraVolumes = runnerSpec.ExtraVolumes
	}

	runner.Spec.RuntimeClassName = spec.RuntimeClassName
	if runnerSpec.RuntimeClassName != nil {
		runner.Spec.RuntimeClassName = runnerSpec.RuntimeClassName
	}

	runner.Spec.PodAnnotations = spec.PodAnnotations
	if runnerSpec.PodAnnotations != nil {
		runner.Spec.PodAnnotations = runnerSpec.PodAnnotations
	}

	runner.Spec.ScratchVolume = spec.ScratchVolume
	if runnerSpec.ScratchVolume != nil {
		runner.Spec.ScratchVolume = runnerSpec.ScratchVolume
	}

	runner.Spec.MaxParallel = runnerSpec.MaxParallel

	logging := &spec.Logging
	if runnerSpec.Logging != nil {
		logging = runnerSpec.Logging
	}

	runner.Spec.Logging = logging

	if runner.Labels == nil {
		runner.Labels = make(map[string]string)
	}

	runner.Labels[renovatev1beta1.LabelRenovator] = string(r.instance.UID)

	if HasRenovatorOperationRenovate(r.instance.Annotations) {
		if runner.Annotations == nil {
			runner.Annotations = make(map[string]string)
		}

		runner.Annotations[renovatev1beta1.RenovatorOperation] = renovatev1beta1.OperationRenovate
	}

	return nil
}
