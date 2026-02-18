package discovery

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	v1beta1 "github.com/thegeeklab/renovate-operator/internal/webhook/v1beta1"
	corev1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Discovery Controller", func() {
	var (
		ctx                context.Context
		reconciler         *Reconciler
		typeNamespacedName types.NamespacedName
	)

	BeforeEach(func() {
		ctx = context.Background()
		reconciler = &Reconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}
	})

	Context("When reconciling a resource via ConfigRef", func() {
		const resourceName = "test-discovery-ref"

		BeforeEach(func() {
			typeNamespacedName = types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}

			config := &renovatev1beta1.RenovateConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config-ref",
					Namespace: "default",
				},
				Spec: renovatev1beta1.RenovateConfigSpec{
					Platform: renovatev1beta1.PlatformSpec{
						Type: "github",
						Token: corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "token"},
								Key:                  "key",
							},
						},
						Endpoint: "https://api.github.com/",
					},
				},
			}
			rcd := &v1beta1.RenovateConfigCustomDefaulter{}
			Expect(rcd.Default(ctx, config)).To(Succeed())
			Expect(k8sClient.Create(ctx, config)).To(Succeed())

			resource := &renovatev1beta1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: renovatev1beta1.DiscoverySpec{
					ConfigRef: "test-config-ref",
					JobSpec: renovatev1beta1.JobSpec{
						Schedule: renovatev1beta1.DefaultSchedule,
					},
					Filter: []string{"org/repo1", "org/repo2"},
				},
			}
			dd := &v1beta1.DiscoveryCustomDefaulter{}
			Expect(dd.Default(ctx, resource)).To(Succeed())
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			resource := &renovatev1beta1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
			}
			_ = k8sClient.Delete(ctx, resource)

			config := &renovatev1beta1.RenovateConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config-ref",
					Namespace: "default",
				},
			}
			_ = k8sClient.Delete(ctx, config)
		})

		It("should successfully reconcile the resource", func() {
			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))
		})
	})

	Context("When reconciling via Labels", func() {
		const (
			discoveryName = "test-discovery-label"
			configName    = "test-config-label"
			labelValue    = "renovator-01"
		)

		BeforeEach(func() {
			config := &renovatev1beta1.RenovateConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configName,
					Namespace: "default",
					Labels: map[string]string{
						renovatev1beta1.RenovatorLabel: labelValue,
					},
				},
				Spec: renovatev1beta1.RenovateConfigSpec{
					Platform: renovatev1beta1.PlatformSpec{
						Type: "github",
						Token: corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "token"},
								Key:                  "key",
							},
						},
					},
				},
			}
			rcd := &v1beta1.RenovateConfigCustomDefaulter{}
			Expect(rcd.Default(ctx, config)).To(Succeed())
			Expect(k8sClient.Create(ctx, config)).To(Succeed())

			resource := &renovatev1beta1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      discoveryName,
					Namespace: "default",
					Labels: map[string]string{
						renovatev1beta1.RenovatorLabel: labelValue,
					},
				},
				Spec: renovatev1beta1.DiscoverySpec{
					JobSpec: renovatev1beta1.JobSpec{
						Schedule: renovatev1beta1.DefaultSchedule,
					},
					Filter: []string{"org/repo1"},
				},
			}
			dd := &v1beta1.DiscoveryCustomDefaulter{}
			Expect(dd.Default(ctx, resource)).To(Succeed())
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			resource := &renovatev1beta1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      discoveryName,
					Namespace: "default",
				},
			}
			_ = k8sClient.Delete(ctx, resource)

			config := &renovatev1beta1.RenovateConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configName,
					Namespace: "default",
				},
			}
			_ = k8sClient.Delete(ctx, config)
		})

		It("should resolve RenovateConfig via labels", func() {
			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: discoveryName, Namespace: "default"},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))
		})
	})

	It("should handle missing RenovateConfig resource gracefully", func() {
		mockClient := &mockErrorClient{Client: k8sClient}
		errorReconciler := &Reconciler{Client: mockClient, Scheme: k8sClient.Scheme()}

		result, err := errorReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: "missing-config-discovery", Namespace: "default"},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(reconcile.Result{}))
	})

	It("should handle missing Discovery resource gracefully", func() {
		nonExistentName := types.NamespacedName{
			Name:      "non-existent-discovery",
			Namespace: "default",
		}

		result, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: nonExistentName,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(reconcile.Result{}))
	})
})

// mockErrorClient is a mock client that returns errors for testing.
type mockErrorClient struct {
	client.Client
}

func (m *mockErrorClient) Get(
	ctx context.Context,
	key client.ObjectKey,
	obj client.Object,
	opts ...client.GetOption,
) error {
	// Return error for RenovateConfig to simulate missing config
	if _, ok := obj.(*renovatev1beta1.RenovateConfig); ok {
		return api_errors.NewNotFound(renovatev1beta1.GroupVersion.WithResource("renovateconfigs").GroupResource(), key.Name)
	}

	return m.Client.Get(ctx, key, obj, opts...)
}
