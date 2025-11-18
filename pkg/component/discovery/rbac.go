package discovery

import (
	"context"
	"fmt"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/metadata"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *Reconciler) reconcileServiceAccount(ctx context.Context) (*ctrl.Result, error) {
	sa := &corev1.ServiceAccount{ObjectMeta: metadata.GenericMetaData(r.req)}

	_, err := k8s.CreateOrPatch(ctx, r.Client, sa, r.instance, func() error {
		return r.updateServiceAccount(sa)
	})
	if err != nil {
		return &ctrl.Result{Requeue: true}, fmt.Errorf("failed to create or update service account: %w", err)
	}

	return &ctrl.Result{}, nil
}

func (r *Reconciler) updateServiceAccount(_ *corev1.ServiceAccount) error {
	return nil
}

func (r *Reconciler) reconcileRole(ctx context.Context) (*ctrl.Result, error) {
	role := &rbacv1.Role{ObjectMeta: metadata.GenericMetaData(r.req)}

	_, err := k8s.CreateOrPatch(ctx, r.Client, role, r.instance, func() error {
		return r.updateRole(role)
	})
	if err != nil {
		return &ctrl.Result{Requeue: true}, fmt.Errorf("failed to create or update role: %w", err)
	}

	return &ctrl.Result{}, nil
}

func (r *Reconciler) updateRole(role *rbacv1.Role) error {
	// Apply least privilege principle
	role.Rules = []rbacv1.PolicyRule{
		{
			// Allow reading the renovator instance
			APIGroups:     []string{renovatev1beta1.GroupVersion.Group},
			Resources:     []string{"renovators"},
			ResourceNames: []string{r.instance.Name},
			Verbs:         []string{"get"},
		},
		{
			// Allow managing gitrepos for this instance
			APIGroups: []string{renovatev1beta1.GroupVersion.Group},
			Resources: []string{"gitrepos"},
			Verbs:     []string{"get", "list", "create", "update", "patch", "delete"},
		},
	}

	return nil
}

func (r *Reconciler) reconcileRoleBinding(ctx context.Context) (*ctrl.Result, error) {
	rb := &rbacv1.RoleBinding{ObjectMeta: metadata.GenericMetaData(r.req)}

	_, err := k8s.CreateOrPatch(ctx, r.Client, rb, r.instance, func() error {
		return r.updateRoleBinding(rb)
	})
	if err != nil {
		return &ctrl.Result{Requeue: true}, fmt.Errorf("failed to create or update role binding: %w", err)
	}

	return &ctrl.Result{}, nil
}

func (r *Reconciler) updateRoleBinding(rb *rbacv1.RoleBinding) error {
	rb.Subjects = []rbacv1.Subject{
		{
			Kind:      "ServiceAccount",
			Name:      metadata.GenericMetaData(r.req).Name,
			Namespace: r.req.Namespace,
		},
	}
	rb.RoleRef = rbacv1.RoleRef{
		APIGroup: "rbac.authorization.k8s.io",
		Kind:     "Role",
		Name:     metadata.GenericMetaData(r.req).Name,
	}

	return nil
}
