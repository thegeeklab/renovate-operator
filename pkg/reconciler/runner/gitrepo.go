package runner

import (
	"context"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *runnerReconciler) ListRepositories(ctx context.Context) ([]string, error) {
	var gitRepoList renovatev1beta1.GitRepoList

	repos := make([]string, 0)

	if err := r.KubeClient.List(ctx, &gitRepoList, client.InNamespace(r.Req.Namespace)); err != nil {
		return nil, err
	}

	for _, repo := range gitRepoList.Items {
		repos = append(repos, repo.Spec.Name)
	}

	return repos, nil
}

func (r *runnerReconciler) CreateBatches(ctx context.Context) ([]util.Batch, error) {
	var batches []util.Batch

	repos, err := r.ListRepositories(ctx)
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
			end := min(i+batchSize, repoCount)
			batch := repos[i:end]
			batches = append(batches, util.Batch{Repositories: batch})
		}
	case renovatev1beta1.RunnerStrategy_NONE:
		fallthrough
	default:
		// NONE strategy and unknown strategies: process all repositories in a single batch
		batches = append(batches, util.Batch{Repositories: repos})
	}

	return batches, nil
}

// calculateOptimalBatchSize determines the best batch size based on configuration and repository count
func (r *runnerReconciler) calculateOptimalBatchSize(repoCount int) int {
	// If BatchSize is explicitly set, use it
	if r.instance.Spec.Runner.BatchSize > 0 {
		return r.instance.Spec.Runner.BatchSize
	}

	// Calculate optimal batch size based on instances and repository count
	instances := int(r.instance.Spec.Runner.Instances)
	if instances <= 0 {
		instances = 1
	}

	// Aim for 2-3 batches per instance to allow for good parallelization
	// while keeping batch sizes reasonable
	targetBatches := instances * 3
	optimalBatchSize := repoCount / targetBatches

	// Ensure batch size is within reasonable bounds
	if optimalBatchSize < 1 {
		optimalBatchSize = 1
	} else if optimalBatchSize > 50 {
		// Cap at 50 repositories per batch to avoid excessively long-running jobs
		optimalBatchSize = 50
	}

	return optimalBatchSize
}
