package worker

import (
	"context"

	"github.com/thegeeklab/renovate-operator/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Batch struct {
	Repositories []string `json:"repos"`
}

type Worker struct {
	client        client.Client
	req           ctrl.Request
	renovatorInst *v1beta1.Renovator
	discoveryInst *v1beta1.Discovery
	scheme        *runtime.Scheme
	Batches       []Batch
}

func New(
	client client.Client,
	req ctrl.Request,
	ri *v1beta1.Renovator,
	di *v1beta1.Discovery,
	scheme *runtime.Scheme,
) *Worker {
	w := &Worker{
		client:        client,
		req:           req,
		scheme:        scheme,
		renovatorInst: ri,
		discoveryInst: di,
	}
	repos := w.discoveryInst.Status.Repositories

	var batches []Batch

	switch w.renovatorInst.Spec.Worker.Strategy {
	case v1beta1.WorkerStrategy_SIZE:
		limit := w.renovatorInst.Spec.Worker.Size
		for i := 0; i < len(repos); i += limit {
			rb := repos[i:min(i+limit, len(repos))]
			batches = append(batches, Batch{Repositories: rb})
		}
	case v1beta1.WorkerStrategy_NONE:
	default:
		batches = append(batches, Batch{Repositories: repos})
	}

	w.Batches = batches

	return w
}

func (w *Worker) Reconcile(ctx context.Context) (*ctrl.Result, error) {
	resutl, err := w.reconcileConfig(ctx)
	if err != nil {
		return resutl, err
	}

	return &ctrl.Result{}, nil
}
