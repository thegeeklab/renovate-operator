package renovator

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
				Name:      "test-renovator",
				Namespace: "default",
				Annotations: map[string]string{
					renovatev1beta1.RenovatorOperation: renovatev1beta1.OperationRenovate,
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
				Name:      "test-runner",
				Namespace: "default",
			}

			err = reconciler.updateRunner(runner)
			Expect(err).NotTo(HaveOccurred())

			Expect(runner.Annotations).NotTo(BeNil())
			Expect(runner.Annotations).To(HaveKey(renovatev1beta1.RenovatorOperation))
			Expect(runner.Annotations[renovatev1beta1.RenovatorOperation]).To(Equal(renovatev1beta1.OperationRenovate))
		})

		It("should not forward annotation when Renovator has no annotations", func() {
			renovator := &renovatev1beta1.Renovator{
				Name:      "test-renovator",
				Namespace: "default",
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
				Name:      "test-runner",
				Namespace: "default",
			}

			err = reconciler.updateRunner(runner)
			Expect(err).NotTo(HaveOccurred())

			Expect(runner.Annotations).To(BeNil())
		})

		It("should preserve existing annotations on Runner", func() {
			renovator := &renovatev1beta1.Renovator{
				Name:      "test-renovator",
				Namespace: "default",
				Annotations: map[string]string{
					renovatev1beta1.RenovatorOperation: renovatev1beta1.OperationRenovate,
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
				Name:      "test-runner",
				Namespace: "default",
				Annotations: map[string]string{
					"existing-annotation": "existing-value",
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
				Name:      "test-renovator-cleanup",
				Namespace: "default",
				Annotations: map[string]string{
					renovatev1beta1.RenovatorOperation: renovatev1beta1.OperationRenovate,
				},
				Spec: renovatev1beta1.RenovatorSpec{
					Runner: renovatev1beta1.RunnerSpec{
						JobSpec: renovatev1beta1.JobSpec{
							Schedule: "0 0 * * *",
						},
					},
				},
			}

			fakeClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(renovator).
				WithStatusSubresource(&renovatev1beta1.Renovator{}).
				Build()

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			_, err = reconciler.Reconcile(ctx)
			Expect(err).NotTo(HaveOccurred())

			Expect(renovator.Annotations).To(BeEmpty())
			Expect(renovator.Annotations).NotTo(HaveKey(renovatev1beta1.RenovatorOperation))
		})

		It("should consume renovate annotation on propagation to prevent duplicate jobs", func() {
			renovator := &renovatev1beta1.Renovator{
				Name:      "test-renovator-consume",
				Namespace: "default",
				Annotations: map[string]string{
					renovatev1beta1.RenovatorOperation: renovatev1beta1.OperationRenovate,
				},
				Spec: renovatev1beta1.RenovatorSpec{
					Runner: renovatev1beta1.RunnerSpec{
						JobSpec: renovatev1beta1.JobSpec{
							Schedule: "0 0 * * *",
						},
					},
				},
			}

			fakeClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(renovator).
				Build()

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			_, err = reconciler.reconcileRunner(ctx)
			Expect(err).NotTo(HaveOccurred())

			stored := &renovatev1beta1.Renovator{}
			err = fakeClient.Get(ctx, client.ObjectKeyFromObject(renovator), stored)
			Expect(err).NotTo(HaveOccurred())
			Expect(stored.Annotations).NotTo(HaveKey(renovatev1beta1.RenovatorOperation))

			Expect(renovator.Annotations).NotTo(HaveKey(renovatev1beta1.RenovatorOperation))

			secondRunner := &renovatev1beta1.Runner{
				Name:      "test-runner-2",
				Namespace: "default",
			}
			err = reconciler.updateRunner(secondRunner)
			Expect(err).NotTo(HaveOccurred())
			Expect(secondRunner.Annotations).NotTo(HaveKey(renovatev1beta1.RenovatorOperation))
		})

		It("should not forward annotation when Renovator has no renovate operation", func() {
			renovator := &renovatev1beta1.Renovator{
				Name:      "test-renovator-discover",
				Namespace: "default",
				Annotations: map[string]string{
					renovatev1beta1.RenovatorOperation: renovatev1beta1.OperationDiscover,
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
				Name:      "test-runner",
				Namespace: "default",
			}

			err = reconciler.updateRunner(runner)
			Expect(err).NotTo(HaveOccurred())
			Expect(runner.Annotations).To(BeNil())
		})

		It("should forward renovate operation annotation when Renovator has multiple operations", func() {
			renovator := &renovatev1beta1.Renovator{
				Name:      "test-renovator-multi",
				Namespace: "default",
				Annotations: map[string]string{
					renovatev1beta1.RenovatorOperation: renovatev1beta1.OperationDiscover + ";" + renovatev1beta1.OperationRenovate,
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
				Name:      "test-runner",
				Namespace: "default",
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
				Name:      "test-runner",
				Namespace: "default",
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

		It("should inherit ImagePullSecrets from the global spec and allow runner override", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					ImageSpec: renovatev1beta1.ImageSpec{
						ImagePullSecrets: []corev1.LocalObjectReference{
							{Name: "renovator-registry-secret"},
						},
					},
					Runner: renovatev1beta1.RunnerSpec{
						ImageSpec: renovatev1beta1.ImageSpec{
							ImagePullSecrets: []corev1.LocalObjectReference{
								{Name: "runner-registry-secret"},
							},
						},
					},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateRunner(existingRunner)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingRunner.Spec.ImagePullSecrets).To(Equal([]corev1.LocalObjectReference{
				{Name: "runner-registry-secret"},
			}))
		})

		It("should inherit ImagePullSecrets from Renovator when Runner ImagePullSecrets is nil", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					ImageSpec: renovatev1beta1.ImageSpec{
						ImagePullSecrets: []corev1.LocalObjectReference{
							{Name: "renovator-registry-secret"},
						},
					},
					Runner: renovatev1beta1.RunnerSpec{},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateRunner(existingRunner)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingRunner.Spec.ImagePullSecrets).To(Equal([]corev1.LocalObjectReference{
				{Name: "renovator-registry-secret"},
			}))
		})

		It("should allow Runner to clear ImagePullSecrets with empty slice", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					ImageSpec: renovatev1beta1.ImageSpec{
						ImagePullSecrets: []corev1.LocalObjectReference{
							{Name: "renovator-registry-secret"},
						},
					},
					Runner: renovatev1beta1.RunnerSpec{
						ImageSpec: renovatev1beta1.ImageSpec{
							ImagePullSecrets: []corev1.LocalObjectReference{},
						},
					},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateRunner(existingRunner)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingRunner.Spec.ImagePullSecrets).To(BeEmpty())
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

		It("should inherit PodSpec from the global spec", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					PodSpec: renovatev1beta1.PodSpec{
						NodeSelector: map[string]string{"disktype": "ssd"},
						Tolerations: []corev1.Toleration{
							{Key: "key1", Operator: corev1.TolerationOpEqual, Value: "value1"},
						},
					},
					Runner: renovatev1beta1.RunnerSpec{},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateRunner(existingRunner)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingRunner.Spec.NodeSelector).To(HaveKeyWithValue("disktype", "ssd"))
			Expect(existingRunner.Spec.Tolerations).To(HaveLen(1))
		})

		It("should override global PodSpec with runner-specific PodSpec", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					PodSpec: renovatev1beta1.PodSpec{
						NodeSelector: map[string]string{"disktype": "ssd"},
					},
					Runner: renovatev1beta1.RunnerSpec{
						PodSpec: renovatev1beta1.PodSpec{
							NodeSelector: map[string]string{"disktype": "hdd"},
						},
					},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateRunner(existingRunner)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingRunner.Spec.NodeSelector).To(HaveKeyWithValue("disktype", "hdd"))
		})

		It("should inherit Resources from the global spec", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					PodSpec: renovatev1beta1.PodSpec{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("100m"),
							},
						},
					},
					Runner: renovatev1beta1.RunnerSpec{},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateRunner(existingRunner)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingRunner.Spec.Resources.Requests).To(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("100m")))
		})

		It("should override global Resources with runner-specific Resources", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					PodSpec: renovatev1beta1.PodSpec{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("100m"),
							},
						},
					},
					Runner: renovatev1beta1.RunnerSpec{
						PodSpec: renovatev1beta1.PodSpec{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("500m"),
								},
							},
						},
					},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateRunner(existingRunner)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingRunner.Spec.Resources.Requests).To(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("500m")))
		})

		It("should inherit SecurityContext from the global spec", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					PodSpec: renovatev1beta1.PodSpec{
						SecurityContext: &corev1.SecurityContext{
							RunAsNonRoot: new(true),
						},
					},
					Runner: renovatev1beta1.RunnerSpec{},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateRunner(existingRunner)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingRunner.Spec.SecurityContext).NotTo(BeNil())
			Expect(*existingRunner.Spec.SecurityContext.RunAsNonRoot).To(BeTrue())
		})

		It("should override global SecurityContext with runner-specific SecurityContext", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					PodSpec: renovatev1beta1.PodSpec{
						SecurityContext: &corev1.SecurityContext{
							RunAsNonRoot: new(true),
						},
					},
					Runner: renovatev1beta1.RunnerSpec{
						PodSpec: renovatev1beta1.PodSpec{
							SecurityContext: &corev1.SecurityContext{
								RunAsUser: new(int64(1000)),
							},
						},
					},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateRunner(existingRunner)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingRunner.Spec.SecurityContext).NotTo(BeNil())
			Expect(*existingRunner.Spec.SecurityContext.RunAsUser).To(Equal(int64(1000)))
		})

		It("should inherit ExtraEnv from the global spec", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					PodSpec: renovatev1beta1.PodSpec{
						ExtraEnv: []corev1.EnvVar{
							{Name: "GLOBAL_VAR", Value: "global_value"},
						},
					},
					Runner: renovatev1beta1.RunnerSpec{},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateRunner(existingRunner)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingRunner.Spec.ExtraEnv).To(HaveLen(1))
			Expect(existingRunner.Spec.ExtraEnv[0].Name).To(Equal("GLOBAL_VAR"))
		})

		It("should override global ExtraEnv with runner-specific ExtraEnv", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					PodSpec: renovatev1beta1.PodSpec{
						ExtraEnv: []corev1.EnvVar{
							{Name: "GLOBAL_VAR", Value: "global_value"},
						},
					},
					Runner: renovatev1beta1.RunnerSpec{
						PodSpec: renovatev1beta1.PodSpec{
							ExtraEnv: []corev1.EnvVar{
								{Name: "RUNNER_VAR", Value: "runner_value"},
							},
						},
					},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateRunner(existingRunner)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingRunner.Spec.ExtraEnv).To(HaveLen(1))
			Expect(existingRunner.Spec.ExtraEnv[0].Name).To(Equal("RUNNER_VAR"))
		})

		It("should inherit ExtraVolumes from the global spec", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					PodSpec: renovatev1beta1.PodSpec{
						ExtraVolumes: []corev1.Volume{
							{Name: "global-vol", EmptyDir: &corev1.EmptyDirVolumeSource{}},
						},
					},
					Runner: renovatev1beta1.RunnerSpec{},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateRunner(existingRunner)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingRunner.Spec.ExtraVolumes).To(HaveLen(1))
			Expect(existingRunner.Spec.ExtraVolumes[0].Name).To(Equal("global-vol"))
		})

		It("should override global ExtraVolumes with runner-specific ExtraVolumes", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					PodSpec: renovatev1beta1.PodSpec{
						ExtraVolumes: []corev1.Volume{
							{Name: "global-vol", EmptyDir: &corev1.EmptyDirVolumeSource{}},
						},
					},
					Runner: renovatev1beta1.RunnerSpec{
						PodSpec: renovatev1beta1.PodSpec{
							ExtraVolumes: []corev1.Volume{
								{Name: "runner-vol", EmptyDir: &corev1.EmptyDirVolumeSource{}},
							},
						},
					},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateRunner(existingRunner)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingRunner.Spec.ExtraVolumes).To(HaveLen(1))
			Expect(existingRunner.Spec.ExtraVolumes[0].Name).To(Equal("runner-vol"))
		})

		It("should inherit RuntimeClassName from the global spec", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					PodSpec: renovatev1beta1.PodSpec{
						RuntimeClassName: new("gvisor"),
					},
					Runner: renovatev1beta1.RunnerSpec{},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateRunner(existingRunner)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingRunner.Spec.RuntimeClassName).NotTo(BeNil())
			Expect(*existingRunner.Spec.RuntimeClassName).To(Equal("gvisor"))
		})

		It("should override global RuntimeClassName with runner-specific RuntimeClassName", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					PodSpec: renovatev1beta1.PodSpec{
						RuntimeClassName: new("gvisor"),
					},
					Runner: renovatev1beta1.RunnerSpec{
						PodSpec: renovatev1beta1.PodSpec{
							RuntimeClassName: new("kata"),
						},
					},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateRunner(existingRunner)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingRunner.Spec.RuntimeClassName).NotTo(BeNil())
			Expect(*existingRunner.Spec.RuntimeClassName).To(Equal("kata"))
		})

		It("should inherit PodAnnotations from the global spec", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					PodSpec: renovatev1beta1.PodSpec{
						PodAnnotations: map[string]string{"prometheus.io/scrape": "true"},
					},
					Runner: renovatev1beta1.RunnerSpec{},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateRunner(existingRunner)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingRunner.Spec.PodAnnotations).To(HaveKeyWithValue("prometheus.io/scrape", "true"))
		})

		It("should override global PodAnnotations with runner-specific PodAnnotations", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					PodSpec: renovatev1beta1.PodSpec{
						PodAnnotations: map[string]string{"prometheus.io/scrape": "true"},
					},
					Runner: renovatev1beta1.RunnerSpec{
						PodSpec: renovatev1beta1.PodSpec{
							PodAnnotations: map[string]string{"istio-injection": "enabled"},
						},
					},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateRunner(existingRunner)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingRunner.Spec.PodAnnotations).To(HaveKeyWithValue("istio-injection", "enabled"))
			Expect(existingRunner.Spec.PodAnnotations).NotTo(HaveKey("prometheus.io/scrape"))
		})

		It("should inherit ScratchVolume from the global spec", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					PodSpec: renovatev1beta1.PodSpec{
						ScratchVolume: &renovatev1beta1.ScratchVolumeSpec{
							Path: "/scratch",
						},
					},
					Runner: renovatev1beta1.RunnerSpec{},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateRunner(existingRunner)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingRunner.Spec.ScratchVolume).NotTo(BeNil())
			Expect(existingRunner.Spec.ScratchVolume.Path).To(Equal("/scratch"))
		})

		It("should override global ScratchVolume with runner-specific ScratchVolume", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					PodSpec: renovatev1beta1.PodSpec{
						ScratchVolume: &renovatev1beta1.ScratchVolumeSpec{
							Path: "/scratch",
						},
					},
					Runner: renovatev1beta1.RunnerSpec{
						PodSpec: renovatev1beta1.PodSpec{
							ScratchVolume: &renovatev1beta1.ScratchVolumeSpec{
								Path: "/runner-scratch",
							},
						},
					},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateRunner(existingRunner)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingRunner.Spec.ScratchVolume).NotTo(BeNil())
			Expect(existingRunner.Spec.ScratchVolume.Path).To(Equal("/runner-scratch"))
		})
	})
})
