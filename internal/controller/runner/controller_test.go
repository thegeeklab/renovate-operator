package runner

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

var _ = Describe("Runner Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-runner"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Runner")

			// Create RenovateConfig resource first
			config := &renovatev1beta1.RenovateConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config",
					Namespace: "default",
				},
				Spec: renovatev1beta1.RenovateConfigSpec{
					Platform: renovatev1beta1.PlatformSpec{
						Type: "github",
						Token: corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "renovate-token-secret",
								},
								Key: "token",
							},
						},
						Endpoint: "https://api.github.com/",
					},
				},
			}

			err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-config", Namespace: "default"}, config)
			if err != nil && api_errors.IsNotFound(err) {
				// Apply webhook defaulter
				rcd := &v1beta1.RenovateConfigCustomDefaulter{}
				Expect(rcd.Default(ctx, config)).To(Succeed())
				Expect(k8sClient.Create(ctx, config)).To(Succeed())
			}

			// Create Runner resource
			err = k8sClient.Get(ctx, typeNamespacedName, &renovatev1beta1.Runner{})
			if err != nil && api_errors.IsNotFound(err) {
				resource := &renovatev1beta1.Runner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: renovatev1beta1.RunnerSpec{
						ConfigRef: "test-config",
						JobSpec: renovatev1beta1.JobSpec{
							Schedule: renovatev1beta1.DefaultSchedule,
						},
						Instances: 1,
					},
				}
				rd := &v1beta1.RunnerCustomDefaulter{}
				Expect(rd.Default(ctx, resource)).To(Succeed())
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// Cleanup RenovateConfig resource
			config := &renovatev1beta1.RenovateConfig{}

			configErr := k8sClient.Get(ctx, types.NamespacedName{Name: "test-config", Namespace: "default"}, config)
			if configErr == nil {
				Expect(k8sClient.Delete(ctx, config)).To(Succeed())
			}

			// Cleanup Runner resource
			resource := &renovatev1beta1.Runner{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Runner")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")

			controllerReconciler := &Reconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			// Verify that the Runner resource still exists after reconciliation
			reconciledRunner := &renovatev1beta1.Runner{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, reconciledRunner)).To(Succeed())
		})

		It("should handle non-existent Runner resource gracefully", func() {
			By("Testing reconciliation with non-existent Runner")

			controllerReconciler := &Reconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			// Use a non-existent resource name
			nonExistentName := types.NamespacedName{
				Name:      "non-existent-runner",
				Namespace: "default",
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: nonExistentName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should handle missing RenovateConfig resource gracefully", func() {
			By("Testing error handling when RenovateConfig is missing")
			// Create a mock client that returns NotFound for RenovateConfig
			mockClient := &mockErrorClient{
				Client: k8sClient,
			}

			errorReconciler := &Reconciler{
				Client: mockClient,
				Scheme: k8sClient.Scheme(),
			}

			// The mock client returns NotFound for RenovateConfig, which should be handled gracefully
			result, err := errorReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
		})
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
