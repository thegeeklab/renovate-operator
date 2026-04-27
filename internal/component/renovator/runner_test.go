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
		Expect(corev1.SchemeBuilder.AddToScheme(scheme)).To(Succeed())
		fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()
	})

	Describe("Annotation Forwarding", func() {
		It("should forward operation annotation from Renovator to Runner", func() {
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

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			runner := &renovatev1beta1.Runner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-runner",
					Namespace: "default",
				},
			}

			err = reconciler.updateRunner(runner)
			Expect(err).NotTo(HaveOccurred())

			Expect(runner.Annotations).NotTo(BeNil())
			Expect(runner.Annotations).To(HaveKey(renovatev1beta1.RenovatorOperation))
			Expect(runner.Annotations[renovatev1beta1.RenovatorOperation]).To(Equal(renovatev1beta1.OperationRenovate))
		})

		It("should not forward annotation when Renovator has no annotations", func() {
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

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			runner := &renovatev1beta1.Runner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-runner",
					Namespace: "default",
				},
			}

			err = reconciler.updateRunner(runner)
			Expect(err).NotTo(HaveOccurred())

			Expect(runner.Annotations).To(BeNil())
		})

		It("should preserve existing annotations on Runner", func() {
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

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			runner := &renovatev1beta1.Runner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-runner",
					Namespace: "default",
					Annotations: map[string]string{
						"existing-annotation": "existing-value",
					},
				},
			}

			err = reconciler.updateRunner(runner)
			Expect(err).NotTo(HaveOccurred())

			Expect(runner.Annotations).NotTo(BeNil())
			Expect(runner.Annotations).To(HaveKey(renovatev1beta1.RenovatorOperation))
			Expect(runner.Annotations[renovatev1beta1.RenovatorOperation]).To(Equal(renovatev1beta1.OperationRenovate))
			Expect(runner.Annotations).To(HaveKey("existing-annotation"))
			Expect(runner.Annotations["existing-annotation"]).To(Equal("existing-value"))
		})

		It("should test annotation cleanup in component reconciler", func() {
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

			Expect(fakeClient.Create(ctx, renovator)).To(Succeed())

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			_, err = reconciler.Reconcile(ctx)
			Expect(err).NotTo(HaveOccurred())

			Expect(renovator.Annotations).To(BeEmpty())
			Expect(renovator.Annotations).NotTo(HaveKey(renovatev1beta1.RenovatorOperation))
		})

		It("should not forward annotation when Renovator has no renovate operation", func() {
			renovator := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator-discover",
					Namespace: "default",
					Annotations: map[string]string{
						renovatev1beta1.RenovatorOperation: renovatev1beta1.OperationDiscover,
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

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			runner := &renovatev1beta1.Runner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-runner",
					Namespace: "default",
				},
			}

			err = reconciler.updateRunner(runner)
			Expect(err).NotTo(HaveOccurred())
			Expect(runner.Annotations).To(BeNil())
		})

		It("should forward renovate operation annotation when Renovator has multiple operations", func() {
			renovator := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator-multi",
					Namespace: "default",
					Annotations: map[string]string{
						renovatev1beta1.RenovatorOperation: renovatev1beta1.OperationDiscover + ";" + renovatev1beta1.OperationRenovate,
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

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			runner := &renovatev1beta1.Runner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-runner",
					Namespace: "default",
				},
			}

			err = reconciler.updateRunner(runner)
			Expect(err).NotTo(HaveOccurred())

			Expect(runner.Annotations).NotTo(BeNil())
			Expect(runner.Annotations).To(HaveKey(renovatev1beta1.RenovatorOperation))
			Expect(runner.Annotations[renovatev1beta1.RenovatorOperation]).To(Equal(renovatev1beta1.OperationRenovate))
		})
	})

	Describe("Spec Synchronization", func() {
		var existingRunner *renovatev1beta1.Runner

		BeforeEach(func() {
			existingRunner = &renovatev1beta1.Runner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-runner",
					Namespace: "default",
				},
				Spec: renovatev1beta1.RunnerSpec{
					JobSpec: renovatev1beta1.JobSpec{
						TTLSecondsAfterFinished: new(int32(3600)),
						Schedule:                "*/10 * * * *",
					},
				},
			}
		})

		It("should inherit properties from the global spec", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					JobSpec: renovatev1beta1.JobSpec{
						TTLSecondsAfterFinished: new(int32(1800)),
						Schedule:                "*/5 * * * *",
					},
					Runner: renovatev1beta1.RunnerSpec{},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateRunner(existingRunner)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingRunner.Spec.TTLSecondsAfterFinished).To(Equal(new(int32(1800))))
			Expect(existingRunner.Spec.Schedule).To(Equal("*/5 * * * *"))
		})

		It("should override global properties with runner-specific properties", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					JobSpec: renovatev1beta1.JobSpec{
						TTLSecondsAfterFinished: new(int32(1800)),
					},
					Runner: renovatev1beta1.RunnerSpec{
						JobSpec: renovatev1beta1.JobSpec{
							TTLSecondsAfterFinished: new(int32(600)),
						},
					},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateRunner(existingRunner)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingRunner.Spec.TTLSecondsAfterFinished).To(Equal(new(int32(600))))
		})

		It("should successfully unset properties if they are removed from the parent Renovator", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					Runner: renovatev1beta1.RunnerSpec{},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingRunner.Spec.TTLSecondsAfterFinished).To(Equal(new(int32(3600))))

			err = reconciler.updateRunner(existingRunner)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingRunner.Spec.TTLSecondsAfterFinished).To(BeNil())
		})
	})
})
