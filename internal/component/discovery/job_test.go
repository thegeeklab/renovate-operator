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
	"k8s.io/apimachinery/pkg/api/resource"
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
			expected, err := DiscoveryLabels(reconciler.req)
			Expect(err).NotTo(HaveOccurred())

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

		It("should propagate Resources to the job container", func() {
			instance.Spec.Resources = corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("100m"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("500m"),
				},
			}

			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
			}
			reconciler.updateJob(job, nil)

			mainContainer := job.Spec.Template.Spec.Containers[0]
			Expect(mainContainer.Resources.Requests).To(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("100m")))
			Expect(mainContainer.Resources.Limits).To(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("500m")))
		})

		It("should propagate SecurityContext to the job container", func() {
			instance.Spec.SecurityContext = &corev1.SecurityContext{
				RunAsNonRoot: new(true),
			}

			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
			}
			reconciler.updateJob(job, nil)

			mainContainer := job.Spec.Template.Spec.Containers[0]
			Expect(mainContainer.SecurityContext).NotTo(BeNil())
			Expect(*mainContainer.SecurityContext.RunAsNonRoot).To(BeTrue())
		})

		It("should propagate ExtraEnv to the job container", func() {
			instance.Spec.ExtraEnv = []corev1.EnvVar{
				{Name: "CUSTOM_VAR", Value: "custom_value"},
			}

			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
			}
			reconciler.updateJob(job, nil)

			env := job.Spec.Template.Spec.Containers[0].Env
			Expect(env).To(ContainElement(HaveField("Name", "CUSTOM_VAR")))
			Expect(env).To(ContainElement(HaveField("Value", "custom_value")))
		})

		It("should propagate ExtraVolumes to the job pod spec", func() {
			instance.Spec.ExtraVolumes = []corev1.Volume{
				{
					Name: "extra-vol",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
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

			Expect(job.Spec.Template.Spec.Volumes).To(ContainElement(HaveField("Name", "extra-vol")))
		})
	})

	Describe("updateJobStatus", func() {
		labels := func() map[string]string {
			l, err := DiscoveryLabels(reconciler.req)
			Expect(err).NotTo(HaveOccurred())

			if val, ok := instance.Labels[renovatev1beta1.LabelRenovator]; ok {
				l[renovatev1beta1.LabelRenovator] = val
			}

			return l
		}

		It("should set JobRunning=True when an active job exists", func() {
			activeJob := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "active-discovery-job",
					Namespace: "default",
					Labels:    labels(),
				},
				Status: batchv1.JobStatus{
					Active: 1,
				},
			}
			Expect(fakeClient.Create(ctx, activeJob)).To(Succeed())

			err := reconciler.updateJobStatus(ctx, labels())
			Expect(err).NotTo(HaveOccurred())

			cond := instance.GetCondition(renovatev1beta1.DiscoveryConditionDiscoveryRunning)
			Expect(cond).NotTo(BeNil())
			Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			Expect(cond.Reason).To(Equal("JobActive"))
		})

		It("should set JobRunning=False when no active job exists", func() {
			err := reconciler.updateJobStatus(ctx, labels())
			Expect(err).NotTo(HaveOccurred())

			cond := instance.GetCondition(renovatev1beta1.DiscoveryConditionDiscoveryRunning)
			Expect(cond).NotTo(BeNil())
			Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).To(Equal("NoJobActive"))
		})

		It("should set JobCompleted=True when latest finished job succeeded", func() {
			completedJob := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "completed-discovery-job",
					Namespace:         "default",
					CreationTimestamp: metav1.Now(),
					Labels:            labels(),
				},
				Status: batchv1.JobStatus{
					Succeeded: 1,
					Conditions: []batchv1.JobCondition{
						{
							Type:   batchv1.JobComplete,
							Status: corev1.ConditionTrue,
						},
					},
				},
			}
			Expect(fakeClient.Create(ctx, completedJob)).To(Succeed())

			err := reconciler.updateJobStatus(ctx, labels())
			Expect(err).NotTo(HaveOccurred())

			completedCond := instance.GetCondition(renovatev1beta1.DiscoveryConditionDiscoveryCompleted)
			Expect(completedCond).NotTo(BeNil())
			Expect(completedCond.Status).To(Equal(metav1.ConditionTrue))
			Expect(completedCond.Reason).To(Equal("JobSucceeded"))

			failedCond := instance.GetCondition(renovatev1beta1.DiscoveryConditionDiscoveryFailed)
			Expect(failedCond).To(BeNil())
		})

		It("should set JobFailed=True when latest finished job failed", func() {
			failedJob := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "failed-discovery-job",
					Namespace:         "default",
					CreationTimestamp: metav1.Now(),
					Labels:            labels(),
				},
				Status: batchv1.JobStatus{
					Failed: 1,
					Conditions: []batchv1.JobCondition{
						{
							Type:   batchv1.JobFailed,
							Status: corev1.ConditionTrue,
						},
					},
				},
			}
			Expect(fakeClient.Create(ctx, failedJob)).To(Succeed())

			err := reconciler.updateJobStatus(ctx, labels())
			Expect(err).NotTo(HaveOccurred())

			failedCond := instance.GetCondition(renovatev1beta1.DiscoveryConditionDiscoveryFailed)
			Expect(failedCond).NotTo(BeNil())
			Expect(failedCond.Status).To(Equal(metav1.ConditionTrue))
			Expect(failedCond.Reason).To(Equal("JobFailed"))

			completedCond := instance.GetCondition(renovatev1beta1.DiscoveryConditionDiscoveryCompleted)
			Expect(completedCond).To(BeNil())
		})

		It("should transition from completed to failed when a newer job fails", func() {
			firstJob := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "first-success-job",
					Namespace:         "default",
					CreationTimestamp: metav1.Now(),
					Labels:            labels(),
				},
				Status: batchv1.JobStatus{
					Succeeded: 1,
					Conditions: []batchv1.JobCondition{
						{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
					},
				},
			}
			Expect(fakeClient.Create(ctx, firstJob)).To(Succeed())

			err := reconciler.updateJobStatus(ctx, labels())
			Expect(err).NotTo(HaveOccurred())

			Expect(instance.GetCondition(renovatev1beta1.DiscoveryConditionDiscoveryCompleted)).NotTo(BeNil())
			Expect(instance.GetCondition(renovatev1beta1.DiscoveryConditionDiscoveryFailed)).To(BeNil())

			secondJob := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "second-fail-job",
					Namespace:         "default",
					CreationTimestamp: metav1.NewTime(firstJob.CreationTimestamp.Add(time.Minute)),
					Labels:            labels(),
				},
				Status: batchv1.JobStatus{
					Failed: 1,
					Conditions: []batchv1.JobCondition{
						{Type: batchv1.JobFailed, Status: corev1.ConditionTrue},
					},
				},
			}
			Expect(fakeClient.Create(ctx, secondJob)).To(Succeed())

			err = reconciler.updateJobStatus(ctx, labels())
			Expect(err).NotTo(HaveOccurred())

			Expect(instance.GetCondition(renovatev1beta1.DiscoveryConditionDiscoveryFailed)).NotTo(BeNil())
			Expect(instance.GetCondition(renovatev1beta1.DiscoveryConditionDiscoveryCompleted)).To(BeNil())
		})

		It("should report Running=True and Completed=True when both active and finished jobs exist", func() {
			activeJob := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "active-job",
					Namespace: "default",
					Labels:    labels(),
				},
				Status: batchv1.JobStatus{
					Active: 1,
				},
			}
			Expect(fakeClient.Create(ctx, activeJob)).To(Succeed())

			finishedJob := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "previous-done-job",
					Namespace:         "default",
					CreationTimestamp: metav1.Now(),
					Labels:            labels(),
				},
				Status: batchv1.JobStatus{
					Succeeded: 1,
					Conditions: []batchv1.JobCondition{
						{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
					},
				},
			}
			Expect(fakeClient.Create(ctx, finishedJob)).To(Succeed())

			err := reconciler.updateJobStatus(ctx, labels())
			Expect(err).NotTo(HaveOccurred())

			runningCond := instance.GetCondition(renovatev1beta1.DiscoveryConditionDiscoveryRunning)
			Expect(runningCond).NotTo(BeNil())
			Expect(runningCond.Status).To(Equal(metav1.ConditionTrue))

			completedCond := instance.GetCondition(renovatev1beta1.DiscoveryConditionDiscoveryCompleted)
			Expect(completedCond).NotTo(BeNil())
			Expect(completedCond.Status).To(Equal(metav1.ConditionTrue))
		})
	})
})
