package v1beta1

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
)

var (
	discoveryLog = logf.Log.WithName("discovery-resource")

	ErrDiscoveryObjectType = errors.New("expected a Discovery object but got other type")
)

// SetupDiscoveryWebhookWithManager registers the webhook for Discovery in the manager.
func SetupDiscoveryWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &renovatev1beta1.Discovery{}).
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
type DiscoveryCustomDefaulter struct{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind Discovery.
func (d *DiscoveryCustomDefaulter) Default(ctx context.Context, discovery *renovatev1beta1.Discovery) error {
	if discovery == nil {
		return fmt.Errorf("%w: %T", ErrDiscoveryObjectType, discovery)
	}

	discoveryLog.Info("Defaulting for Discovery", "name", discovery.GetName())

	if discovery.Spec.Image == "" {
		discovery.Spec.Image = renovatev1beta1.DefaultOperatorContainerImage
	}

	if discovery.Spec.ImagePullPolicy == "" {
		discovery.Spec.ImagePullPolicy = corev1.PullIfNotPresent
	}

	if discovery.Spec.Logging == nil {
		discovery.Spec.Logging = &renovatev1beta1.LoggingSpec{}
	}

	if discovery.Spec.Logging.Level == "" {
		discovery.Spec.Logging.Level = renovatev1beta1.LogLevel_INFO
	}

	if discovery.Spec.Suspend == nil {
		discovery.Spec.Suspend = ptr.To(false)
	}

	if discovery.Spec.Schedule == "" {
		discovery.Spec.Schedule = renovatev1beta1.DefaultSchedule
	}

	return nil
}
