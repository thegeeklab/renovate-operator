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
	spec := r.instance.Spec
	disco := r.instance.Spec.Discovery

	discovery.Spec.ConfigRef = disco.ConfigRef
	discovery.Spec.Filter = disco.Filter

	if spec.Image != "" {
		discovery.Spec.Image = spec.Image
	}

	if disco.Image != "" {
		discovery.Spec.Image = disco.Image
	}

	if spec.ImagePullPolicy != "" {
		discovery.Spec.ImagePullPolicy = spec.ImagePullPolicy
	}

	if disco.ImagePullPolicy != "" {
		discovery.Spec.ImagePullPolicy = disco.ImagePullPolicy
	}

	if spec.Schedule != "" {
		discovery.Spec.Schedule = spec.Schedule
	}

	if disco.Schedule != "" {
		discovery.Spec.Schedule = disco.Schedule
	}

	if spec.Suspend != nil {
		discovery.Spec.Suspend = spec.Suspend
	}

	if disco.Suspend != nil {
		discovery.Spec.Suspend = disco.Suspend
	}

	logging := &spec.Logging
	if disco.Logging != nil {
		logging = disco.Logging
	}

	discovery.Spec.Logging = logging

	if discovery.Labels == nil {
		discovery.Labels = make(map[string]string)
	}

	discovery.Labels[renovatev1beta1.RenovatorLabel] = string(r.instance.UID)

	if HasRenovatorOperationDiscover(r.instance.Annotations) {
		if discovery.Annotations == nil {
			discovery.Annotations = make(map[string]string)
		}

		discovery.Annotations[renovatev1beta1.RenovatorOperation] = renovatev1beta1.OperationDiscover
	}

	return nil
}
