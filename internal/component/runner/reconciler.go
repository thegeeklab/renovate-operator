package runner

import (
	"context"
	"errors"
	"fmt"
	"math"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/util/reconciler"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ErrMaxRepoCount = errors.New("max repo count reached")

type Reconciler struct {
	client.Client
	scheme     *runtime.Scheme
	req        ctrl.Request
	instance   *renovatev1beta1.Runner
	renovate   *renovatev1beta1.RenovateConfig
	index      []JobData
	indexCount int32
}

type JobData struct {
	Repositories []string `json:"repositories"`
}

func NewReconciler(
	ctx context.Context,
	c client.Client,
	scheme *runtime.Scheme,
	instance *renovatev1beta1.Runner,
	renovate *renovatev1beta1.RenovateConfig,
) (*Reconciler, error) {
	r := &Reconciler{
		Client:   c,
		scheme:   scheme,
		req:      ctrl.Request{NamespacedName: client.ObjectKey{Namespace: instance.Namespace, Name: instance.Name}},
		instance: instance,
		renovate: renovate,
	}

	gitRepoList := &renovatev1beta1.GitRepoList{}
	if err := r.List(ctx, gitRepoList, client.InNamespace(r.req.Namespace)); err != nil {
		return nil, err
	}

	maxRepos := len(gitRepoList.Items)
	if maxRepos > math.MaxInt32 {
		return nil, fmt.Errorf("%w: %d", ErrMaxRepoCount, maxRepos)
	}

	r.indexCount = int32(maxRepos)

	index := make([]JobData, r.indexCount)
	for i, repo := range gitRepoList.Items {
		index[i] = JobData{
			Repositories: []string{repo.Name},
		}
	}

	r.index = index

	return r, nil
}

func (r *Reconciler) Reconcile(ctx context.Context) (*ctrl.Result, error) {
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
