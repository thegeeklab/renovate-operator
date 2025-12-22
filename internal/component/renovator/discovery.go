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
	discovery.Spec = r.instance.Spec.Discovery
	discovery.Spec.ConfigRef = r.instance.Name

	if discovery.Spec.Logging == nil {
		discovery.Spec.Logging = &r.instance.Spec.Logging
	}

	if discovery.Spec.Image == "" {
		discovery.Spec.Image = r.instance.Spec.Image
	}

	if discovery.Spec.ImagePullPolicy == "" {
		discovery.Spec.ImagePullPolicy = r.instance.Spec.ImagePullPolicy
	}

	// Forward operation annotations from Renovator to Discovery
	if HasRenovatorOperationDiscover(r.instance.Annotations) {
		if discovery.Annotations == nil {
			discovery.Annotations = make(map[string]string)
		}

		discovery.Annotations[renovatev1beta1.RenovatorOperation] = renovatev1beta1.OperationDiscover
	}

	return nil
}
