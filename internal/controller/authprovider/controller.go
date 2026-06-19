package authprovider

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/auth"
	auth_gitea "github.com/thegeeklab/renovate-operator/internal/auth/gitea"
	"github.com/thegeeklab/renovate-operator/internal/controller"
)

const (
	ControllerName = "authprovider"
	// FinalizerCleanup is the finalizer added to AuthProvider resources to ensure
	// the provider is unregistered from the auth manager before deletion.
	FinalizerCleanup = "authprovider.renovate.thegeeklab.de/cleanup"
)

var (
	errSecretKeyNotFound = errors.New("secret key not found")
	errUnsupportedType   = errors.New("unsupported platform type")
)

// Reconciler reconciles an AuthProvider object.
type Reconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	EventRecorder events.EventRecorder
	AuthManager   *auth.Manager
}

//nolint:lll
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=authproviders,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=authproviders/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=renovate.thegeeklab.de,resources=authproviders/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.V(1).Info("Reconciling object", "object", req.NamespacedName)

	ap := &renovatev1beta1.AuthProvider{}
	if err := r.Get(ctx, req.NamespacedName, ap); err != nil {
		if api_errors.IsNotFound(err) {
			// Object was deleted — refresh the intended flag from the
			// remaining AuthProvider CRs so the auth system can transition
			// back to disabled when the last one is gone.
			r.refreshIntended(ctx)

			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	original := ap.DeepCopy()

	outcome := r.reconcile(ctx, ap)
	controller.FinalizeStatus(ctx, r.Client, r.EventRecorder, original, ap, outcome,
		controller.FinalizeStatusOptions{SuccessMessage: "AuthProvider reconciled successfully"})

	return controller.HandleReconcileResult(outcome.Result, outcome.Err)
}

// reconcile runs the AuthProvider reconciliation pipeline.
func (r *Reconciler) reconcile(
	ctx context.Context, ap *renovatev1beta1.AuthProvider,
) controller.Outcome {
	// List all AuthProvider CRs to determine if auth is intended
	var authProviderList renovatev1beta1.AuthProviderList
	if err := r.List(ctx, &authProviderList); err != nil {
		return controller.Outcome{Err: fmt.Errorf("failed to list AuthProviders: %w", err)}
	}

	r.AuthManager.SetIntended(len(authProviderList.Items) > 0)

	// Handle deletion
	if !ap.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(ap, FinalizerCleanup) {
			// Unregister the provider
			r.AuthManager.Unregister(ap.Name)
			ap.Status.Registered = false

			// Remove the finalizer
			patch := client.MergeFrom(ap.DeepCopy())
			controllerutil.RemoveFinalizer(ap, FinalizerCleanup)

			if err := r.Patch(ctx, ap, patch); err != nil && !api_errors.IsNotFound(err) {
				return controller.Outcome{Err: fmt.Errorf("failed to remove finalizer: %w", err)}
			}
		}

		return controller.Outcome{Result: &ctrl.Result{}}
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(ap, FinalizerCleanup) {
		patch := client.MergeFrom(ap.DeepCopy())
		controllerutil.AddFinalizer(ap, FinalizerCleanup)

		if err := r.Patch(ctx, ap, patch); err != nil {
			return controller.Outcome{Err: fmt.Errorf("failed to add finalizer: %w", err)}
		}

		return controller.Outcome{Result: &ctrl.Result{}}
	}

	// Skip the expensive provider construction (which performs a network OIDC
	// discovery round-trip) and re-registration when the spec is unchanged and
	// the provider is already registered with the auth manager. Reconciles also
	// fire on periodic resyncs, secret watches and our own status patches, so
	// without this guard the issuer would be contacted on every pass.
	if r.isProviderUpToDate(ap) {
		return controller.Outcome{Result: &ctrl.Result{}}
	}

	// Get client secret
	clientSecret, err := r.getClientSecret(ctx, ap)
	if err != nil {
		return controller.Outcome{Err: fmt.Errorf("failed to get client secret: %w", err)}
	}

	// Create auth provider
	provider, err := r.createAuthProvider(ctx, ap, clientSecret)
	if err != nil {
		return controller.Outcome{Err: fmt.Errorf("failed to create auth provider: %w", err)}
	}

	// Register with auth manager
	r.AuthManager.Register(provider)

	ap.Status.Registered = true

	return controller.Outcome{Result: &ctrl.Result{}}
}

// isProviderUpToDate reports whether the provider for ap is already registered
// for the current spec generation, so that re-construction and re-registration
// can be safely skipped.
func (r *Reconciler) isProviderUpToDate(ap *renovatev1beta1.AuthProvider) bool {
	if !ap.Status.Registered {
		return false
	}

	ready := ap.GetCondition(renovatev1beta1.ConditionReady)
	if ready == nil || ready.ObservedGeneration != ap.Generation {
		return false
	}

	_, ok := r.AuthManager.Get(ap.Name)

	return ok
}

// refreshIntended re-lists all AuthProvider CRs and updates the intended flag.
// This ensures the flag stays accurate when objects are deleted.
func (r *Reconciler) refreshIntended(ctx context.Context) {
	var authProviderList renovatev1beta1.AuthProviderList
	if err := r.List(ctx, &authProviderList); err != nil {
		return
	}

	r.AuthManager.SetIntended(len(authProviderList.Items) > 0)
}

func (r *Reconciler) getClientSecret(ctx context.Context, ap *renovatev1beta1.AuthProvider) (string, error) {
	secretName := ap.Spec.ClientSecret.Name
	secretKey := ap.Spec.ClientSecret.Key

	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{
		Name:      secretName,
		Namespace: ap.Namespace,
	}, &secret); err != nil {
		return "", fmt.Errorf("failed to get secret %s: %w", secretName, err)
	}

	secretValue, ok := secret.Data[secretKey]
	if !ok {
		return "", fmt.Errorf("%w: %s in secret %s", errSecretKeyNotFound, secretKey, secretName)
	}

	return string(secretValue), nil
}

//nolint:ireturn
func (r *Reconciler) createAuthProvider(
	ctx context.Context, ap *renovatev1beta1.AuthProvider, clientSecret string,
) (auth.AuthProvider, error) {
	forgeURL := ap.Spec.ForgeURL
	if forgeURL == "" {
		forgeURL = ap.Spec.Endpoint
	}

	cfg := auth.ProviderConfig{
		Name:         ap.Name,
		Type:         string(ap.Spec.Type),
		IssuerURL:    ap.Spec.IssuerURL,
		ClientID:     ap.Spec.ClientID,
		ClientSecret: clientSecret,
		RedirectURL:  ap.Spec.RedirectURL,
		ForgeURL:     forgeURL,
		AuthURL:      ap.Spec.AuthURL,
		Insecure:     ap.Spec.Insecure,
	}

	switch ap.Spec.Type {
	case renovatev1beta1.PlatformType_GITEA:
		return auth_gitea.NewGiteaProvider(ctx, cfg)
	default:
		return nil, fmt.Errorf("%w: %s", errUnsupportedType, ap.Spec.Type)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.EventRecorder = mgr.GetEventRecorder(ControllerName)

	return ctrl.NewControllerManagedBy(mgr).
		For(&renovatev1beta1.AuthProvider{}).
		Named(ControllerName).
		Complete(r)
}
