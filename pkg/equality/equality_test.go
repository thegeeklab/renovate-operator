package equality

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Equality", func() {
	Context("ServiceAccountEqual", func() {
		It("should return true when labels match", func() {
			a := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
			}
			b := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
			}
			Expect(ServiceAccountEqual(a, b)).To(BeTrue())
		})

		It("should return false when labels differ", func() {
			a := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test1"},
				},
			}
			b := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test2"},
				},
			}
			Expect(ServiceAccountEqual(a, b)).To(BeFalse())
		})

		It("should return false when comparing with different object type", func() {
			a := &corev1.ServiceAccount{}
			b := &corev1.ConfigMap{}
			Expect(ServiceAccountEqual(a, b)).To(BeFalse())
		})
	})

	Context("ConfigMapEqual", func() {
		It("should return true when data matches", func() {
			a := &corev1.ConfigMap{
				Data: map[string]string{"key": "value"},
			}
			b := &corev1.ConfigMap{
				Data: map[string]string{"key": "value"},
			}
			Expect(ConfigMapEqual(a, b)).To(BeTrue())
		})

		It("should return false when data differs", func() {
			a := &corev1.ConfigMap{
				Data: map[string]string{"key": "value1"},
			}
			b := &corev1.ConfigMap{
				Data: map[string]string{"key": "value2"},
			}
			Expect(ConfigMapEqual(a, b)).To(BeFalse())
		})
	})

	Context("RoleEqual", func() {
		It("should return true when labels and rules match", func() {
			a := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Rules: []rbacv1.PolicyRule{{
					Verbs:     []string{"get"},
					Resources: []string{"pods"},
				}},
			}
			b := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Rules: []rbacv1.PolicyRule{{
					Verbs:     []string{"get"},
					Resources: []string{"pods"},
				}},
			}
			Expect(RoleEqual(a, b)).To(BeTrue())
		})
	})

	Context("RoleBindingEqual", func() {
		It("should return true when all fields match", func() {
			a := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				RoleRef: rbacv1.RoleRef{
					Kind: "Role",
					Name: "test-role",
				},
				Subjects: []rbacv1.Subject{{
					Kind: "ServiceAccount",
					Name: "test-sa",
				}},
			}
			b := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				RoleRef: rbacv1.RoleRef{
					Kind: "Role",
					Name: "test-role",
				},
				Subjects: []rbacv1.Subject{{
					Kind: "ServiceAccount",
					Name: "test-sa",
				}},
			}
			Expect(RoleBindingEqual(a, b)).To(BeTrue())
		})
	})

	Context("CronJobEqual", func() {
		It("should return true when labels and spec match", func() {
			schedule := "*/5 * * * *"
			a := &batchv1.CronJob{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Spec: batchv1.CronJobSpec{
					Schedule: schedule,
				},
			}
			b := &batchv1.CronJob{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Spec: batchv1.CronJobSpec{
					Schedule: schedule,
				},
			}
			Expect(CronJobEqual(a, b)).To(BeTrue())
		})

		It("should return false when comparing with different object type", func() {
			a := &batchv1.CronJob{}
			b := &corev1.Pod{}
			Expect(CronJobEqual(a, b)).To(BeFalse())
		})
	})
})
