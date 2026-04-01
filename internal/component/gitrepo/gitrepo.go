package gitrepo

import (
	"context"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// reconcileGitRepo manages the finalizer for the GitRepo resource.
func (r *Reconciler) reconcileGitRepo(ctx context.Context) (*ctrl.Result, error) {
	if !r.instance.DeletionTimestamp.IsZero() {
		if r.instance.Status.WebhookID == "" &&
			controllerutil.ContainsFinalizer(r.instance, renovatev1beta1.FinalizerGitRepoWebhook) {
			_, err := k8s.CreateOrUpdate(ctx, r.Client, r.instance, r.renovate, func() error {
				controllerutil.RemoveFinalizer(r.instance, renovatev1beta1.FinalizerGitRepoWebhook)

				return nil
			})
			if err != nil {
				return &ctrl.Result{}, err
			}
		}

		return &ctrl.Result{}, nil
	}

	_, err := k8s.CreateOrUpdate(ctx, r.Client, r.instance, r.renovate, func() error {
		if !controllerutil.ContainsFinalizer(r.instance, renovatev1beta1.FinalizerGitRepoWebhook) {
			controllerutil.AddFinalizer(r.instance, renovatev1beta1.FinalizerGitRepoWebhook)
		}

		return nil
	})

	return &ctrl.Result{}, err
}
