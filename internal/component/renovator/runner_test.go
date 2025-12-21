package renovator

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Renovator Runner", func() {
	var (
		ctx        context.Context
		scheme     *runtime.Scheme
		fakeClient client.Client
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(renovatev1beta1.AddToScheme(scheme)).To(Succeed())
		fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()
	})

	Describe("Annotation Forwarding", func() {
		It("should forward operation annotation from Renovator to Runner", func() {
			// Create a Renovator instance with operation annotation
			renovator := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator",
					Namespace: "default",
					Annotations: map[string]string{
						renovatev1beta1.RenovatorOperation: renovatev1beta1.OperationRenovate,
					},
				},
				Spec: renovatev1beta1.RenovatorSpec{
					Runner: renovatev1beta1.RunnerSpec{
						JobSpec: renovatev1beta1.JobSpec{
							Schedule: "0 0 * * *",
						},
					},
				},
			}

			// Create a Reconciler
			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			// Create a Runner instance
			runner := &renovatev1beta1.Runner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-runner",
					Namespace: "default",
				},
			}

			// Call updateRunner
			err = reconciler.updateRunner(runner)
			Expect(err).NotTo(HaveOccurred())

			// Verify the annotation was forwarded
			Expect(runner.Annotations).NotTo(BeNil())
			Expect(runner.Annotations).To(HaveKey(renovatev1beta1.RenovatorOperation))
			Expect(runner.Annotations[renovatev1beta1.RenovatorOperation]).To(Equal(renovatev1beta1.OperationRenovate))
		})

		It("should not forward annotation when Renovator has no annotations", func() {
			// Create a Renovator instance without annotations
			renovator := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator",
					Namespace: "default",
				},
				Spec: renovatev1beta1.RenovatorSpec{
					Runner: renovatev1beta1.RunnerSpec{
						JobSpec: renovatev1beta1.JobSpec{
							Schedule: "0 0 * * *",
						},
					},
				},
			}

			// Create a Reconciler
			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			// Create a Runner instance
			runner := &renovatev1beta1.Runner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-runner",
					Namespace: "default",
				},
			}

			// Call updateRunner
			err = reconciler.updateRunner(runner)
			Expect(err).NotTo(HaveOccurred())

			// Verify no annotation was added
			Expect(runner.Annotations).To(BeNil())
		})

		It("should preserve existing annotations on Runner", func() {
			// Create a Renovator instance with operation annotation
			renovator := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator",
					Namespace: "default",
					Annotations: map[string]string{
						renovatev1beta1.RenovatorOperation: renovatev1beta1.OperationRenovate,
					},
				},
				Spec: renovatev1beta1.RenovatorSpec{
					Runner: renovatev1beta1.RunnerSpec{
						JobSpec: renovatev1beta1.JobSpec{
							Schedule: "0 0 * * *",
						},
					},
				},
			}

			// Create a Reconciler
			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			// Create a Runner instance with existing annotations
			runner := &renovatev1beta1.Runner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-runner",
					Namespace: "default",
					Annotations: map[string]string{
						"existing-annotation": "existing-value",
					},
				},
			}

			// Call updateRunner
			err = reconciler.updateRunner(runner)
			Expect(err).NotTo(HaveOccurred())

			// Verify both annotations exist
			Expect(runner.Annotations).NotTo(BeNil())
			Expect(runner.Annotations).To(HaveKey(renovatev1beta1.RenovatorOperation))
			Expect(runner.Annotations[renovatev1beta1.RenovatorOperation]).To(Equal(renovatev1beta1.OperationRenovate))
			Expect(runner.Annotations).To(HaveKey("existing-annotation"))
			Expect(runner.Annotations["existing-annotation"]).To(Equal("existing-value"))
		})

		It("should test annotation cleanup in component reconciler", func() {
			// Create a Renovator instance with operation annotation
			renovator := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator-cleanup",
					Namespace: "default",
					Annotations: map[string]string{
						renovatev1beta1.RenovatorOperation: renovatev1beta1.OperationRenovate,
					},
				},
				Spec: renovatev1beta1.RenovatorSpec{
					Runner: renovatev1beta1.RunnerSpec{
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
			Expect(renovator.Annotations).NotTo(BeNil())
			Expect(renovator.Annotations).NotTo(HaveKey(renovatev1beta1.RenovatorOperation))
		})
	})
})
