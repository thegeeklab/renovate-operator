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
	renovateconfigLog = logf.Log.WithName("renovateconfig-resource")

	ErrRenovateConfigObjectType = errors.New("expected a RenovateConfig object but got other type")
)

// SetupRenovateConfigWebhookWithManager registers the webhook for RenovateConfig in the manager.
func SetupRenovateConfigWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&renovatev1beta1.RenovateConfig{}).
		WithDefaulter(&RenovateConfigCustomDefaulter{}).
		Complete()
}

//nolint:lll
// +kubebuilder:webhook:path=/mutate-renovate-thegeeklab-de-v1beta1-renovateconfig,mutating=true,failurePolicy=fail,sideEffects=None,groups=renovate.thegeeklab.de,resources=renovateconfigs,verbs=create;update,versions=v1beta1,name=mrenovateconfig-v1beta1.kb.io,admissionReviewVersions=v1

// RenovateConfigCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind RenovateConfig when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type RenovateConfigCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

var _ webhook.CustomDefaulter = &RenovateConfigCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind RenovateConfig.
func (d *RenovateConfigCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	renovateconfig, ok := obj.(*renovatev1beta1.RenovateConfig)

	if !ok {
		return fmt.Errorf("%w: %T", ErrRenovatorObjectType, obj)
	}

	renovateconfigLog.Info("Defaulting for RenovateConfig", "name", renovateconfig.GetName())

	renovateconfig.Default()

	return nil
}
