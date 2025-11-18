package runner

import (
	"context"
	"math"

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
	batches        []Batch
}

type Batch struct {
	Repositories []string `json:"repositories"`
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
	ctx context.Context,
	c client.Client,
	scheme *runtime.Scheme,
	instance *renovatev1beta1.Renovator,
) (*Reconciler, error) {
	r := &Reconciler{
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
	}

	batches, err := r.createBatches(ctx)
	if err != nil {
		return nil, err
	}

	r.batches = batches

	return r, nil
}

func (r *Reconciler) Reconcile(ctx context.Context, res *renovatev1beta1.Renovator) (*ctrl.Result, error) {
	results := &reconciler.Results{}

	reconcileFuncs := []func(context.Context) (*ctrl.Result, error){
		r.reconcileConfigMap,
		r.reconcileCronJob,
	}

	for _, reconcileFunc := range reconcileFuncs {
		result, err := reconcileFunc(ctx)
		if err != nil {
			return result, err
		}

		results.Collect(result)
	}

	return results.ToResult(), nil
}

func (r *Reconciler) listRepositories(ctx context.Context) ([]string, error) {
	var gitRepoList renovatev1beta1.GitRepoList

	repos := make([]string, 0)

	if err := r.List(ctx, &gitRepoList, client.InNamespace(r.req.Namespace)); err != nil {
		return nil, err
	}

	for i, repo := range gitRepoList.Items {
		repos[i] = repo.Spec.Name
	}

	return repos, nil
}

func (r *Reconciler) createBatches(ctx context.Context) ([]Batch, error) {
	batches := make([]Batch, 0)

	repos, err := r.listRepositories(ctx)
	if err != nil {
		return nil, err
	}

	repoCount := len(repos)
	if repoCount == 0 {
		// No repositories to process, return empty batch list
		return batches, nil
	}

	switch r.instance.Spec.Runner.Strategy {
	case renovatev1beta1.RunnerStrategy_BATCH:
		batchSize := r.calculateOptimalBatchSize(repoCount)

		for i := 0; i < repoCount; i += batchSize {
			end := int(math.Min(float64(i+batchSize), float64(repoCount)))
			batch := repos[i:end]
			batches = append(batches, Batch{Repositories: batch})
		}

		return batches, nil

	case renovatev1beta1.RunnerStrategy_NONE:
		fallthrough
	default:
		// NONE strategy and unknown strategies: process all repositories in a single batch
		return []Batch{{Repositories: repos}}, nil
	}
}

// calculateOptimalBatchSize determines the best batch size based on configuration and repository count.
func (r *Reconciler) calculateOptimalBatchSize(repoCount int) int {
	const (
		instanceMultiplier = 3  // Aim for 2-3 batches per instance
		maxBatchSize       = 50 // Cap at 50 repositories per batch
	)

	// If BatchSize is explicitly set, use it
	if r.instance.Spec.Runner.BatchSize > 0 {
		return r.instance.Spec.Runner.BatchSize
	}

	// Calculate optimal batch size based on instances and repository count
	instances := int(r.instance.Spec.Runner.Instances)
	if instances <= 0 {
		instances = 1
	}

	targetBatches := instances * instanceMultiplier
	optimalBatchSize := repoCount / targetBatches

	// Ensure batch size is within reasonable bounds
	if optimalBatchSize < 1 {
		optimalBatchSize = 1
	} else if optimalBatchSize > maxBatchSize {
		optimalBatchSize = maxBatchSize
	}

	return optimalBatchSize
}
