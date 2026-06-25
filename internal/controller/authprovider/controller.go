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
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/controller"
	"github.com/thegeeklab/renovate-operator/internal/frontend/auth"
	auth_gitea "github.com/thegeeklab/renovate-operator/internal/frontend/auth/gitea"
)

const (
	ControllerName = "authprovider"

	//nolint:gosec // G101: This is a field path for indexing, not a credential
	secretRefIndexKey = ".spec.clientSecret.name"
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
	r.refreshIntended(ctx)

	if !ap.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(ap, renovatev1beta1.FinalizerAuthProviderCleanup) {
			r.AuthManager.Unregister(ap.Name)
			ap.Status.Registered = false

			patch := client.MergeFrom(ap.DeepCopy())
			controllerutil.RemoveFinalizer(ap, renovatev1beta1.FinalizerAuthProviderCleanup)

			if err := r.Patch(ctx, ap, patch); err != nil && !api_errors.IsNotFound(err) {
				return controller.Outcome{Err: fmt.Errorf("failed to remove finalizer: %w", err)}
			}
		}

		return controller.Outcome{Result: &ctrl.Result{}}
	}

	if !controllerutil.ContainsFinalizer(ap, renovatev1beta1.FinalizerAuthProviderCleanup) {
		patch := client.MergeFrom(ap.DeepCopy())
		controllerutil.AddFinalizer(ap, renovatev1beta1.FinalizerAuthProviderCleanup)

		if err := r.Patch(ctx, ap, patch); err != nil {
			return controller.Outcome{Err: fmt.Errorf("failed to add finalizer: %w", err)}
		}

		return controller.Outcome{Result: &ctrl.Result{}}
	}

	secret, err := r.getSecret(ctx, ap)
	if err != nil {
		r.AuthManager.Unregister(ap.Name)
		ap.Status.Registered = false

		return controller.Outcome{Err: fmt.Errorf("failed to get secret: %w", err)}
	}

	if r.isProviderUpToDate(ap, secret) {
		return controller.Outcome{Result: &ctrl.Result{}}
	}

	clientSecret, err := r.extractClientSecret(ap, secret)
	if err != nil {
		r.AuthManager.Unregister(ap.Name)
		ap.Status.Registered = false

		return controller.Outcome{Err: fmt.Errorf("failed to extract client secret: %w", err)}
	}

	provider, err := r.createAuthProvider(ctx, ap, clientSecret)
	if err != nil {
		r.AuthManager.Unregister(ap.Name)
		ap.Status.Registered = false

		return controller.Outcome{Err: fmt.Errorf("failed to create auth provider: %w", err)}
	}

	r.AuthManager.Register(provider)

	ap.Status.Registered = true
	ap.Status.SecretResourceVersion = secret.ResourceVersion

	return controller.Outcome{Result: &ctrl.Result{}}
}

func (r *Reconciler) isProviderUpToDate(ap *renovatev1beta1.AuthProvider, secret *corev1.Secret) bool {
	if !ap.Status.Registered {
		return false
	}

	ready := ap.GetCondition(renovatev1beta1.ConditionReady)
	if ready == nil || ready.ObservedGeneration != ap.Generation {
		return false
	}

	_, ok := r.AuthManager.Get(ap.Name)
	if !ok {
		return false
	}

	return ap.Status.SecretResourceVersion == secret.ResourceVersion
}

func (r *Reconciler) refreshIntended(ctx context.Context) {
	var authProviderList renovatev1beta1.AuthProviderList
	if err := r.List(ctx, &authProviderList); err != nil {
		return
	}

	r.AuthManager.SetIntended(len(authProviderList.Items) > 0)
}

func (r *Reconciler) getSecret(ctx context.Context, ap *renovatev1beta1.AuthProvider) (*corev1.Secret, error) {
	secretName := ap.Spec.ClientSecret.Name

	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{
		Name:      secretName,
		Namespace: ap.Namespace,
	}, &secret); err != nil {
		return nil, fmt.Errorf("failed to get secret %s: %w", secretName, err)
	}

	return &secret, nil
}

func (r *Reconciler) extractClientSecret(ap *renovatev1beta1.AuthProvider, secret *corev1.Secret) (string, error) {
	secretKey := ap.Spec.ClientSecret.Key

	secretValue, ok := secret.Data[secretKey]
	if !ok {
		return "", fmt.Errorf("%w: %s in secret %s", errSecretKeyNotFound, secretKey, secret.Name)
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
		DisplayName:  ap.Spec.DisplayName,
		Type:         string(ap.Spec.Type),
		Endpoint:     ap.Spec.Endpoint,
		ClientID:     ap.Spec.ClientID,
		ClientSecret: clientSecret,
		RedirectURL:  ap.Spec.RedirectURL,
		ForgeURL:     forgeURL,
		AuthURL:      ap.Spec.AuthURL,
		IconURL:      ap.Spec.IconURL,
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

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(), &renovatev1beta1.AuthProvider{}, secretRefIndexKey, authProviderSecretRefIndexFn,
	); err != nil {
		return err
	}

	secretPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return r.isSecretReferenced(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return r.isSecretReferenced(e.ObjectNew)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return r.isSecretReferenced(e.Object)
		},
		GenericFunc: func(_ event.GenericEvent) bool {
			return false
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&renovatev1beta1.AuthProvider{}).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.mapSecretWithAuthProvider),
			builder.WithPredicates(secretPredicate),
		).
		Named(ControllerName).
		Complete(r)
}

// authProviderSecretRefIndexFn returns the secret name reference for indexing.
func authProviderSecretRefIndexFn(rawObj client.Object) []string {
	authProvider, ok := rawObj.(*renovatev1beta1.AuthProvider)
	if !ok {
		return nil
	}

	if authProvider.Spec.ClientSecret.Name == "" {
		return nil
	}

	return []string{authProvider.Spec.ClientSecret.Name}
}

// listAuthProvidersForSecret returns AuthProviders that reference the given secret.
func (r *Reconciler) listAuthProvidersForSecret(
	ctx context.Context, secret client.Object,
) (*renovatev1beta1.AuthProviderList, error) {
	authProviderList := &renovatev1beta1.AuthProviderList{}
	if err := r.List(
		ctx, authProviderList,
		client.InNamespace(secret.GetNamespace()),
		client.MatchingFields{secretRefIndexKey: secret.GetName()},
	); err != nil {
		return nil, err
	}

	return authProviderList, nil
}

// isSecretReferenced checks if a secret is referenced by any AuthProvider.
func (r *Reconciler) isSecretReferenced(secret client.Object) bool {
	list, err := r.listAuthProvidersForSecret(context.Background(), secret)
	if err != nil {
		return false
	}

	return len(list.Items) > 0
}

// mapSecretWithAuthProvider maps a Secret event to a Request for the AuthProvider(s) that reference it.
func (r *Reconciler) mapSecretWithAuthProvider(ctx context.Context, obj client.Object) []ctrl.Request {
	list, err := r.listAuthProvidersForSecret(ctx, obj)
	if err != nil {
		return nil
	}

	reqs := make([]ctrl.Request, len(list.Items))
	for i := range list.Items {
		reqs[i] = ctrl.Request{
			NamespacedName: client.ObjectKey{
				Name:      list.Items[i].Name,
				Namespace: list.Items[i].Namespace,
			},
		}
	}

	return reqs
}
