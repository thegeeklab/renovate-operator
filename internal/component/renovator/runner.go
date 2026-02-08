//nolint:dupl
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
	// Copy the runner configuration from the Renovator spec
	runner.Spec = r.instance.Spec.Runner

	if runner.Labels == nil {
		runner.Labels = make(map[string]string)
	}

	runner.Labels[renovatev1beta1.RenovatorLabel] = string(r.instance.UID)

	// Forward operation annotations from Renovator to Discovery
	if HasRenovatorOperationRenovate(r.instance.Annotations) {
		if runner.Annotations == nil {
			runner.Annotations = make(map[string]string)
		}

		runner.Annotations[renovatev1beta1.RenovatorOperation] = renovatev1beta1.OperationRenovate
	}

	return nil
}
