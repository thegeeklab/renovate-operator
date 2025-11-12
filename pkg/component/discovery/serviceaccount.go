package discovery

import (
	"context"

	"github.com/thegeeklab/renovate-operator/pkg/metadata"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *Reconciler) reconcileServiceAccount(ctx context.Context) (*ctrl.Result, error) {
	ctxLogger := logf.FromContext(ctx)

	obj, err := r.createServiceAccount()
	if err != nil {
		return &ctrl.Result{}, err
	}

	op, err := ctrl.CreateOrUpdate(ctx, r.Client, obj, nil)
	if err != nil {
		return &ctrl.Result{}, err
	}

	ctxLogger.V(1).Info("Discovery RBAC ServiceAccount", "object", client.ObjectKeyFromObject(obj), "operation", op)

	return &ctrl.Result{}, nil
}

func (r *Reconciler) createServiceAccount() (*corev1.ServiceAccount, error) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metadata.GenericMetaData(r.Req),
	}

	if err := controllerutil.SetControllerReference(r.Instance, sa, r.Scheme); err != nil {
		return nil, err
	}

	return sa, nil
}
