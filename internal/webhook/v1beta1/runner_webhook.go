package v1beta1

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
)

var (
	runnerLog = logf.Log.WithName("runner-resource")

	ErrRunnerObjectType = errors.New("expected a Runner object but got other type")
)

// SetupRunnerWebhookWithManager registers the webhook for Runner in the manager.
func SetupRunnerWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&renovatev1beta1.Runner{}).
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
type RunnerCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

var _ webhook.CustomDefaulter = &RunnerCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind Runner.
func (d *RunnerCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	runner, ok := obj.(*renovatev1beta1.Runner)

	if !ok {
		return fmt.Errorf("%w: %T", ErrRunnerObjectType, obj)
	}

	runnerLog.Info("Defaulting for Runner", "name", runner.GetName())

	runner.Default()

	return nil
}
