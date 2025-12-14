package renovator

import (
	"context"
	"fmt"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/metadata"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"

	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *Reconciler) reconcileRenovateConfig(ctx context.Context) (*ctrl.Result, error) {
	renovate := &renovatev1beta1.RenovateConfig{ObjectMeta: metadata.GenericMetadata(r.req)}

	_, err := k8s.CreateOrPatch(ctx, r.Client, renovate, r.instance, func() error {
		return r.updateRenovateConfig(renovate)
	})
	if err != nil {
		return &ctrl.Result{Requeue: true}, fmt.Errorf("failed to create or update RenovateConfig: %w", err)
	}

	return &ctrl.Result{}, nil
}

func (r *Reconciler) updateRenovateConfig(renovate *renovatev1beta1.RenovateConfig) error {
	renovate.Spec = r.instance.Spec.Renovate

	if renovate.Spec.Logging == nil {
		renovate.Spec.Logging = &r.instance.Spec.Logging
	}

	return nil
}
