package discovery

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/thegeeklab/renovate-operator/internal/webhook/v1beta1"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/metadata"
	"github.com/thegeeklab/renovate-operator/internal/scheduler"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	fakeclock "k8s.io/utils/clock/testing"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("ReconcileJob", func() {
	var (
		fakeClient client.Client
		reconciler *Reconciler
		instance   *renovatev1beta1.Discovery
		renovate   *renovatev1beta1.RenovateConfig
		ctx        context.Context
		scheme     *runtime.Scheme
		now        time.Time
		fakeClock  *fakeclock.FakeClock
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
		Expect(renovatev1beta1.AddToScheme(scheme)).To(Succeed())

		instance = &renovatev1beta1.Discovery{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-discovery",
				Namespace: "default",
				Labels: map[string]string{
					renovatev1beta1.LabelRenovator: "renovator-id",
				},
			},
			Spec: renovatev1beta1.DiscoverySpec{
				JobSpec: renovatev1beta1.JobSpec{
					Schedule: "*/5 * * * *",
				},
				Filter: []string{"*"},
			},
		}
		dd := &DiscoveryCustomDefaulter{}
		Expect(dd.Default(ctx, instance)).To(Succeed())

		renovate = &renovatev1beta1.RenovateConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-renovate",
				Namespace: "default",
			},
			Spec: renovatev1beta1.RenovateConfigSpec{
				ImageSpec: renovatev1beta1.ImageSpec{
					Image:           "renovate/renovate:latest",
					ImagePullPolicy: corev1.PullAlways,
				},
				Platform: renovatev1beta1.PlatformSpec{
					Type: "github",
				},
			},
		}
		rd := &RenovateConfigCustomDefaulter{}
		Expect(rd.Default(ctx, renovate)).To(Succeed())

		now = time.Date(2026, 2, 27, 15, 0, 0, 0, time.UTC)
		fakeClock = fakeclock.NewFakeClock(now)

		fakeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(instance, renovate).
			WithStatusSubresource(instance).
			Build()

		reconciler = &Reconciler{
			Client:    fakeClient,
			scheme:    scheme,
			scheduler: scheduler.NewManager(fakeClient, scheme, fakeClock),
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: instance.Namespace,
					Name:      instance.Name,
				},
			},
			instance: instance,
			renovate: renovate,
		}
	})

	Describe("reconcileJob", func() {
		expectedLabels := func() map[string]string {
			expected := DiscoveryLabels(reconciler.req)

			if val, ok := instance.Labels[renovatev1beta1.LabelRenovator]; ok {
				expected[renovatev1beta1.LabelRenovator] = val
			}

			return expected
		}

		Context("when discovery is suspended", func() {
			BeforeEach(func() {
				suspended := true
				instance.Spec.Suspend = &suspended
				Expect(fakeClient.Update(ctx, instance)).To(Succeed())
			})

			It("should skip job creation", func() {
				result, err := reconciler.reconcileJob(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(&ctrl.Result{}))
			})
		})

		Context("when discovery is suspended but manually triggered", func() {
			BeforeEach(func() {
				suspended := true
				instance.Spec.Suspend = &suspended
				instance.Annotations = map[string]string{
					"renovate.thegeeklab.de/operation": "discover",
				}
				Expect(fakeClient.Update(ctx, instance)).To(Succeed())
			})

			It("should create a job and remove the annotation", func() {
				_, err := reconciler.reconcileJob(ctx)
				Expect(err).NotTo(HaveOccurred())

				jobList := &batchv1.JobList{}
				Expect(fakeClient.List(ctx, jobList, client.InNamespace("default"))).To(Succeed())
				Expect(jobList.Items).To(HaveLen(1))

				job := jobList.Items[0]
				Expect(job.GenerateName).To(HavePrefix("test-discovery-"))

				updatedInstance := &renovatev1beta1.Discovery{}
				Expect(fakeClient.Get(ctx, reconciler.req.NamespacedName, updatedInstance)).To(Succeed())
				Expect(updatedInstance.Annotations).NotTo(HaveKey("renovate.thegeeklab.de/operation"))

				Expect(updatedInstance.Status.LastScheduleTime).NotTo(BeNil())
			})
		})

		Context("when there are active jobs", func() {
			BeforeEach(func() {
				activeJob := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "active-job",
						Namespace: "default",
						Labels:    expectedLabels(),
					},
					Status: batchv1.JobStatus{
						Active: 1,
					},
				}
				Expect(fakeClient.Create(ctx, activeJob)).To(Succeed())
			})

			It("should requeue after 1 minute", func() {
				result, err := reconciler.reconcileJob(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.RequeueAfter).To(Equal(1 * time.Minute))
			})
		})

		Context("when job should run based on schedule", func() {
			It("should create a new job", func() {
				_, err := reconciler.reconcileJob(ctx)
				Expect(err).NotTo(HaveOccurred())

				jobList := &batchv1.JobList{}
				Expect(fakeClient.List(ctx, jobList, client.InNamespace("default"))).To(Succeed())
				Expect(jobList.Items).To(HaveLen(1))

				job := jobList.Items[0]
				Expect(job.GenerateName).To(HavePrefix("test-discovery-"))
				Expect(job.Namespace).To(Equal("default"))
				Expect(job.Labels).To(Equal(expectedLabels()))
			})

			It("should update status after job creation", func() {
				_, err := reconciler.reconcileJob(ctx)
				Expect(err).NotTo(HaveOccurred())

				updatedInstance := &renovatev1beta1.Discovery{}
				Expect(fakeClient.Get(ctx, reconciler.req.NamespacedName, updatedInstance)).To(Succeed())
				Expect(updatedInstance.Status.LastScheduleTime).NotTo(BeNil())
			})
		})
	})

	Describe("updateJob", func() {
		It("should configure job with init and main containers", func() {
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
			}
			reconciler.updateJob(job, nil)

			Expect(job.Spec.Template.Spec.InitContainers).To(HaveLen(1))
			Expect(job.Spec.Template.Spec.InitContainers[0].Name).To(Equal("renovate-init"))

			Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(job.Spec.Template.Spec.Containers[0].Name).To(Equal("renovate-discovery"))

			expectedServiceAccountName := metadata.GenericMetadata(reconciler.req).Name
			Expect(job.Spec.Template.Spec.ServiceAccountName).To(Equal(expectedServiceAccountName))
		})

		It("should propagate ImagePullSecrets to the job pod spec", func() {
			instance.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
				{Name: "discovery-registry-secret"},
			}

			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
			}
			reconciler.updateJob(job, nil)

			Expect(job.Spec.Template.Spec.ImagePullSecrets).To(HaveLen(1))
			Expect(job.Spec.Template.Spec.ImagePullSecrets[0].Name).To(Equal("discovery-registry-secret"))
		})

		It("should propagate NodeSelector to the job pod spec", func() {
			instance.Spec.NodeSelector = map[string]string{"disktype": "ssd"}

			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
			}
			reconciler.updateJob(job, nil)

			Expect(job.Spec.Template.Spec.NodeSelector).To(HaveKeyWithValue("disktype", "ssd"))
		})

		It("should propagate Affinity to the job pod spec", func() {
			instance.Spec.Affinity = &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{MatchExpressions: []corev1.NodeSelectorRequirement{
								{Key: "kubernetes.io/e2e-az-name", Operator: corev1.NodeSelectorOpIn, Values: []string{"e2e-az1"}},
							}},
						},
					},
				},
			}

			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
			}
			reconciler.updateJob(job, nil)

			Expect(job.Spec.Template.Spec.Affinity).NotTo(BeNil())
			Expect(job.Spec.Template.Spec.Affinity.NodeAffinity).NotTo(BeNil())
		})

		It("should propagate Tolerations to the job pod spec", func() {
			instance.Spec.Tolerations = []corev1.Toleration{
				{Key: "key1", Operator: corev1.TolerationOpEqual, Value: "value1", Effect: corev1.TaintEffectNoSchedule},
			}

			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
			}
			reconciler.updateJob(job, nil)

			Expect(job.Spec.Template.Spec.Tolerations).To(HaveLen(1))
			Expect(job.Spec.Template.Spec.Tolerations[0].Key).To(Equal("key1"))
		})

		It("should propagate TopologySpreadConstraints to the job pod spec", func() {
			instance.Spec.TopologySpreadConstraints = []corev1.TopologySpreadConstraint{
				{MaxSkew: 1, TopologyKey: "zone", WhenUnsatisfiable: corev1.DoNotSchedule},
			}

			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
			}
			reconciler.updateJob(job, nil)

			Expect(job.Spec.Template.Spec.TopologySpreadConstraints).To(HaveLen(1))
			Expect(job.Spec.Template.Spec.TopologySpreadConstraints[0].TopologyKey).To(Equal("zone"))
		})
	})
})
