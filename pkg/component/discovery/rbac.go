package discovery

import (
	"context"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/metadata"
	rbacv1 "k8s.io/api/rbac/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *Reconciler) reconcileRole(ctx context.Context) (*ctrl.Result, error) {
	ctxLogger := logf.FromContext(ctx)

	obj, err := r.createRole()
	if err != nil {
		return &ctrl.Result{}, err
	}

	op, err := ctrl.CreateOrUpdate(ctx, r.Client, obj, nil)
	if err != nil {
		return &ctrl.Result{}, err
	}

	ctxLogger.V(1).Info("Discovery RBAC Role", "object", client.ObjectKeyFromObject(obj), "operation", op)

	return &ctrl.Result{}, nil
}

func (r *Reconciler) reconcileRoleBinding(ctx context.Context) (*ctrl.Result, error) {
	ctxLogger := logf.FromContext(ctx)

	obj, err := r.createRoleBinding()
	if err != nil {
		return &ctrl.Result{}, err
	}

	op, err := ctrl.CreateOrUpdate(ctx, r.Client, obj, nil)
	if err != nil {
		return &ctrl.Result{}, err
	}

	ctxLogger.V(1).Info("Discovery RBAC RoleBinding", "object", client.ObjectKeyFromObject(obj), "operation", op)

	return &ctrl.Result{}, nil
}

func (r *Reconciler) createRole() (*rbacv1.Role, error) {
	role := &rbacv1.Role{
		ObjectMeta: metadata.GenericMetaData(r.Req),
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{renovatev1beta1.GroupVersion.Group},
				Resources: []string{"renovators"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups: []string{renovatev1beta1.GroupVersion.Group},
				Resources: []string{"gitrepos"},
				Verbs:     []string{"get", "list", "create", "update", "patch", "delete"},
			},
		},
	}
	if err := controllerutil.SetControllerReference(r.Instance, role, r.Scheme); err != nil {
		return nil, err
	}

	return role, nil
}

func (r *Reconciler) createRoleBinding() (*rbacv1.RoleBinding, error) {
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metadata.GenericMetaData(r.Req),
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      metadata.GenericMetaData(r.Req).Name,
				Namespace: r.Req.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     metadata.GenericMetaData(r.Req).Name,
		},
	}
	if err := controllerutil.SetControllerReference(r.Instance, roleBinding, r.Scheme); err != nil {
		return nil, err
	}

	return roleBinding, nil
}
