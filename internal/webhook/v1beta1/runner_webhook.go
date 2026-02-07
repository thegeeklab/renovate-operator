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
	runnerLog = logf.Log.WithName("runner-resource")

	ErrRunnerObjectType = errors.New("expected a Runner object but got other type")
)

// SetupRunnerWebhookWithManager registers the webhook for Runner in the manager.
func SetupRunnerWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &renovatev1beta1.Runner{}).
		WithDefaulter(&RunnerCustomDefaulter{}).
		Complete()
}

//nolint:lll
// +kubebuilder:webhook:path=/mutate-renovate-thegeeklab-de-v1beta1-runner,mutating=true,failurePolicy=fail,sideEffects=None,groups=renovate.thegeeklab.de,resources=runners,verbs=create;update,versions=v1beta1,name=mrunner-v1beta1.kb.io,admissionReviewVersions=v1

// RunnerCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind Runner when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type RunnerCustomDefaulter struct{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind Runner.
func (d *RunnerCustomDefaulter) Default(ctx context.Context, runner *renovatev1beta1.Runner) error {
	if runner == nil {
		return fmt.Errorf("%w: %T", ErrRunnerObjectType, runner)
	}

	runnerLog.Info("Defaulting for Runner", "name", runner.GetName())

	if runner.Spec.Logging == nil {
		runner.Spec.Logging = &renovatev1beta1.LoggingSpec{}
	}

	if runner.Spec.Logging.Level == "" {
		runner.Spec.Logging.Level = renovatev1beta1.LogLevel_INFO
	}

	if runner.Spec.Strategy == "" {
		runner.Spec.Strategy = renovatev1beta1.RunnerStrategy_NONE
	}

	if runner.Spec.Instances == 0 {
		runner.Spec.Instances = 1
	}

	if runner.Spec.Image == "" {
		runner.Spec.Image = renovatev1beta1.OperatorContainerImage
	}

	if runner.Spec.ImagePullPolicy == "" {
		runner.Spec.ImagePullPolicy = corev1.PullIfNotPresent
	}

	return nil
}
