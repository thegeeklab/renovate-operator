//nolint:dupl
package renovator

import (
	"context"
	"fmt"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/metadata"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"

	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *Reconciler) reconcileRunner(ctx context.Context) (*ctrl.Result, error) {
	runner := &renovatev1beta1.Runner{ObjectMeta: metadata.GenericMetadata(r.req)}

	_, err := k8s.CreateOrPatch(ctx, r.Client, runner, r.instance, func() error {
		return r.updateRunner(runner)
	})
	if err != nil {
		return &ctrl.Result{Requeue: true}, fmt.Errorf("failed to create or update runner: %w", err)
	}

	return &ctrl.Result{}, nil
}

func (r *Reconciler) updateRunner(runner *renovatev1beta1.Runner) error {
	// Copy the runner configuration from the Renovator spec
	runner.Spec = r.instance.Spec.Runner
	runner.Spec.ConfigRef = metadata.GenericName(r.req)

	if runner.Spec.Logging == nil {
		runner.Spec.Logging = &r.instance.Spec.Logging
	}

	if runner.Spec.Image == "" {
		runner.Spec.Image = r.instance.Spec.Image
	}

	if runner.Spec.ImagePullPolicy == "" {
		runner.Spec.ImagePullPolicy = r.instance.Spec.ImagePullPolicy
	}

	// Forward operation annotations from Renovator to Discovery
	if HasRenovatorOperationRenovate(r.instance.Annotations) {
		if runner.Annotations == nil {
			runner.Annotations = make(map[string]string)
		}

		runner.Annotations[renovatev1beta1.RenovatorOperation] = renovatev1beta1.OperationRenovate
	}

	return nil
}
