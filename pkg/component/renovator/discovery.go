package renovator

import (
	"context"
	"fmt"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/metadata"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"

	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *Reconciler) reconcileDiscovery(ctx context.Context) (*ctrl.Result, error) {
	discovery := &renovatev1beta1.Discovery{ObjectMeta: metadata.GenericMetadata(r.req)}

	_, err := k8s.CreateOrPatch(ctx, r.Client, discovery, r.instance, func() error {
		return r.updateDiscovery(discovery)
	})
	if err != nil {
		return &ctrl.Result{Requeue: true}, fmt.Errorf("failed to create or update discovery: %w", err)
	}

	return &ctrl.Result{}, nil
}

func (r *Reconciler) updateDiscovery(discovery *renovatev1beta1.Discovery) error {
	// Copy the discovery configuration from the Renovator spec
	discovery.Spec = renovatev1beta1.DiscoverySpec{
		ImageSpec: renovatev1beta1.ImageSpec{
			Image:           r.instance.Spec.Image,
			ImagePullPolicy: r.instance.Spec.ImagePullPolicy,
		},
		Suspend:  r.instance.Spec.Discovery.Suspend,
		Schedule: r.instance.Spec.Discovery.Schedule,
		Filter:   r.instance.Spec.Discovery.Filter,
	}

	return nil
}
