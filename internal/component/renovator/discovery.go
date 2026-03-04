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
	discoverySpec := r.instance.Spec.Discovery

	discovery.Spec.ConfigRef = discoverySpec.ConfigRef
	discovery.Spec.Filter = discoverySpec.Filter

	if spec.Image != "" {
		discovery.Spec.Image = spec.Image
	}

	if discoverySpec.Image != "" {
		discovery.Spec.Image = discoverySpec.Image
	}

	if spec.ImagePullPolicy != "" {
		discovery.Spec.ImagePullPolicy = spec.ImagePullPolicy
	}

	if discoverySpec.ImagePullPolicy != "" {
		discovery.Spec.ImagePullPolicy = discoverySpec.ImagePullPolicy
	}

	if spec.Suspend != nil {
		discovery.Spec.Suspend = spec.Suspend
	}

	if discoverySpec.Suspend != nil {
		discovery.Spec.Suspend = discoverySpec.Suspend
	}

	if spec.Schedule != "" {
		discovery.Spec.Schedule = spec.Schedule
	}

	if discoverySpec.Schedule != "" {
		discovery.Spec.Schedule = discoverySpec.Schedule
	}

	if spec.SuccessLimit != nil {
		discovery.Spec.SuccessLimit = spec.SuccessLimit
	}

	if discoverySpec.SuccessLimit != nil {
		discovery.Spec.SuccessLimit = discoverySpec.SuccessLimit
	}

	if spec.FailedLimit != nil {
		discovery.Spec.FailedLimit = spec.FailedLimit
	}

	if discoverySpec.FailedLimit != nil {
		discovery.Spec.FailedLimit = discoverySpec.FailedLimit
	}

	if spec.BackoffLimit != nil {
		discovery.Spec.BackoffLimit = spec.BackoffLimit
	}

	if discoverySpec.BackoffLimit != nil {
		discovery.Spec.BackoffLimit = discoverySpec.BackoffLimit
	}

	if spec.TTLSecondsAfterFinished != nil {
		discovery.Spec.TTLSecondsAfterFinished = spec.TTLSecondsAfterFinished
	}

	if discoverySpec.TTLSecondsAfterFinished != nil {
		discovery.Spec.TTLSecondsAfterFinished = discoverySpec.TTLSecondsAfterFinished
	}

	logging := &spec.Logging
	if discoverySpec.Logging != nil {
		logging = discoverySpec.Logging
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
