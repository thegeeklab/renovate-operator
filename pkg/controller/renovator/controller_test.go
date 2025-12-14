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
				resource.Default()
				resource.Spec.Schedule = "0 0 * * *"
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &renovatev1beta1.Renovator{}
			resource.Default()
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

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
