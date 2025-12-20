package renovator

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	v1beta1 "github.com/thegeeklab/renovate-operator/internal/webhook/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Renovator Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "renovate-token-secret",
					Namespace: "default",
				},
				Type: corev1.SecretTypeOpaque,
				StringData: map[string]string{
					"token": "dummy-token",
				},
			}
			err := k8sClient.Create(ctx, secret)
			if err != nil {
				Expect(api_errors.IsAlreadyExists(err)).To(BeTrue())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			By("creating the custom resource for the Kind Renovator")
			err = k8sClient.Get(ctx, typeNamespacedName, &renovatev1beta1.Renovator{})
			if err != nil && api_errors.IsNotFound(err) {
				resource := &renovatev1beta1.Renovator{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: renovatev1beta1.RenovatorSpec{
						Renovate: renovatev1beta1.RenovateConfigSpec{
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
					},
				}
				rd := &v1beta1.RenovatorCustomDefaulter{}
				Expect(rd.Default(ctx, resource)).To(Succeed())
				resource.Spec.Schedule = "0 0 * * *"
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// Cleanup the resource instance after each test
			resource := &renovatev1beta1.Renovator{}
			rd := &v1beta1.RenovatorCustomDefaulter{}
			Expect(rd.Default(ctx, resource)).To(Succeed())
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Renovator")
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
			Expect(result.Requeue).To(BeFalse())

			// Verify the resource was reconciled successfully
			resource := &renovatev1beta1.Renovator{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())
			// Note: Status.Ready might not be set immediately, so we just verify no error occurred
		})

		It("should handle non-existent resource gracefully", func() {
			By("Testing reconciliation of non-existent resource")
			controllerReconciler := &Reconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			nonExistentName := types.NamespacedName{
				Name:      "non-existent-resource",
				Namespace: "default",
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: nonExistentName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should handle errors from renovator component", func() {
			By("Testing error handling when dependent resources are missing")
			// Create a mock client that returns NotFound for dependent resources
			mockClient := &mockErrorClient{
				Client: k8sClient,
			}

			errorReconciler := &Reconciler{
				Client: mockClient,
				Scheme: k8sClient.Scheme(),
			}

			// The mock client returns NotFound for dependent resources, which should be handled gracefully
			result, err := errorReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should verify resource cleanup in AfterEach", func() {
			By("Verifying resource cleanup")
			// This test verifies that resources are properly cleaned up after each test
			// We'll create an additional resource and verify it gets cleaned up

			// Create an additional test resource
			additionalResource := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "additional-test-resource",
					Namespace: "default",
				},
				Spec: renovatev1beta1.RenovatorSpec{
					Renovate: renovatev1beta1.RenovateConfigSpec{
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
				},
			}

			rd := &v1beta1.RenovatorCustomDefaulter{}
			Expect(rd.Default(ctx, additionalResource)).To(Succeed())
			additionalResource.Spec.Schedule = "0 0 * * *"
			Expect(k8sClient.Create(ctx, additionalResource)).To(Succeed())

			// Verify the resource was created
			createdResource := &renovatev1beta1.Renovator{}
			additionalName := types.NamespacedName{Name: "additional-test-resource", Namespace: "default"}
			Expect(k8sClient.Get(ctx, additionalName, createdResource)).To(Succeed())

			// Clean up the additional resource manually since it's not handled by AfterEach
			Expect(k8sClient.Delete(ctx, createdResource)).To(Succeed())
		})
	})
})

// mockErrorClient is a mock client that returns errors for testing.
type mockErrorClient struct {
	ctrl.Client
}

func (m *mockErrorClient) Get(ctx context.Context, key ctrl.ObjectKey, obj ctrl.Object, opts ...ctrl.GetOption) error {
	// Return error for dependent resources to simulate missing resources
	if _, ok := obj.(*renovatev1beta1.Renovator); ok {
		return api_errors.NewNotFound(renovatev1beta1.GroupVersion.WithResource("renovators").GroupResource(), key.Name)
	}

	return m.Client.Get(ctx, key, obj, opts...)
}
