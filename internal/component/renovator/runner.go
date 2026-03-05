package renovator

import (
	"context"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/metadata"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"

	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *Reconciler) reconcileRunner(ctx context.Context) (*ctrl.Result, error) {
	runner := &renovatev1beta1.Runner{ObjectMeta: metadata.GenericMetadata(r.req)}

	_, err := k8s.CreateOrUpdate(ctx, r.Client, runner, r.instance, func() error {
		return r.updateRunner(runner)
	})

	return &ctrl.Result{}, err
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

	runner.Spec.Schedule = spec.Schedule
	if runnerSpec.Schedule != "" {
		runner.Spec.Schedule = runnerSpec.Schedule
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

	logging := &spec.Logging
	if runnerSpec.Logging != nil {
		logging = runnerSpec.Logging
	}

	runner.Spec.Logging = logging

	if runner.Labels == nil {
		runner.Labels = make(map[string]string)
	}

	runner.Labels[renovatev1beta1.RenovatorLabel] = string(r.instance.UID)

	if HasRenovatorOperationRenovate(r.instance.Annotations) {
		if runner.Annotations == nil {
			runner.Annotations = make(map[string]string)
		}

		runner.Annotations[renovatev1beta1.RenovatorOperation] = renovatev1beta1.OperationRenovate
	}

	return nil
}
