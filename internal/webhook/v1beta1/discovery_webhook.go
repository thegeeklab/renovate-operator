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
	discoveryLog = logf.Log.WithName("discovery-resource")

	ErrDiscoveryObjectType = errors.New("expected a Discovery object but got other type")
)

// SetupDiscoveryWebhookWithManager registers the webhook for Discovery in the manager.
func SetupDiscoveryWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&renovatev1beta1.Discovery{}).
		WithDefaulter(&DiscoveryCustomDefaulter{}).
		Complete()
}

//nolint:lll
// +kubebuilder:webhook:path=/mutate-renovate-thegeeklab-de-v1beta1-discovery,mutating=true,failurePolicy=fail,sideEffects=None,groups=renovate.thegeeklab.de,resources=discoveries,verbs=create;update,versions=v1beta1,name=mdiscovery-v1beta1.kb.io,admissionReviewVersions=v1

// DiscoveryCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind Discovery when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type DiscoveryCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

var _ webhook.CustomDefaulter = &DiscoveryCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind Discovery.
func (d *DiscoveryCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	discovery, ok := obj.(*renovatev1beta1.Discovery)

	if !ok {
		return fmt.Errorf("%w: %T", ErrDiscoveryObjectType, obj)
	}

	discoveryLog.Info("Defaulting for Discovery", "name", discovery.GetName())

	discovery.Default()

	return nil
}
