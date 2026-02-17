package renovator

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Renovator Discovery", func() {
	var (
		ctx        context.Context
		scheme     *runtime.Scheme
		fakeClient client.Client
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(renovatev1beta1.AddToScheme(scheme)).To(Succeed())
		Expect(corev1.SchemeBuilder.AddToScheme(scheme)).To(Succeed())
		fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()
	})

	Describe("Annotation Forwarding", func() {
		It("should forward operation annotation from Renovator to Discovery", func() {
			// Create a Renovator instance with operation annotation
			renovator := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator",
					Namespace: "default",
					Annotations: map[string]string{
						renovatev1beta1.RenovatorOperation: renovatev1beta1.OperationDiscover,
					},
				},
				Spec: renovatev1beta1.RenovatorSpec{
					Discovery: renovatev1beta1.DiscoverySpec{
						JobSpec: renovatev1beta1.JobSpec{
							Schedule: "0 0 * * *",
						},
					},
				},
			}

			// Create a Reconciler
			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			// Create a Discovery instance
			discovery := &renovatev1beta1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-discovery",
					Namespace: "default",
				},
			}

			// Call updateDiscovery
			err = reconciler.updateDiscovery(discovery)
			Expect(err).NotTo(HaveOccurred())

			// Verify the annotation was forwarded
			Expect(discovery.Annotations).NotTo(BeNil())
			Expect(discovery.Annotations).To(HaveKey(renovatev1beta1.RenovatorOperation))
			Expect(discovery.Annotations[renovatev1beta1.RenovatorOperation]).To(Equal(renovatev1beta1.OperationDiscover))
		})

		It("should not forward annotation when Renovator has no annotations", func() {
			// Create a Renovator instance without annotations
			renovator := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator",
					Namespace: "default",
				},
				Spec: renovatev1beta1.RenovatorSpec{
					Discovery: renovatev1beta1.DiscoverySpec{
						JobSpec: renovatev1beta1.JobSpec{
							Schedule: "0 0 * * *",
						},
					},
				},
			}

			// Create a Reconciler
			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			// Create a Discovery instance
			discovery := &renovatev1beta1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-discovery",
					Namespace: "default",
				},
			}

			// Call updateDiscovery
			err = reconciler.updateDiscovery(discovery)
			Expect(err).NotTo(HaveOccurred())

			// Verify no annotation was added
			Expect(discovery.Annotations).To(BeNil())
		})

		It("should preserve existing annotations on Discovery", func() {
			// Create a Renovator instance with operation annotation
			renovator := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator",
					Namespace: "default",
					Annotations: map[string]string{
						renovatev1beta1.RenovatorOperation: renovatev1beta1.OperationDiscover,
					},
				},
				Spec: renovatev1beta1.RenovatorSpec{
					Discovery: renovatev1beta1.DiscoverySpec{
						JobSpec: renovatev1beta1.JobSpec{
							Schedule: "0 0 * * *",
						},
					},
				},
			}

			// Create a Reconciler
			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			// Create a Discovery instance with existing annotations
			discovery := &renovatev1beta1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-discovery",
					Namespace: "default",
					Annotations: map[string]string{
						"existing-annotation": "existing-value",
					},
				},
			}

			// Call updateDiscovery
			err = reconciler.updateDiscovery(discovery)
			Expect(err).NotTo(HaveOccurred())

			// Verify both annotations exist
			Expect(discovery.Annotations).NotTo(BeNil())
			Expect(discovery.Annotations).To(HaveKey(renovatev1beta1.RenovatorOperation))
			Expect(discovery.Annotations[renovatev1beta1.RenovatorOperation]).To(Equal(renovatev1beta1.OperationDiscover))
			Expect(discovery.Annotations).To(HaveKey("existing-annotation"))
			Expect(discovery.Annotations["existing-annotation"]).To(Equal("existing-value"))
		})

		It("should test annotation cleanup in component reconciler", func() {
			// Create a Renovator instance with operation annotation
			renovator := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator-cleanup",
					Namespace: "default",
					Annotations: map[string]string{
						renovatev1beta1.RenovatorOperation: renovatev1beta1.OperationDiscover,
					},
				},
				Spec: renovatev1beta1.RenovatorSpec{
					Discovery: renovatev1beta1.DiscoverySpec{
						JobSpec: renovatev1beta1.JobSpec{
							Schedule: "0 0 * * *",
						},
					},
				},
			}

			// Create the Renovator resource in the fake client first
			Expect(fakeClient.Create(ctx, renovator)).To(Succeed())

			// Create a Reconciler
			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			// Call the full Reconcile method to test annotation cleanup
			_, err = reconciler.Reconcile(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Verify the annotation was removed from the Renovator
			Expect(renovator.Annotations).To(BeEmpty())
			Expect(renovator.Annotations).NotTo(HaveKey(renovatev1beta1.RenovatorOperation))
		})

		It("should copy Discovery configuration from Renovator spec", func() {
			// Create a Renovator instance with Discovery configuration
			renovator := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator-config",
					Namespace: "default",
				},
				Spec: renovatev1beta1.RenovatorSpec{
					ImageSpec: renovatev1beta1.ImageSpec{
						Image:           "renovate/renovate:36",
						ImagePullPolicy: corev1.PullIfNotPresent,
					},
					Logging: renovatev1beta1.LoggingSpec{
						Level: renovatev1beta1.LogLevel_DEBUG,
					},
					Discovery: renovatev1beta1.DiscoverySpec{
						JobSpec: renovatev1beta1.JobSpec{
							Schedule: "0 0 * * *",
						},
						ConfigRef: "test-config",
						Filter:    []string{"test-filter"},
					},
				},
			}

			// Create a Reconciler
			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			// Create a Discovery instance
			discovery := &renovatev1beta1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-discovery",
					Namespace: "default",
				},
			}

			// Call updateDiscovery
			err = reconciler.updateDiscovery(discovery)
			Expect(err).NotTo(HaveOccurred())

			// Verify the Discovery configuration was copied
			Expect(discovery.Spec.ConfigRef).To(Equal("test-config"))
			Expect(discovery.Spec.Filter).To(Equal([]string{"test-filter"}))
			Expect(discovery.Spec.Image).To(Equal("renovate/renovate:36"))
			Expect(discovery.Spec.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
			Expect(discovery.Spec.Logging).NotTo(BeNil())
			Expect(discovery.Spec.Logging.Level).To(BeEquivalentTo(renovatev1beta1.LogLevel_DEBUG))
		})

		It("should set default Image and ImagePullPolicy from Renovator", func() {
			// Create a Renovator instance with Image and ImagePullPolicy
			renovator := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator-defaults",
					Namespace: "default",
				},
				Spec: renovatev1beta1.RenovatorSpec{
					ImageSpec: renovatev1beta1.ImageSpec{
						Image:           "renovate/renovate:35",
						ImagePullPolicy: corev1.PullAlways,
					},
					Discovery: renovatev1beta1.DiscoverySpec{
						JobSpec: renovatev1beta1.JobSpec{
							Schedule: "0 0 * * *",
						},
					},
				},
			}

			// Create a Reconciler
			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			// Create a Discovery instance with empty Image and ImagePullPolicy
			discovery := &renovatev1beta1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-discovery",
					Namespace: "default",
				},
				Spec: renovatev1beta1.DiscoverySpec{
					JobSpec: renovatev1beta1.JobSpec{
						Schedule: "0 0 * * *",
					},
				},
			}

			// Call updateDiscovery
			err = reconciler.updateDiscovery(discovery)
			Expect(err).NotTo(HaveOccurred())

			// Verify the default Image and ImagePullPolicy were set
			Expect(discovery.Spec.Image).To(Equal("renovate/renovate:35"))
			Expect(discovery.Spec.ImagePullPolicy).To(Equal(corev1.PullAlways))
		})

		It("should set Renovator UID label on Discovery", func() {
			// Create a Renovator instance
			renovator := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator-label",
					Namespace: "default",
					UID:       "test-uid-123",
				},
				Spec: renovatev1beta1.RenovatorSpec{
					Discovery: renovatev1beta1.DiscoverySpec{
						JobSpec: renovatev1beta1.JobSpec{
							Schedule: "0 0 * * *",
						},
					},
				},
			}

			// Create a Reconciler
			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			// Create a Discovery instance
			discovery := &renovatev1beta1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-discovery",
					Namespace: "default",
				},
			}

			// Call updateDiscovery
			err = reconciler.updateDiscovery(discovery)
			Expect(err).NotTo(HaveOccurred())

			// Verify the Renovator UID label was set
			Expect(discovery.Labels).NotTo(BeNil())
			Expect(discovery.Labels).To(HaveKey(renovatev1beta1.RenovatorLabel))
			Expect(discovery.Labels[renovatev1beta1.RenovatorLabel]).To(Equal("test-uid-123"))
		})

		It("should preserve existing labels on Discovery", func() {
			// Create a Renovator instance
			renovator := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator-preserve-labels",
					Namespace: "default",
					UID:       "test-uid-456",
				},
				Spec: renovatev1beta1.RenovatorSpec{
					Discovery: renovatev1beta1.DiscoverySpec{
						JobSpec: renovatev1beta1.JobSpec{
							Schedule: "0 0 * * *",
						},
					},
				},
			}

			// Create a Reconciler
			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			// Create a Discovery instance with existing labels
			discovery := &renovatev1beta1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-discovery",
					Namespace: "default",
					Labels: map[string]string{
						"existing-label": "existing-value",
					},
				},
			}

			// Call updateDiscovery
			err = reconciler.updateDiscovery(discovery)
			Expect(err).NotTo(HaveOccurred())

			// Verify both labels exist
			Expect(discovery.Labels).NotTo(BeNil())
			Expect(discovery.Labels).To(HaveKey(renovatev1beta1.RenovatorLabel))
			Expect(discovery.Labels[renovatev1beta1.RenovatorLabel]).To(Equal("test-uid-456"))
			Expect(discovery.Labels).To(HaveKey("existing-label"))
			Expect(discovery.Labels["existing-label"]).To(Equal("existing-value"))
		})
	})
})
