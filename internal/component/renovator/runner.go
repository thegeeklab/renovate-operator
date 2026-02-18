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
	runSpec := r.instance.Spec.Runner

	runner.Spec.ConfigRef = runSpec.ConfigRef

	if runSpec.Instances > 0 {
		runner.Spec.Instances = runSpec.Instances
	}

	if spec.Image != "" {
		runner.Spec.Image = spec.Image
	}

	if runSpec.Image != "" {
		runner.Spec.Image = runSpec.Image
	}

	if spec.ImagePullPolicy != "" {
		runner.Spec.ImagePullPolicy = spec.ImagePullPolicy
	}

	if runSpec.ImagePullPolicy != "" {
		runner.Spec.ImagePullPolicy = runSpec.ImagePullPolicy
	}

	if spec.Schedule != "" {
		runner.Spec.Schedule = spec.Schedule
	}

	if runSpec.Schedule != "" {
		runner.Spec.Schedule = runSpec.Schedule
	}

	if spec.Suspend != nil {
		runner.Spec.Suspend = spec.Suspend
	}

	if runSpec.Suspend != nil {
		runner.Spec.Suspend = runSpec.Suspend
	}

	logging := &spec.Logging
	if runSpec.Logging != nil {
		logging = runSpec.Logging
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
