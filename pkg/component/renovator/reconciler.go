package renovator

import (
	"context"
	"fmt"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/util/reconciler"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Reconciler struct {
	client.Client
	scheme         *runtime.Scheme
	req            ctrl.Request
	instance       *renovatev1beta1.Renovator
	renovateConfig *Renovate
}

type Renovate struct {
	Onboarding    bool                         `json:"onboarding"`
	PrHourlyLimit int                          `json:"prHourlyLimit"`
	DryRun        renovatev1beta1.DryRun       `json:"dryRun"`
	Platform      renovatev1beta1.PlatformType `json:"platform"`
	Endpoint      string                       `json:"endpoint"`
	AddLabels     []string                     `json:"addLabels,omitempty"`
	Repositories  []string                     `json:"repositories,omitempty"`
}

func NewReconciler(
	_ context.Context,
	c client.Client,
	scheme *runtime.Scheme,
	instance *renovatev1beta1.Renovator,
) (*Reconciler, error) {
	return &Reconciler{
		Client:   c,
		scheme:   scheme,
		req:      ctrl.Request{NamespacedName: client.ObjectKey{Namespace: instance.Namespace, Name: instance.Name}},
		instance: instance,
		renovateConfig: &Renovate{
			Onboarding:    instance.Spec.Renovate.Onboarding != nil && *instance.Spec.Renovate.Onboarding,
			PrHourlyLimit: instance.Spec.Renovate.PrHourlyLimit,
			DryRun:        instance.Spec.Renovate.DryRun,
			Platform:      instance.Spec.Renovate.Platform.Type,
			Endpoint:      instance.Spec.Renovate.Platform.Endpoint,
			AddLabels:     instance.Spec.Renovate.AddLabels,
		},
	}, nil
}

func (r *Reconciler) Reconcile(ctx context.Context, instance *renovatev1beta1.Renovator) (*ctrl.Result, error) {
	// Update the instance reference to ensure we're working with the latest version
	r.instance = instance

	results := &reconciler.Results{}

	// Define the reconciliation order
	reconcileFuncs := []func(context.Context) (*ctrl.Result, error){
		r.reconcileConfigMap,
		r.reconcileDiscovery,
	}

	// Execute each reconciliation step
	for _, reconcileFunc := range reconcileFuncs {
		res, err := reconcileFunc(ctx)
		if err != nil {
			return &ctrl.Result{Requeue: true}, fmt.Errorf("reconciliation failed: %w", err)
		}

		results.Collect(res)
	}

	return results.ToResult(), nil
}
