package equality

import (
	"reflect"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ServiceAccountEqual(a, b client.Object) bool {
	ax, ok := a.(*corev1.ServiceAccount)
	if !ok {
		return false
	}

	bx, ok := b.(*corev1.ServiceAccount)
	if !ok {
		return false
	}

	return reflect.DeepEqual(ax.Labels, bx.Labels)
}

func ConfigMapEqual(a, b client.Object) bool {
	ax, ok := a.(*corev1.ConfigMap)
	if !ok {
		return false
	}

	bx, ok := b.(*corev1.ConfigMap)
	if !ok {
		return false
	}

	return equality.Semantic.DeepDerivative(ax.Data, bx.Data)
}

func RoleEqual(a, b client.Object) bool {
	ax, ok := a.(*rbacv1.Role)
	if !ok {
		return false
	}

	bx, ok := b.(*rbacv1.Role)
	if !ok {
		return false
	}

	return reflect.DeepEqual(ax.Labels, bx.Labels) &&
		reflect.DeepEqual(ax.Rules, bx.Rules)
}

func RoleBindingEqual(a, b client.Object) bool {
	ax, ok := a.(*rbacv1.RoleBinding)
	if !ok {
		return false
	}

	bx, ok := b.(*rbacv1.RoleBinding)
	if !ok {
		return false
	}

	return reflect.DeepEqual(ax.Labels, bx.Labels) &&
		equality.Semantic.DeepDerivative(ax.RoleRef, bx.RoleRef) &&
		equality.Semantic.DeepDerivative(ax.Subjects, bx.Subjects)
}

func CronJobEqual(a, b client.Object) bool {
	ax, ok := a.(*batchv1.CronJob)
	if !ok {
		return false
	}

	bx, ok := b.(*batchv1.CronJob)
	if !ok {
		return false
	}

	return reflect.DeepEqual(ax.Labels, bx.Labels) &&
		equality.Semantic.DeepDerivative(ax.Spec, bx.Spec)
}
