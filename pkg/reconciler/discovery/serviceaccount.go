package discovery

import (
	"context"

	"github.com/thegeeklab/renovate-operator/pkg/equality"
	"github.com/thegeeklab/renovate-operator/pkg/metadata"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *discoveryReconciler) reconcileServiceAccount(ctx context.Context) (*ctrl.Result, error) {
	expected, err := r.createServiceAccount()
	if err != nil {
		return &ctrl.Result{}, err
	}

	return r.ReconcileResource(ctx, &corev1.ServiceAccount{}, expected, equality.ServiceAccountEqual)
}

func (r *discoveryReconciler) createServiceAccount() (*corev1.ServiceAccount, error) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metadata.GenericMetaData(r.Req),
	}

	if err := controllerutil.SetControllerReference(r.instance, sa, r.Scheme); err != nil {
		return nil, err
	}

	return sa, nil
}
