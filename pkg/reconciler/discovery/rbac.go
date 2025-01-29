package discovery

import (
	"context"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/equality"
	"github.com/thegeeklab/renovate-operator/pkg/metadata"
	rbacv1 "k8s.io/api/rbac/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *discoveryReconciler) reconcileRole(ctx context.Context) (*ctrl.Result, error) {
	expected, err := r.createRole()
	if err != nil {
		return &ctrl.Result{}, err
	}

	return r.ReconcileResource(ctx, &rbacv1.Role{}, expected, equality.RoleEqual)
}

func (r *discoveryReconciler) reconcileRoleBinding(ctx context.Context) (*ctrl.Result, error) {
	expected, err := r.createRoleBinding()
	if err != nil {
		return &ctrl.Result{}, err
	}

	return r.ReconcileResource(ctx, &rbacv1.RoleBinding{}, expected, equality.RoleBindingEqual)
}

func (r *discoveryReconciler) createRole() (*rbacv1.Role, error) {
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
				Resources: []string{"renovators/status"},
				Verbs:     []string{"get", "patch"},
			},
		},
	}
	if err := controllerutil.SetControllerReference(r.instance, role, r.Scheme); err != nil {
		return nil, err
	}

	return role, nil
}

func (r *discoveryReconciler) createRoleBinding() (*rbacv1.RoleBinding, error) {
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
	if err := controllerutil.SetControllerReference(r.instance, roleBinding, r.Scheme); err != nil {
		return nil, err
	}

	return roleBinding, nil
}
