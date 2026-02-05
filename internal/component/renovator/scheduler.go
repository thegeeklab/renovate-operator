//nolint:dupl
package renovator

import (
	"context"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/metadata"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"

	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *Reconciler) reconcileScheduler(ctx context.Context) (*ctrl.Result, error) {
	scheduler := &renovatev1beta1.Scheduler{ObjectMeta: metadata.GenericMetadata(r.req)}

	_, err := k8s.CreateOrUpdate(ctx, r.Client, scheduler, r.instance, func() error {
		return r.updateScheduler(scheduler)
	})

	return &ctrl.Result{}, err
}

func (r *Reconciler) updateScheduler(scheduler *renovatev1beta1.Scheduler) error {
	// Copy the scheduler configuration from the Renovator spec
	scheduler.Spec = r.instance.Spec.Scheduler
	scheduler.Spec.ConfigRef = metadata.GenericName(r.req)

	if scheduler.Spec.Logging == nil {
		scheduler.Spec.Logging = &r.instance.Spec.Logging
	}

	if scheduler.Spec.Image == "" {
		scheduler.Spec.Image = r.instance.Spec.Image
	}

	if scheduler.Spec.ImagePullPolicy == "" {
		scheduler.Spec.ImagePullPolicy = r.instance.Spec.ImagePullPolicy
	}

	// Forward operation annotations from Renovator to Discovery
	if HasRenovatorOperationRenovate(r.instance.Annotations) {
		if scheduler.Annotations == nil {
			scheduler.Annotations = make(map[string]string)
		}

		scheduler.Annotations[renovatev1beta1.RenovatorOperation] = renovatev1beta1.OperationRenovate
	}

	return nil
}
