package v1beta1

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

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
		WithValidator(&RunnerCustomValidator{}).
		Complete()
}

//nolint:lll
// +kubebuilder:webhook:path=/mutate-renovate-thegeeklab-de-v1beta1-runner,mutating=true,failurePolicy=fail,sideEffects=None,groups=renovate.thegeeklab.de,resources=runners,verbs=create;update,versions=v1beta1,name=mrunner-v1beta1.kb.io,admissionReviewVersions=v1
// +kubebuilder:webhook:path=/validate-renovate-thegeeklab-de-v1beta1-runner,mutating=false,failurePolicy=fail,sideEffects=None,groups=renovate.thegeeklab.de,resources=runners,verbs=create;update,versions=v1beta1,name=vrunner-v1beta1.kb.io,admissionReviewVersions=v1

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

	if runner.Spec.Image == "" {
		runner.Spec.Image = renovatev1beta1.DefaultOperatorContainerImage
	}

	if runner.Spec.ImagePullPolicy == "" {
		runner.Spec.ImagePullPolicy = corev1.PullIfNotPresent
	}

	if runner.Spec.Logging == nil {
		runner.Spec.Logging = &renovatev1beta1.LoggingSpec{}
	}

	if runner.Spec.Logging.Level == "" {
		runner.Spec.Logging.Level = renovatev1beta1.LogLevel_INFO
	}

	if runner.Spec.Suspend == nil {
		runner.Spec.Suspend = new(false)
	}

	if runner.Spec.Schedule == "" {
		runner.Spec.Schedule = renovatev1beta1.DefaultSchedule
	}

	if runner.Spec.SuccessLimit == nil {
		runner.Spec.SuccessLimit = new(renovatev1beta1.DefaultSuccessLimit)
	}

	if runner.Spec.FailedLimit == nil {
		runner.Spec.FailedLimit = new(renovatev1beta1.DefaultFailedLimit)
	}

	if runner.Spec.BackoffLimit == nil {
		runner.Spec.BackoffLimit = new(renovatev1beta1.DefaultBackoffLimit)
	}

	if runner.Spec.MaxParallel == nil {
		runner.Spec.MaxParallel = new(renovatev1beta1.DefaultRunnerMaxParallel)
	}

	defaultScratchVolume(runner.Spec.ScratchVolume)

	return nil
}

// RunnerCustomValidator struct is responsible for validating the Kind Runner resource
// when it is created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type RunnerCustomValidator struct{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the Kind Runner.
func (v *RunnerCustomValidator) ValidateCreate(
	_ context.Context,
	runner *renovatev1beta1.Runner,
) (admission.Warnings, error) {
	if runner == nil {
		return nil, fmt.Errorf("%w: %T", ErrRunnerObjectType, runner)
	}

	runnerLog.Info("Validation for Runner upon creation", "name", runner.GetName())

	if err := validateTimezone(runner.Spec.Timezone); err != nil {
		return nil, err
	}

	if err := validateScratchVolumePath(runner.Spec.ScratchVolume); err != nil {
		return nil, err
	}

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the Kind Runner.
func (v *RunnerCustomValidator) ValidateUpdate(
	_ context.Context,
	_, newRunner *renovatev1beta1.Runner,
) (admission.Warnings, error) {
	if newRunner == nil {
		return nil, fmt.Errorf("%w: %T", ErrRunnerObjectType, newRunner)
	}

	runnerLog.Info("Validation for Runner upon update", "name", newRunner.GetName())

	if err := validateTimezone(newRunner.Spec.Timezone); err != nil {
		return nil, err
	}

	if err := validateScratchVolumePath(newRunner.Spec.ScratchVolume); err != nil {
		return nil, err
	}

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the Kind Runner.
func (v *RunnerCustomValidator) ValidateDelete(
	_ context.Context,
	runner *renovatev1beta1.Runner,
) (admission.Warnings, error) {
	if runner == nil {
		return nil, fmt.Errorf("%w: %T", ErrRunnerObjectType, runner)
	}

	return nil, nil
}
