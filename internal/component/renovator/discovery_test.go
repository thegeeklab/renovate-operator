package renovator

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			discovery := &renovatev1beta1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-discovery",
					Namespace: "default",
				},
			}

			err = reconciler.updateDiscovery(discovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(discovery.Annotations).NotTo(BeNil())
			Expect(discovery.Annotations).To(HaveKey(renovatev1beta1.RenovatorOperation))
			Expect(discovery.Annotations[renovatev1beta1.RenovatorOperation]).To(Equal(renovatev1beta1.OperationDiscover))
		})

		It("should not forward annotation when Renovator has no annotations", func() {
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

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			discovery := &renovatev1beta1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-discovery",
					Namespace: "default",
				},
			}

			err = reconciler.updateDiscovery(discovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(discovery.Annotations).To(BeNil())
		})

		It("should preserve existing annotations on Discovery", func() {
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

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			discovery := &renovatev1beta1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-discovery",
					Namespace: "default",
					Annotations: map[string]string{
						"existing-annotation": "existing-value",
					},
				},
			}

			err = reconciler.updateDiscovery(discovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(discovery.Annotations).NotTo(BeNil())
			Expect(discovery.Annotations).To(HaveKey(renovatev1beta1.RenovatorOperation))
			Expect(discovery.Annotations[renovatev1beta1.RenovatorOperation]).To(Equal(renovatev1beta1.OperationDiscover))
			Expect(discovery.Annotations).To(HaveKey("existing-annotation"))
			Expect(discovery.Annotations["existing-annotation"]).To(Equal("existing-value"))
		})

		It("should test annotation cleanup in component reconciler", func() {
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

		It("should copy Discovery configuration from Renovator spec", func() {
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
						Topics:    []string{"renovate", "production"},
					},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			discovery := &renovatev1beta1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-discovery",
					Namespace: "default",
				},
			}

			err = reconciler.updateDiscovery(discovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(discovery.Spec.ConfigRef).To(Equal("test-config"))
			Expect(discovery.Spec.Filter).To(Equal([]string{"test-filter"}))
			Expect(discovery.Spec.Topics).To(Equal([]string{"renovate", "production"}))
			Expect(discovery.Spec.Image).To(Equal("renovate/renovate:36"))
			Expect(discovery.Spec.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
			Expect(discovery.Spec.Logging).NotTo(BeNil())
			Expect(discovery.Spec.Logging.Level).To(BeEquivalentTo(renovatev1beta1.LogLevel_DEBUG))
		})

		It("should set default Image and ImagePullPolicy from Renovator", func() {
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

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

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

			err = reconciler.updateDiscovery(discovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(discovery.Spec.Image).To(Equal("renovate/renovate:35"))
			Expect(discovery.Spec.ImagePullPolicy).To(Equal(corev1.PullAlways))
		})

		It("should set default ImagePullSecrets from Renovator and allow Discovery override", func() {
			renovator := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator-pull-secrets",
					Namespace: "default",
				},
				Spec: renovatev1beta1.RenovatorSpec{
					ImageSpec: renovatev1beta1.ImageSpec{
						ImagePullSecrets: []corev1.LocalObjectReference{
							{Name: "renovator-registry-secret"},
						},
					},
					Discovery: renovatev1beta1.DiscoverySpec{
						JobSpec: renovatev1beta1.JobSpec{
							Schedule: "0 0 * * *",
						},
						ImageSpec: renovatev1beta1.ImageSpec{
							ImagePullSecrets: []corev1.LocalObjectReference{
								{Name: "discovery-registry-secret"},
							},
						},
					},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			discovery := &renovatev1beta1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-discovery",
					Namespace: "default",
				},
			}

			err = reconciler.updateDiscovery(discovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(discovery.Spec.ImagePullSecrets).To(Equal([]corev1.LocalObjectReference{
				{Name: "discovery-registry-secret"},
			}))
		})

		It("should inherit ImagePullSecrets from Renovator when Discovery ImagePullSecrets is nil", func() {
			renovator := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator-inherit-secrets",
					Namespace: "default",
				},
				Spec: renovatev1beta1.RenovatorSpec{
					ImageSpec: renovatev1beta1.ImageSpec{
						ImagePullSecrets: []corev1.LocalObjectReference{
							{Name: "renovator-registry-secret"},
						},
					},
					Discovery: renovatev1beta1.DiscoverySpec{
						JobSpec: renovatev1beta1.JobSpec{
							Schedule: "0 0 * * *",
						},
					},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			discovery := &renovatev1beta1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-discovery",
					Namespace: "default",
				},
			}

			err = reconciler.updateDiscovery(discovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(discovery.Spec.ImagePullSecrets).To(Equal([]corev1.LocalObjectReference{
				{Name: "renovator-registry-secret"},
			}))
		})

		It("should allow Discovery to clear ImagePullSecrets with empty slice", func() {
			renovator := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator-clear-secrets",
					Namespace: "default",
				},
				Spec: renovatev1beta1.RenovatorSpec{
					ImageSpec: renovatev1beta1.ImageSpec{
						ImagePullSecrets: []corev1.LocalObjectReference{
							{Name: "renovator-registry-secret"},
						},
					},
					Discovery: renovatev1beta1.DiscoverySpec{
						JobSpec: renovatev1beta1.JobSpec{
							Schedule: "0 0 * * *",
						},
						ImageSpec: renovatev1beta1.ImageSpec{
							ImagePullSecrets: []corev1.LocalObjectReference{},
						},
					},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			discovery := &renovatev1beta1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-discovery",
					Namespace: "default",
				},
			}

			err = reconciler.updateDiscovery(discovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(discovery.Spec.ImagePullSecrets).To(BeEmpty())
		})

		It("should set Renovator UID label on Discovery", func() {
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

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			discovery := &renovatev1beta1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-discovery",
					Namespace: "default",
				},
			}

			err = reconciler.updateDiscovery(discovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(discovery.Labels).NotTo(BeNil())
			Expect(discovery.Labels).To(HaveKey(renovatev1beta1.LabelRenovator))
			Expect(discovery.Labels[renovatev1beta1.LabelRenovator]).To(Equal("test-uid-123"))
		})

		It("should preserve existing labels on Discovery", func() {
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

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			discovery := &renovatev1beta1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-discovery",
					Namespace: "default",
					Labels: map[string]string{
						"existing-label": "existing-value",
					},
				},
			}

			err = reconciler.updateDiscovery(discovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(discovery.Labels).NotTo(BeNil())
			Expect(discovery.Labels).To(HaveKey(renovatev1beta1.LabelRenovator))
			Expect(discovery.Labels[renovatev1beta1.LabelRenovator]).To(Equal("test-uid-456"))
			Expect(discovery.Labels).To(HaveKey("existing-label"))
			Expect(discovery.Labels["existing-label"]).To(Equal("existing-value"))
		})
	})

	Describe("Spec Synchronization", func() {
		var existingDiscovery *renovatev1beta1.Discovery

		BeforeEach(func() {
			existingDiscovery = &renovatev1beta1.Discovery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-discovery",
					Namespace: "default",
				},
				Spec: renovatev1beta1.DiscoverySpec{
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
					Discovery: renovatev1beta1.DiscoverySpec{},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateDiscovery(existingDiscovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingDiscovery.Spec.TTLSecondsAfterFinished).To(Equal(new(int32(1800))))
			Expect(existingDiscovery.Spec.Schedule).To(Equal("*/5 * * * *"))
		})

		It("should override global properties with discovery-specific properties", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					JobSpec: renovatev1beta1.JobSpec{
						TTLSecondsAfterFinished: new(int32(1800)),
					},
					Discovery: renovatev1beta1.DiscoverySpec{
						JobSpec: renovatev1beta1.JobSpec{
							TTLSecondsAfterFinished: new(int32(600)),
						},
					},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateDiscovery(existingDiscovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingDiscovery.Spec.TTLSecondsAfterFinished).To(Equal(new(int32(600))))
		})

		It("should successfully unset properties if they are removed from the parent Renovator", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					Discovery: renovatev1beta1.DiscoverySpec{},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingDiscovery.Spec.TTLSecondsAfterFinished).To(Equal(new(int32(3600))))

			err = reconciler.updateDiscovery(existingDiscovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingDiscovery.Spec.TTLSecondsAfterFinished).To(BeNil())
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
					Discovery: renovatev1beta1.DiscoverySpec{},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateDiscovery(existingDiscovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingDiscovery.Spec.NodeSelector).To(HaveKeyWithValue("disktype", "ssd"))
			Expect(existingDiscovery.Spec.Tolerations).To(HaveLen(1))
		})

		It("should override global PodSpec with discovery-specific PodSpec", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					PodSpec: renovatev1beta1.PodSpec{
						NodeSelector: map[string]string{"disktype": "ssd"},
					},
					Discovery: renovatev1beta1.DiscoverySpec{
						PodSpec: renovatev1beta1.PodSpec{
							NodeSelector: map[string]string{"disktype": "hdd"},
						},
					},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateDiscovery(existingDiscovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingDiscovery.Spec.NodeSelector).To(HaveKeyWithValue("disktype", "hdd"))
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
					Discovery: renovatev1beta1.DiscoverySpec{},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateDiscovery(existingDiscovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingDiscovery.Spec.Resources.Requests).To(
				HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("100m")),
			)
		})

		It("should override global Resources with discovery-specific Resources", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					PodSpec: renovatev1beta1.PodSpec{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("100m"),
							},
						},
					},
					Discovery: renovatev1beta1.DiscoverySpec{
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

			err = reconciler.updateDiscovery(existingDiscovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingDiscovery.Spec.Resources.Requests).To(
				HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("500m")),
			)
		})

		It("should inherit SecurityContext from the global spec", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					PodSpec: renovatev1beta1.PodSpec{
						SecurityContext: &corev1.SecurityContext{
							RunAsNonRoot: new(true),
						},
					},
					Discovery: renovatev1beta1.DiscoverySpec{},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateDiscovery(existingDiscovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingDiscovery.Spec.SecurityContext).NotTo(BeNil())
			Expect(*existingDiscovery.Spec.SecurityContext.RunAsNonRoot).To(BeTrue())
		})

		It("should override global SecurityContext with discovery-specific SecurityContext", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					PodSpec: renovatev1beta1.PodSpec{
						SecurityContext: &corev1.SecurityContext{
							RunAsNonRoot: new(true),
						},
					},
					Discovery: renovatev1beta1.DiscoverySpec{
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

			err = reconciler.updateDiscovery(existingDiscovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingDiscovery.Spec.SecurityContext).NotTo(BeNil())
			Expect(*existingDiscovery.Spec.SecurityContext.RunAsUser).To(Equal(int64(1000)))
		})

		It("should inherit ExtraEnv from the global spec", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					PodSpec: renovatev1beta1.PodSpec{
						ExtraEnv: []corev1.EnvVar{
							{Name: "GLOBAL_VAR", Value: "global_value"},
						},
					},
					Discovery: renovatev1beta1.DiscoverySpec{},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateDiscovery(existingDiscovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingDiscovery.Spec.ExtraEnv).To(HaveLen(1))
			Expect(existingDiscovery.Spec.ExtraEnv[0].Name).To(Equal("GLOBAL_VAR"))
		})

		It("should override global ExtraEnv with discovery-specific ExtraEnv", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					PodSpec: renovatev1beta1.PodSpec{
						ExtraEnv: []corev1.EnvVar{
							{Name: "GLOBAL_VAR", Value: "global_value"},
						},
					},
					Discovery: renovatev1beta1.DiscoverySpec{
						PodSpec: renovatev1beta1.PodSpec{
							ExtraEnv: []corev1.EnvVar{
								{Name: "DISCOVERY_VAR", Value: "discovery_value"},
							},
						},
					},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateDiscovery(existingDiscovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingDiscovery.Spec.ExtraEnv).To(HaveLen(1))
			Expect(existingDiscovery.Spec.ExtraEnv[0].Name).To(Equal("DISCOVERY_VAR"))
		})

		It("should inherit ExtraVolumes from the global spec", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					PodSpec: renovatev1beta1.PodSpec{
						ExtraVolumes: []corev1.Volume{
							{Name: "global-vol", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
						},
					},
					Discovery: renovatev1beta1.DiscoverySpec{},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateDiscovery(existingDiscovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingDiscovery.Spec.ExtraVolumes).To(HaveLen(1))
			Expect(existingDiscovery.Spec.ExtraVolumes[0].Name).To(Equal("global-vol"))
		})

		It("should override global ExtraVolumes with discovery-specific ExtraVolumes", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					PodSpec: renovatev1beta1.PodSpec{
						ExtraVolumes: []corev1.Volume{
							{Name: "global-vol", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
						},
					},
					Discovery: renovatev1beta1.DiscoverySpec{
						PodSpec: renovatev1beta1.PodSpec{
							ExtraVolumes: []corev1.Volume{
								{Name: "discovery-vol", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
							},
						},
					},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateDiscovery(existingDiscovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingDiscovery.Spec.ExtraVolumes).To(HaveLen(1))
			Expect(existingDiscovery.Spec.ExtraVolumes[0].Name).To(Equal("discovery-vol"))
		})

		It("should inherit RuntimeClassName from the global spec", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					PodSpec: renovatev1beta1.PodSpec{
						RuntimeClassName: new("gvisor"),
					},
					Discovery: renovatev1beta1.DiscoverySpec{},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateDiscovery(existingDiscovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingDiscovery.Spec.RuntimeClassName).NotTo(BeNil())
			Expect(*existingDiscovery.Spec.RuntimeClassName).To(Equal("gvisor"))
		})

		It("should override global RuntimeClassName with discovery-specific RuntimeClassName", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					PodSpec: renovatev1beta1.PodSpec{
						RuntimeClassName: new("gvisor"),
					},
					Discovery: renovatev1beta1.DiscoverySpec{
						PodSpec: renovatev1beta1.PodSpec{
							RuntimeClassName: new("kata"),
						},
					},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateDiscovery(existingDiscovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingDiscovery.Spec.RuntimeClassName).NotTo(BeNil())
			Expect(*existingDiscovery.Spec.RuntimeClassName).To(Equal("kata"))
		})

		It("should inherit PodAnnotations from the global spec", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					PodSpec: renovatev1beta1.PodSpec{
						PodAnnotations: map[string]string{"prometheus.io/scrape": "true"},
					},
					Discovery: renovatev1beta1.DiscoverySpec{},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateDiscovery(existingDiscovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingDiscovery.Spec.PodAnnotations).To(HaveKeyWithValue("prometheus.io/scrape", "true"))
		})

		It("should override global PodAnnotations with discovery-specific PodAnnotations", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					PodSpec: renovatev1beta1.PodSpec{
						PodAnnotations: map[string]string{"prometheus.io/scrape": "true"},
					},
					Discovery: renovatev1beta1.DiscoverySpec{
						PodSpec: renovatev1beta1.PodSpec{
							PodAnnotations: map[string]string{"istio-injection": "enabled"},
						},
					},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateDiscovery(existingDiscovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingDiscovery.Spec.PodAnnotations).To(HaveKeyWithValue("istio-injection", "enabled"))
			Expect(existingDiscovery.Spec.PodAnnotations).NotTo(HaveKey("prometheus.io/scrape"))
		})

		It("should inherit ScratchVolume from the global spec", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					PodSpec: renovatev1beta1.PodSpec{
						ScratchVolume: &renovatev1beta1.ScratchVolumeSpec{
							Path: "/scratch",
						},
					},
					Discovery: renovatev1beta1.DiscoverySpec{},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateDiscovery(existingDiscovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingDiscovery.Spec.ScratchVolume).NotTo(BeNil())
			Expect(existingDiscovery.Spec.ScratchVolume.Path).To(Equal("/scratch"))
		})

		It("should override global ScratchVolume with discovery-specific ScratchVolume", func() {
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					PodSpec: renovatev1beta1.PodSpec{
						ScratchVolume: &renovatev1beta1.ScratchVolumeSpec{
							Path: "/scratch",
						},
					},
					Discovery: renovatev1beta1.DiscoverySpec{
						PodSpec: renovatev1beta1.PodSpec{
							ScratchVolume: &renovatev1beta1.ScratchVolumeSpec{
								Path: "/discovery-scratch",
							},
						},
					},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.updateDiscovery(existingDiscovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(existingDiscovery.Spec.ScratchVolume).NotTo(BeNil())
			Expect(existingDiscovery.Spec.ScratchVolume.Path).To(Equal("/discovery-scratch"))
		})
	})

	Describe("Webhooks propagation", func() {
		It("should propagate webhooks config from Renovator to Discovery", func() {
			disabled := false
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					Webhooks: renovatev1beta1.WebhooksSpec{
						Enabled: &disabled,
					},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			discovery := &renovatev1beta1.Discovery{}
			err = reconciler.updateDiscovery(discovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(discovery.Spec.Webhooks.Enabled).NotTo(BeNil())
			Expect(*discovery.Spec.Webhooks.Enabled).To(BeFalse())
		})

		It("should let discovery-level webhooks override Renovator-level webhooks", func() {
			renovatorDisabled := false
			discoveryEnabled := true
			renovator := &renovatev1beta1.Renovator{
				Spec: renovatev1beta1.RenovatorSpec{
					Webhooks: renovatev1beta1.WebhooksSpec{
						Enabled: &renovatorDisabled,
					},
					Discovery: renovatev1beta1.DiscoverySpec{
						Webhooks: renovatev1beta1.WebhooksSpec{
							Enabled: &discoveryEnabled,
						},
					},
				},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			discovery := &renovatev1beta1.Discovery{}
			err = reconciler.updateDiscovery(discovery)
			Expect(err).NotTo(HaveOccurred())

			Expect(discovery.Spec.Webhooks.Enabled).NotTo(BeNil())
			Expect(*discovery.Spec.Webhooks.Enabled).To(BeTrue())
		})
	})
})
