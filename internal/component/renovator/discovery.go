//nolint:dupl
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
	// Copy the discovery configuration from the Renovator spec
	discovery.Spec = r.instance.Spec.Discovery

	if discovery.Spec.Logging == nil {
		discovery.Spec.Logging = &r.instance.Spec.Logging
	}

	if discovery.Spec.Image == "" {
		discovery.Spec.Image = r.instance.Spec.Image
	}

	if discovery.Spec.ImagePullPolicy == "" {
		discovery.Spec.ImagePullPolicy = r.instance.Spec.ImagePullPolicy
	}

	if discovery.Labels == nil {
		discovery.Labels = make(map[string]string)
	}

	discovery.Labels[renovatev1beta1.RenovatorLabel] = string(r.instance.UID)

	// Forward operation annotations from Renovator to Discovery
	if HasRenovatorOperationDiscover(r.instance.Annotations) {
		if discovery.Annotations == nil {
			discovery.Annotations = make(map[string]string)
		}

		discovery.Annotations[renovatev1beta1.RenovatorOperation] = renovatev1beta1.OperationDiscover
	}

	return nil
}
