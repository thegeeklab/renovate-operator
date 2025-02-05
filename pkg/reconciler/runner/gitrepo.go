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

	switch r.instance.Spec.Runner.Strategy {
	case renovatev1beta1.RunnerStrategy_BATCH:
		limit := r.instance.Spec.Runner.BatchSize
		for i := 0; i < len(repos); i += limit {
			batch := repos[i:min(i+limit, len(repos))]
			batches = append(batches, util.Batch{Repositories: batch})
		}
	case renovatev1beta1.RunnerStrategy_NONE:
	default:
		batches = append(batches, util.Batch{Repositories: repos})
	}

	return batches, nil
}
