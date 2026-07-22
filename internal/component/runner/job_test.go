package runner

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/thegeeklab/renovate-operator/internal/webhook/v1beta1"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/mock"
	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/logreader"
	"github.com/thegeeklab/renovate-operator/internal/logreader/mocks"
	"github.com/thegeeklab/renovate-operator/internal/metadata"
	"github.com/thegeeklab/renovate-operator/internal/metrics"
	"github.com/thegeeklab/renovate-operator/internal/scheduler"
	"github.com/thegeeklab/renovate-operator/pkg/util/k8s"
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
		instance   *renovatev1beta1.Runner
		renovate   *renovatev1beta1.RenovateConfig
		repo1      *renovatev1beta1.GitRepo
		repo2      *renovatev1beta1.GitRepo
		repo3      *renovatev1beta1.GitRepo
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

		instance = &renovatev1beta1.Runner{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-runner",
				Namespace: "default",
				Labels: map[string]string{
					renovatev1beta1.LabelRenovator: "renovator-id",
				},
			},
			Spec: renovatev1beta1.RunnerSpec{
				JobSpec: renovatev1beta1.JobSpec{
					Schedule: "*/5 * * * *",
				},
			},
		}
		rr := &RunnerCustomDefaulter{}
		Expect(rr.Default(ctx, instance)).To(Succeed())

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

		repo1 = &renovatev1beta1.GitRepo{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "repo-1",
				Namespace: instance.Namespace,
				Labels: map[string]string{
					renovatev1beta1.LabelRenovator: "renovator-id",
				},
			},
			Spec: renovatev1beta1.GitRepoSpec{Name: "test/repo-1"},
		}
		repo2 = &renovatev1beta1.GitRepo{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "repo-2",
				Namespace: instance.Namespace,
				Labels: map[string]string{
					renovatev1beta1.LabelRenovator: "renovator-id",
				},
			},
			Spec: renovatev1beta1.GitRepoSpec{Name: "test/repo-2"},
		}

		repo3 = &renovatev1beta1.GitRepo{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "repo-ignored",
				Namespace: instance.Namespace,
				Labels: map[string]string{
					renovatev1beta1.LabelRenovator: "different-id",
				},
			},
			Spec: renovatev1beta1.GitRepoSpec{Name: "test/repo-ignored"},
		}

		now = time.Date(2026, 2, 27, 15, 0, 0, 0, time.UTC)
		fakeClock = fakeclock.NewFakeClock(now)

		fakeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(instance, renovate, repo1, repo2, repo3).
			WithStatusSubresource(instance, repo1, repo2, repo3).
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
			metrics:  nil,
		}
	})

	Describe("reconcileJob", func() {
		expectedLabels := func(repoName string) map[string]string {
			expected, err := RunnerLabels(reconciler.req)
			Expect(err).NotTo(HaveOccurred())

			if val, ok := instance.Labels[renovatev1beta1.LabelRenovator]; ok {
				expected[renovatev1beta1.LabelRenovator] = val
			}

			if repoName != "" {
				label, err := k8s.SanitizeLabel(repoName)
				Expect(err).NotTo(HaveOccurred())

				expected[renovatev1beta1.LabelGitRepo] = label
			}

			return expected
		}

		Context("when runner is suspended", func() {
			BeforeEach(func() {
				suspended := true
				instance.Spec.Suspend = &suspended
				Expect(fakeClient.Update(ctx, instance)).To(Succeed())
			})

			It("should skip job creation", func() {
				result, err := reconciler.reconcileJob(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(&ctrl.Result{}))

				jobList := &batchv1.JobList{}
				Expect(fakeClient.List(ctx, jobList, client.InNamespace("default"))).To(Succeed())
				Expect(jobList.Items).To(BeEmpty())
			})
		})

		Context("when runner is suspended but globally manually triggered", func() {
			BeforeEach(func() {
				suspended := true
				instance.Spec.Suspend = &suspended
				instance.Annotations = map[string]string{
					"renovate.thegeeklab.de/operation": "renovate",
				}
				Expect(fakeClient.Update(ctx, instance)).To(Succeed())
			})

			It("should create jobs for matching repos and remove the runner annotation", func() {
				_, err := reconciler.reconcileJob(ctx)
				Expect(err).NotTo(HaveOccurred())

				jobList := &batchv1.JobList{}
				Expect(fakeClient.List(ctx, jobList, client.InNamespace("default"))).To(Succeed())
				Expect(jobList.Items).To(HaveLen(2))

				updatedInstance := &renovatev1beta1.Runner{}
				Expect(fakeClient.Get(ctx, reconciler.req.NamespacedName, updatedInstance)).To(Succeed())
				Expect(updatedInstance.Annotations).NotTo(HaveKey("renovate.thegeeklab.de/operation"))
				Expect(updatedInstance.Status.LastScheduleTime).NotTo(BeNil())
			})
		})

		Context("when runner is suspended but a specific GitRepo is manually triggered", func() {
			BeforeEach(func() {
				suspended := true
				instance.Spec.Suspend = &suspended
				Expect(fakeClient.Update(ctx, instance)).To(Succeed())

				repo1.Annotations = map[string]string{
					"renovate.thegeeklab.de/operation": "renovate",
				}
				Expect(fakeClient.Update(ctx, repo1)).To(Succeed())
			})

			It("should create a job ONLY for the triggered repo and remove its annotation", func() {
				result, err := reconciler.reconcileJob(ctx)
				Expect(err).NotTo(HaveOccurred())

				Expect(result).To(Equal(&ctrl.Result{}))

				jobList := &batchv1.JobList{}
				Expect(fakeClient.List(ctx, jobList, client.InNamespace("default"))).To(Succeed())
				Expect(jobList.Items).To(HaveLen(1))

				job := jobList.Items[0]
				Expect(job.GenerateName).To(HavePrefix("repo-1-"))
				Expect(job.Labels).To(Equal(expectedLabels("repo-1")))

				updatedRepo := &renovatev1beta1.GitRepo{}
				repoKey := types.NamespacedName{Name: repo1.Name, Namespace: repo1.Namespace}
				Expect(fakeClient.Get(ctx, repoKey, updatedRepo)).To(Succeed())
				Expect(updatedRepo.Annotations).NotTo(HaveKey("renovate.thegeeklab.de/operation"))

				updatedInstance := &renovatev1beta1.Runner{}
				Expect(fakeClient.Get(ctx, reconciler.req.NamespacedName, updatedInstance)).To(Succeed())
				Expect(updatedInstance.Status.LastScheduleTime).To(BeNil())
			})
		})

		Context("when there is an active job for one of the repos", func() {
			BeforeEach(func() {
				activeJob := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "active-job-repo-1",
						Namespace: "default",
						Labels:    expectedLabels("repo-1"),
					},
					Status: batchv1.JobStatus{
						Active: 1,
					},
				}
				Expect(fakeClient.Create(ctx, activeJob)).To(Succeed())
			})

			It("should skip the active repo but create a job for the other matching repos", func() {
				_, err := reconciler.reconcileJob(ctx)
				Expect(err).NotTo(HaveOccurred())

				jobList := &batchv1.JobList{}
				Expect(fakeClient.List(ctx, jobList, client.InNamespace("default"))).To(Succeed())

				Expect(jobList.Items).To(HaveLen(2))

				newJobsFound := 0

				for _, job := range jobList.Items {
					if job.Name != "active-job-repo-1" {
						Expect(job.GenerateName).To(HavePrefix("repo-2-"))
						Expect(job.Labels).To(Equal(expectedLabels("repo-2")))

						newJobsFound++
					}
				}

				Expect(newJobsFound).To(Equal(1))
			})
		})

		Context("when job should run globally based on schedule", func() {
			It("should create new jobs for all matching repos and update status", func() {
				_, err := reconciler.reconcileJob(ctx)
				Expect(err).NotTo(HaveOccurred())

				jobList := &batchv1.JobList{}
				Expect(fakeClient.List(ctx, jobList, client.InNamespace("default"))).To(Succeed())
				Expect(jobList.Items).To(HaveLen(2))

				for _, job := range jobList.Items {
					repoName := job.Labels[renovatev1beta1.LabelGitRepo]
					Expect(job.Labels).To(Equal(expectedLabels(repoName)))
				}

				updatedInstance := &renovatev1beta1.Runner{}
				Expect(fakeClient.Get(ctx, reconciler.req.NamespacedName, updatedInstance)).To(Succeed())
				Expect(updatedInstance.Status.LastScheduleTime).NotTo(BeNil())
			})
		})

		Context("when repo name exceeds 63 characters", func() {
			var longRepo *renovatev1beta1.GitRepo

			BeforeEach(func() {
				longName := "very-long-organization-name-very-long-repository-name-that-exceeds-limit"
				longRepo = &renovatev1beta1.GitRepo{
					ObjectMeta: metav1.ObjectMeta{
						Name:      longName,
						Namespace: instance.Namespace,
						Labels: map[string]string{
							renovatev1beta1.LabelRenovator: "renovator-id",
						},
					},
					Spec: renovatev1beta1.GitRepoSpec{Name: longName},
				}
				Expect(fakeClient.Create(ctx, longRepo)).To(Succeed())
			})

			It("should create jobs with truncated label values", func() {
				_, err := reconciler.reconcileJob(ctx)
				Expect(err).NotTo(HaveOccurred())

				jobList := &batchv1.JobList{}
				Expect(fakeClient.List(ctx, jobList, client.InNamespace("default"))).To(Succeed())

				longRepoLabel, err := k8s.SanitizeLabel(longRepo.Name)
				Expect(err).NotTo(HaveOccurred())

				var longRepoJob *batchv1.Job

				for i := range jobList.Items {
					if jobList.Items[i].Labels[renovatev1beta1.LabelGitRepo] == longRepoLabel {
						longRepoJob = &jobList.Items[i]

						break
					}
				}

				Expect(longRepoJob).NotTo(BeNil(), "job for long-named repo should exist")

				gitRepoLabel := longRepoJob.Labels[renovatev1beta1.LabelGitRepo]
				Expect(len(gitRepoLabel)).To(BeNumerically("<=", 63), "label value must not exceed 63 chars")
				Expect(gitRepoLabel).To(Equal(longRepoLabel))
			})

			It("should allow querying jobs by truncated label value", func() {
				_, err := reconciler.reconcileJob(ctx)
				Expect(err).NotTo(HaveOccurred())

				truncatedLabel, err := k8s.SanitizeLabel(longRepo.Name)
				Expect(err).NotTo(HaveOccurred())

				jobList := &batchv1.JobList{}
				err = fakeClient.List(
					ctx, jobList,
					client.InNamespace("default"),
					client.MatchingLabels{renovatev1beta1.LabelGitRepo: truncatedLabel},
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(jobList.Items).To(HaveLen(1), "should find job by truncated label")
			})
		})
	})

	Describe("updateJob", func() {
		It("should configure job with correct specifications for a GitRepo", func() {
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
			}
			reconciler.updateJob(job, repo1, nil)

			Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
			mainContainer := job.Spec.Template.Spec.Containers[0]
			Expect(mainContainer.Name).To(Equal("renovate"))
			Expect(mainContainer.Image).To(Equal("renovate/renovate:latest"))

			expectedSA := metadata.GenericMetadata(reconciler.req).Name
			Expect(job.Spec.Template.Spec.ServiceAccountName).To(Equal(expectedSA))
		})

		It("should propagate ImagePullSecrets to the job pod spec", func() {
			instance.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
				{Name: "runner-registry-secret"},
			}

			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
			}
			reconciler.updateJob(job, repo1, nil)

			Expect(job.Spec.Template.Spec.ImagePullSecrets).To(HaveLen(1))
			Expect(job.Spec.Template.Spec.ImagePullSecrets[0].Name).To(Equal("runner-registry-secret"))
		})

		It("should propagate NodeSelector to the job pod spec", func() {
			instance.Spec.NodeSelector = map[string]string{"disktype": "ssd"}

			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
			}
			reconciler.updateJob(job, repo1, nil)

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
			reconciler.updateJob(job, repo1, nil)

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
			reconciler.updateJob(job, repo1, nil)

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
			reconciler.updateJob(job, repo1, nil)

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
			reconciler.updateJob(job, repo1, nil)

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
			reconciler.updateJob(job, repo1, nil)

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
			reconciler.updateJob(job, repo1, nil)

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
			reconciler.updateJob(job, repo1, nil)

			Expect(job.Spec.Template.Spec.Volumes).To(ContainElement(HaveField("Name", "extra-vol")))
		})

		It("should propagate RuntimeClassName to the job pod spec", func() {
			instance.Spec.RuntimeClassName = new("gvisor")

			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
			}
			reconciler.updateJob(job, repo1, nil)

			Expect(job.Spec.Template.Spec.RuntimeClassName).NotTo(BeNil())
			Expect(*job.Spec.Template.Spec.RuntimeClassName).To(Equal("gvisor"))
		})

		It("should propagate PodAnnotations to the job pod spec", func() {
			instance.Spec.PodAnnotations = map[string]string{"prometheus.io/scrape": "true"}

			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
			}
			reconciler.updateJob(job, repo1, nil)

			Expect(job.Spec.Template.Annotations).To(HaveKeyWithValue("prometheus.io/scrape", "true"))
		})

		It("should propagate ScratchVolume to the job pod spec", func() {
			sizeLimit := resource.MustParse("1Gi")
			instance.Spec.ScratchVolume = &renovatev1beta1.ScratchVolumeSpec{
				Path:      "/scratch",
				Medium:    corev1.StorageMediumMemory,
				SizeLimit: &sizeLimit,
			}

			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
			}
			reconciler.updateJob(job, repo1, nil)

			var scratchVol *corev1.Volume

			for i := range job.Spec.Template.Spec.Volumes {
				if job.Spec.Template.Spec.Volumes[i].Name == "renovate-tmp" {
					scratchVol = &job.Spec.Template.Spec.Volumes[i]

					break
				}
			}

			Expect(scratchVol).NotTo(BeNil())
			Expect(scratchVol.EmptyDir).NotTo(BeNil())
			Expect(scratchVol.EmptyDir.Medium).To(Equal(corev1.StorageMediumMemory))
			Expect(scratchVol.EmptyDir.SizeLimit).To(Equal(&sizeLimit))

			env := job.Spec.Template.Spec.Containers[0].Env
			Expect(env).To(ContainElement(HaveField("Name", "RENOVATE_BASE_DIR")))
			Expect(env).To(ContainElement(HaveField("Value", "/scratch")))
		})
	})

	Describe("updateJobStatus", func() {
		var metricsRecorder metrics.Recorder

		BeforeEach(func() {
			reg := prometheus.NewRegistry()
			metricsRecorder = metrics.New(reg, reg, 5000)
			reconciler.metrics = metricsRecorder
		})

		It("should emit metrics when a job succeeds", func() {
			// Ensure repo doesn't have LastRenovateTime set
			repo1.Status.LastRenovateTime = nil
			Expect(fakeClient.Status().Update(ctx, repo1)).To(Succeed())

			finishedJob := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "finished-job",
					Namespace:         "default",
					CreationTimestamp: metav1.Now(),
					Labels: map[string]string{
						renovatev1beta1.LabelRenovator: "renovator-id",
						renovatev1beta1.LabelGitRepo:   "repo-1",
					},
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
			Expect(fakeClient.Create(ctx, finishedJob)).To(Succeed())

			err := reconciler.updateJobStatus(ctx, repo1, map[string]string{
				renovatev1beta1.LabelRenovator: "renovator-id",
				renovatev1beta1.LabelGitRepo:   "repo-1",
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify metrics were recorded
			metricFamilies, err := metricsRecorder.Gatherer().Gather()
			Expect(err).NotTo(HaveOccurred())
			Expect(metricFamilies).ToNot(BeEmpty())
		})

		It("should emit metrics when a job fails", func() {
			// Ensure repo doesn't have LastRenovateTime set
			repo2.Status.LastRenovateTime = nil
			Expect(fakeClient.Status().Update(ctx, repo2)).To(Succeed())

			failedJob := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "failed-job",
					Namespace:         "default",
					CreationTimestamp: metav1.Now(),
					Labels: map[string]string{
						renovatev1beta1.LabelRenovator: "renovator-id",
						renovatev1beta1.LabelGitRepo:   "repo-2",
					},
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

			err := reconciler.updateJobStatus(ctx, repo2, map[string]string{
				renovatev1beta1.LabelRenovator: "renovator-id",
				renovatev1beta1.LabelGitRepo:   "repo-2",
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify metrics were recorded
			metricFamilies, err := metricsRecorder.Gatherer().Gather()
			Expect(err).NotTo(HaveOccurred())
			Expect(metricFamilies).ToNot(BeEmpty())
		})

		It("should not double-count metrics on subsequent reconciles", func() {
			// Ensure repo doesn't have LastRenovateTime set
			repo1.Status.LastRenovateTime = nil
			Expect(fakeClient.Status().Update(ctx, repo1)).To(Succeed())

			finishedJob := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "finished-job-idempotent",
					Namespace:         "default",
					CreationTimestamp: metav1.Now(),
					Labels: map[string]string{
						renovatev1beta1.LabelRenovator: "renovator-id",
						renovatev1beta1.LabelGitRepo:   "repo-1",
					},
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
			Expect(fakeClient.Create(ctx, finishedJob)).To(Succeed())

			// First reconcile
			err := reconciler.updateJobStatus(ctx, repo1, map[string]string{
				renovatev1beta1.LabelRenovator: "renovator-id",
				renovatev1beta1.LabelGitRepo:   "repo-1",
			})
			Expect(err).NotTo(HaveOccurred())

			// Get metric count after first reconcile
			metricFamilies1, err := metricsRecorder.Gatherer().Gather()
			Expect(err).NotTo(HaveOccurred())

			// Second reconcile (same job, should not increment)
			err = reconciler.updateJobStatus(ctx, repo1, map[string]string{
				renovatev1beta1.LabelRenovator: "renovator-id",
				renovatev1beta1.LabelGitRepo:   "repo-1",
			})
			Expect(err).NotTo(HaveOccurred())

			// Get metric count after second reconcile
			metricFamilies2, err := metricsRecorder.Gatherer().Gather()
			Expect(err).NotTo(HaveOccurred())

			// Counts should be the same (idempotent)
			Expect(metricFamilies2).To(HaveLen(len(metricFamilies1)))
		})

		It("should correctly increment counter values across multiple runs", func() {
			// Ensure repo doesn't have LastRenovateTime set
			repo1.Status.LastRenovateTime = nil
			Expect(fakeClient.Status().Update(ctx, repo1)).To(Succeed())

			// First successful run
			job1 := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "job-1",
					Namespace:         "default",
					CreationTimestamp: metav1.Now(),
					Labels: map[string]string{
						renovatev1beta1.LabelRenovator: "renovator-id",
						renovatev1beta1.LabelGitRepo:   "repo-1",
					},
				},
				Status: batchv1.JobStatus{
					Succeeded: 1,
					Conditions: []batchv1.JobCondition{
						{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
					},
				},
			}
			Expect(fakeClient.Create(ctx, job1)).To(Succeed())

			err := reconciler.updateJobStatus(ctx, repo1, map[string]string{
				renovatev1beta1.LabelRenovator: "renovator-id",
				renovatev1beta1.LabelGitRepo:   "repo-1",
			})
			Expect(err).NotTo(HaveOccurred())

			// Get metric families and find the counter
			metricFamilies1, err := metricsRecorder.Gatherer().Gather()
			Expect(err).NotTo(HaveOccurred())

			var runsTotal1 float64

			for _, mf := range metricFamilies1 {
				if mf.GetName() == "renovate_operator_gitrepo_runs_total" {
					for _, m := range mf.GetMetric() {
						for _, label := range m.GetLabel() {
							if label.GetName() == "status" && label.GetValue() == "succeeded" {
								runsTotal1 = m.GetCounter().GetValue()
							}
						}
					}
				}
			}

			Expect(runsTotal1).To(Equal(float64(1)))

			// Update repo's LastRenovateTime to allow second run
			repo1.Status.LastRenovateTime = &metav1.Time{Time: job1.CreationTimestamp.Time}
			Expect(fakeClient.Status().Update(ctx, repo1)).To(Succeed())

			// Second successful run
			job2 := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "job-2",
					Namespace:         "default",
					CreationTimestamp: metav1.NewTime(job1.CreationTimestamp.Add(time.Minute)),
					Labels: map[string]string{
						renovatev1beta1.LabelRenovator: "renovator-id",
						renovatev1beta1.LabelGitRepo:   "repo-1",
					},
				},
				Status: batchv1.JobStatus{
					Succeeded: 1,
					Conditions: []batchv1.JobCondition{
						{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
					},
				},
			}
			Expect(fakeClient.Create(ctx, job2)).To(Succeed())

			err = reconciler.updateJobStatus(ctx, repo1, map[string]string{
				renovatev1beta1.LabelRenovator: "renovator-id",
				renovatev1beta1.LabelGitRepo:   "repo-1",
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify counter incremented
			metricFamilies2, err := metricsRecorder.Gatherer().Gather()
			Expect(err).NotTo(HaveOccurred())

			var runsTotal2 float64

			for _, mf := range metricFamilies2 {
				if mf.GetName() == "renovate_operator_gitrepo_runs_total" {
					for _, m := range mf.GetMetric() {
						for _, label := range m.GetLabel() {
							if label.GetName() == "status" && label.GetValue() == "succeeded" {
								runsTotal2 = m.GetCounter().GetValue()
							}
						}
					}
				}
			}

			Expect(runsTotal2).To(Equal(float64(2)))
		})

		It("should handle status transitions correctly", func() {
			// Ensure repo doesn't have LastRenovateTime set
			repo1.Status.LastRenovateTime = nil
			Expect(fakeClient.Status().Update(ctx, repo1)).To(Succeed())

			// First run succeeds
			job1 := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "job-success",
					Namespace:         "default",
					CreationTimestamp: metav1.Now(),
					Labels: map[string]string{
						renovatev1beta1.LabelRenovator: "renovator-id",
						renovatev1beta1.LabelGitRepo:   "repo-1",
					},
				},
				Status: batchv1.JobStatus{
					Succeeded: 1,
					Conditions: []batchv1.JobCondition{
						{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
					},
				},
			}
			Expect(fakeClient.Create(ctx, job1)).To(Succeed())

			err := reconciler.updateJobStatus(ctx, repo1, map[string]string{
				renovatev1beta1.LabelRenovator: "renovator-id",
				renovatev1beta1.LabelGitRepo:   "repo-1",
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify run_failed gauge is 0
			metricFamilies, err := metricsRecorder.Gatherer().Gather()
			Expect(err).NotTo(HaveOccurred())

			var runFailed float64

			for _, mf := range metricFamilies {
				if mf.GetName() == "renovate_operator_gitrepo_run_failed" {
					for _, m := range mf.GetMetric() {
						runFailed = m.GetGauge().GetValue()
					}
				}
			}

			Expect(runFailed).To(Equal(float64(0)))

			// Update repo's LastRenovateTime to allow second run
			repo1.Status.LastRenovateTime = &metav1.Time{Time: job1.CreationTimestamp.Time}
			Expect(fakeClient.Status().Update(ctx, repo1)).To(Succeed())

			// Second run fails
			job2 := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "job-fail",
					Namespace:         "default",
					CreationTimestamp: metav1.NewTime(job1.CreationTimestamp.Add(time.Minute)),
					Labels: map[string]string{
						renovatev1beta1.LabelRenovator: "renovator-id",
						renovatev1beta1.LabelGitRepo:   "repo-1",
					},
				},
				Status: batchv1.JobStatus{
					Failed: 1,
					Conditions: []batchv1.JobCondition{
						{Type: batchv1.JobFailed, Status: corev1.ConditionTrue},
					},
				},
			}
			Expect(fakeClient.Create(ctx, job2)).To(Succeed())

			err = reconciler.updateJobStatus(ctx, repo1, map[string]string{
				renovatev1beta1.LabelRenovator: "renovator-id",
				renovatev1beta1.LabelGitRepo:   "repo-1",
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify run_failed gauge is now 1
			metricFamilies, err = metricsRecorder.Gatherer().Gather()
			Expect(err).NotTo(HaveOccurred())

			for _, mf := range metricFamilies {
				if mf.GetName() == "renovate_operator_gitrepo_run_failed" {
					for _, m := range mf.GetMetric() {
						runFailed = m.GetGauge().GetValue()
					}
				}
			}

			Expect(runFailed).To(Equal(float64(1)))
		})

		It("should sanitize long repository names in metric labels", func() {
			// Create repo with very long name (>63 chars)
			longRepo := &renovatev1beta1.GitRepo{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "very-long-repository-name-that-exceeds-the-kubernetes-label-limit-of-63-characters",
					Namespace: "default",
					Labels: map[string]string{
						renovatev1beta1.LabelRenovator: "renovator-id",
					},
				},
				Spec: renovatev1beta1.GitRepoSpec{Name: "test/long-repo"},
			}
			Expect(fakeClient.Create(ctx, longRepo)).To(Succeed())

			// Ensure repo doesn't have LastRenovateTime set
			longRepo.Status.LastRenovateTime = nil
			Expect(fakeClient.Status().Update(ctx, longRepo)).To(Succeed())

			finishedJob := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "job-long-name",
					Namespace:         "default",
					CreationTimestamp: metav1.Now(),
					Labels: map[string]string{
						renovatev1beta1.LabelRenovator: "renovator-id",
						renovatev1beta1.LabelGitRepo:   "very-long-repository-name-that-exceeds-the-kubernetes-label",
					},
				},
				Status: batchv1.JobStatus{
					Succeeded: 1,
					Conditions: []batchv1.JobCondition{
						{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
					},
				},
			}
			Expect(fakeClient.Create(ctx, finishedJob)).To(Succeed())

			err := reconciler.updateJobStatus(ctx, longRepo, map[string]string{
				renovatev1beta1.LabelRenovator: "renovator-id",
				renovatev1beta1.LabelGitRepo:   "very-long-repository-name-that-exceeds-the-kubernetes-label",
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify metrics were recorded with sanitized label
			metricFamilies, err := metricsRecorder.Gatherer().Gather()
			Expect(err).NotTo(HaveOccurred())

			var foundSanitizedLabel bool

			for _, mf := range metricFamilies {
				if mf.GetName() == "renovate_operator_gitrepo_runs_total" {
					for _, m := range mf.GetMetric() {
						for _, label := range m.GetLabel() {
							if label.GetName() == "gitrepo" {
								// Label should be sanitized and <= 63 chars
								Expect(len(label.GetValue())).To(BeNumerically("<=", 63))

								foundSanitizedLabel = true
							}
						}
					}
				}
			}

			Expect(foundSanitizedLabel).To(BeTrue())
		})
	})

	Describe("updateLogMetrics", func() {
		var (
			metricsRecorder metrics.Recorder
			testJob         *batchv1.Job
		)

		BeforeEach(func() {
			reg := prometheus.NewRegistry()
			metricsRecorder = metrics.New(reg, reg, 5000)
			reconciler.metrics = metricsRecorder

			testJob = &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
					UID:       types.UID("test-uid"),
				},
			}

			testPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Labels:    map[string]string{"job-name": "test-job"},
				},
				Status: corev1.PodStatus{Phase: corev1.PodSucceeded},
			}

			Expect(fakeClient.Create(ctx, testJob)).To(Succeed())
			Expect(fakeClient.Create(ctx, testPod)).To(Succeed())
		})

		It("sets dependency_issues=1 when logs have warnings", func() {
			reconciler.logReader = newLogReaderMock(`{"level":40,"msg":"Config warning"}`, nil)

			reconciler.updateLogMetrics(ctx, testJob, "renovator-1", "repo-1")

			metricFamilies, err := metricsRecorder.Gatherer().Gather()
			Expect(err).NotTo(HaveOccurred())

			var issues float64

			for _, mf := range metricFamilies {
				if mf.GetName() == "renovate_operator_gitrepo_dependency_issues" {
					for _, m := range mf.GetMetric() {
						issues = m.GetGauge().GetValue()
					}
				}
			}

			Expect(issues).To(Equal(float64(1)))
		})

		It("sets dependency_issues=0 when logs are clean", func() {
			reconciler.logReader = newLogReaderMock(`{"level":30,"msg":"Repository finished","result":"done"}`, nil)

			reconciler.updateLogMetrics(ctx, testJob, "renovator-1", "repo-1")

			metricFamilies, err := metricsRecorder.Gatherer().Gather()
			Expect(err).NotTo(HaveOccurred())

			var issues float64

			for _, mf := range metricFamilies {
				if mf.GetName() == "renovate_operator_gitrepo_dependency_issues" {
					for _, m := range mf.GetMetric() {
						issues = m.GetGauge().GetValue()
					}
				}
			}

			Expect(issues).To(Equal(float64(0)))
		})

		It("sets approvals_needed count from log entries", func() {
			ba := `{"branchName":"renovate/dep-a","prNo":null,"prTitle":"Update dep-a","result":"needs-approval"}`
			bb := `{"branchName":"renovate/dep-b","prNo":null,"prTitle":"Update dep-b","result":"needs-approval"}`
			bi := `{"level":30,"msg":"branches info extended","branchesInformation":[` + ba + `,` + bb + `]}`
			reconciler.logReader = newLogReaderMock(bi, nil)

			reconciler.updateLogMetrics(ctx, testJob, "renovator-1", "repo-1")

			metricFamilies, err := metricsRecorder.Gatherer().Gather()
			Expect(err).NotTo(HaveOccurred())

			var approvals float64

			for _, mf := range metricFamilies {
				if mf.GetName() == "renovate_operator_gitrepo_approvals_needed" {
					for _, m := range mf.GetMetric() {
						approvals = m.GetGauge().GetValue()
					}
				}
			}

			Expect(approvals).To(Equal(float64(2)))
		})

		It("does nothing when logReader is nil", func() {
			reconciler.logReader = nil

			reconciler.updateLogMetrics(ctx, testJob, "renovator-1", "repo-1")

			metricFamilies, err := metricsRecorder.Gatherer().Gather()
			Expect(err).NotTo(HaveOccurred())

			for _, mf := range metricFamilies {
				if mf.GetName() == "renovate_operator_gitrepo_dependency_issues" {
					Expect(mf.GetMetric()).To(BeEmpty())
				}

				if mf.GetName() == "renovate_operator_gitrepo_approvals_needed" {
					Expect(mf.GetMetric()).To(BeEmpty())
				}
			}
		})

		It("does not crash when logReader returns an error", func() {
			reconciler.logReader = newLogReaderMock("", errors.New("pod not found"))

			Expect(func() {
				reconciler.updateLogMetrics(ctx, testJob, "renovator-1", "repo-1")
			}).NotTo(Panic())
		})

		It("sets both dependency_issues and approvals_needed from the same log", func() {
			errLog := `{"level":50,"msg":"Fatal error in dependency lookup"}`
			ba := `{"branchName":"renovate/dep-x","prNo":null,"prTitle":"Update dep-x","result":"needs-approval"}`
			bi := `{"level":30,"msg":"branches info extended","branchesInformation":[` + ba + `]}`

			reconciler.logReader = newLogReaderMock(errLog+"\n"+bi, nil)

			reconciler.updateLogMetrics(ctx, testJob, "renovator-1", "repo-1")

			metricFamilies, err := metricsRecorder.Gatherer().Gather()
			Expect(err).NotTo(HaveOccurred())

			var issues, approvals float64

			for _, mf := range metricFamilies {
				switch mf.GetName() {
				case "renovate_operator_gitrepo_dependency_issues":
					for _, m := range mf.GetMetric() {
						issues = m.GetGauge().GetValue()
					}
				case "renovate_operator_gitrepo_approvals_needed":
					for _, m := range mf.GetMetric() {
						approvals = m.GetGauge().GetValue()
					}
				}
			}

			Expect(issues).To(Equal(float64(1)))
			Expect(approvals).To(Equal(float64(1)))
		})
	})
})

func newLogReaderMock(logs string, err error) *mocks.Reader {
	m := &mocks.Reader{}
	m.On("ReadJobLogs", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(io.NopCloser(strings.NewReader(logs)), err)

	return m
}

var _ logreader.Reader = (*mocks.Reader)(nil)
