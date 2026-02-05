//nolint:dupl
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
	schedulerLog = logf.Log.WithName("scheduler-resource")

	ErrSchedulerObjectType = errors.New("expected a Scheduler object but got other type")
)

// SetupSchedulerWebhookWithManager registers the webhook for Scheduler in the manager.
func SetupSchedulerWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &renovatev1beta1.Scheduler{}).
		WithDefaulter(&SchedulerCustomDefaulter{}).
		Complete()
}

//nolint:lll
// +kubebuilder:webhook:path=/mutate-renovate-thegeeklab-de-v1beta1-scheduler,mutating=true,failurePolicy=fail,sideEffects=None,groups=renovate.thegeeklab.de,resources=schedulers,verbs=create;update,versions=v1beta1,name=mscheduler-v1beta1.kb.io,admissionReviewVersions=v1

// SchedulerCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind Scheduler when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type SchedulerCustomDefaulter struct{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind Scheduler.
func (d *SchedulerCustomDefaulter) Default(ctx context.Context, scheduler *renovatev1beta1.Scheduler) error {
	if scheduler == nil {
		return fmt.Errorf("%w: %T", ErrSchedulerObjectType, scheduler)
	}

	schedulerLog.Info("Defaulting for Scheduler", "name", scheduler.GetName())

	if scheduler.Spec.Logging == nil {
		scheduler.Spec.Logging = &renovatev1beta1.LoggingSpec{}
	}

	if scheduler.Spec.Logging.Level == "" {
		scheduler.Spec.Logging.Level = renovatev1beta1.LogLevel_INFO
	}

	if scheduler.Spec.Image == "" {
		scheduler.Spec.Image = renovatev1beta1.OperatorContainerImage
	}

	if scheduler.Spec.ImagePullPolicy == "" {
		scheduler.Spec.ImagePullPolicy = corev1.PullIfNotPresent
	}

	return nil
}
