package v1beta1

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
)

var (
	renovatorLog = logf.Log.WithName("renovator-resource")

	ErrRenovatorObjectType = errors.New("expected a Renovator object but got other type")
)

// SetupRenovatorWebhookWithManager registers the webhook for Renovator in the manager.
func SetupRenovatorWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &renovatev1beta1.Renovator{}).
		WithDefaulter(&RenovatorCustomDefaulter{}).
		Complete()
}

//nolint:lll
// +kubebuilder:webhook:path=/mutate-renovate-thegeeklab-de-v1beta1-renovator,mutating=true,failurePolicy=fail,sideEffects=None,groups=renovate.thegeeklab.de,resources=renovators,verbs=create;update,versions=v1beta1,name=mrenovator-v1beta1.kb.io,admissionReviewVersions=v1

// RenovatorCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind Renovator when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type RenovatorCustomDefaulter struct{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind Renovator.
func (d *RenovatorCustomDefaulter) Default(_ context.Context, renovator *renovatev1beta1.Renovator) error {
	if renovator == nil {
		return fmt.Errorf("%w: %T", ErrRenovatorObjectType, renovator)
	}

	renovatorLog.Info("Defaulting for renovator", "name", renovator.GetName())

	if renovator.Spec.Logging.Level == "" {
		renovator.Spec.Logging.Level = renovatev1beta1.LogLevel_INFO
	}

	if renovator.Spec.Scheduler.Strategy == "" {
		renovator.Spec.Scheduler.Strategy = renovatev1beta1.SchedulerStrategy_NONE
	}

	if renovator.Spec.Scheduler.Instances == 0 {
		renovator.Spec.Scheduler.Instances = 1
	}

	if renovator.Spec.Discovery.Schedule == "" {
		renovator.Spec.Discovery.Schedule = "0 */2 * * *"
	}

	if renovator.Spec.Image == "" {
		renovator.Spec.Image = renovatev1beta1.OperatorContainerImage
	}

	if renovator.Spec.ImagePullPolicy == "" {
		renovator.Spec.ImagePullPolicy = corev1.PullIfNotPresent
	}

	if renovator.Spec.Renovate.Image == "" {
		renovator.Spec.Renovate.Image = renovatev1beta1.RenovateContainerImage
	}

	if renovator.Spec.Renovate.ImagePullPolicy == "" {
		renovator.Spec.Renovate.ImagePullPolicy = corev1.PullIfNotPresent
	}

	return nil
}
