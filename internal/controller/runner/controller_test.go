package runner

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	v1beta1 "github.com/thegeeklab/renovate-operator/internal/webhook/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"
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
			err := k8sClient.Get(ctx, typeNamespacedName, &renovatev1beta1.Runner{})
			if err != nil && api_errors.IsNotFound(err) {
				resource := &renovatev1beta1.Runner{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: renovatev1beta1.RunnerSpec{
						JobSpec: renovatev1beta1.JobSpec{
							Schedule: "0 */2 * * *",
						},
						Strategy:  renovatev1beta1.RunnerStrategy_NONE,
						Instances: 1,
					},
				}
				rd := &v1beta1.RunnerCustomDefaulter{}
				Expect(rd.Default(ctx, resource)).To(Succeed())
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
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
	ctrl.Client
}

func (m *mockErrorClient) Get(ctx context.Context, key ctrl.ObjectKey, obj ctrl.Object, opts ...ctrl.GetOption) error {
	// Return error for RenovateConfig to simulate missing config
	if _, ok := obj.(*renovatev1beta1.RenovateConfig); ok {
		return api_errors.NewNotFound(renovatev1beta1.GroupVersion.WithResource("renovateconfigs").GroupResource(), key.Name)
	}

	return m.Client.Get(ctx, key, obj, opts...)
}
