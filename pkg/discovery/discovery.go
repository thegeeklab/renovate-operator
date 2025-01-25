package discovery

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

type Discovery struct {
	client   client.Client
	req      ctrl.Request
	instance v1beta1.Renovator
	scheme   *runtime.Scheme
	Batches  []Batch
}

func New(client client.Client, req ctrl.Request, instance v1beta1.Renovator, scheme *runtime.Scheme) *Discovery {
	d := &Discovery{
		client:   client,
		req:      req,
		scheme:   scheme,
		instance: instance,
	}

	return d
}

func (d *Discovery) Reconcile(ctx context.Context) (*ctrl.Result, error) {
	resutl, err := d.reconcileDiscovery(ctx)
	if err != nil {
		return resutl, err
	}

	return &ctrl.Result{}, nil
}
