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
	renovatorLog = logf.Log.WithName("renovator-resource")

	ErrRenovatorObjectType = errors.New("expected a Renovator object but got other type")
)

// SetupRenovatorWebhookWithManager registers the webhook for Renovator in the manager.
func SetupRenovatorWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&renovatev1beta1.Renovator{}).
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
type RenovatorCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

var _ webhook.CustomDefaulter = &RenovatorCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind Renovator.
func (d *RenovatorCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	renovator, ok := obj.(*renovatev1beta1.Renovator)

	if !ok {
		return fmt.Errorf("%w: %T", ErrRenovatorObjectType, obj)
	}

	renovatorLog.Info("Defaulting for renovator", "name", renovator.GetName())

	renovator.Default()

	return nil
}
