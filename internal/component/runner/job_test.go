package runner

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/metadata"
	"github.com/thegeeklab/renovate-operator/internal/webhook/v1beta1"
	. "github.com/thegeeklab/renovate-operator/internal/webhook/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
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
		ctx        context.Context
		scheme     *runtime.Scheme
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
			},
			Spec: renovatev1beta1.RunnerSpec{
				JobSpec: renovatev1beta1.JobSpec{
					Schedule: "*/5 * * * *",
				},
			},
		}

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

		_ = (&RunnerCustomDefaulter{}).Default(ctx, instance)
		_ = (&v1beta1.RenovateConfigCustomDefaulter{}).Default(ctx, renovate)

		fakeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(instance, renovate).
			WithStatusSubresource(instance).
			Build()

		reconciler = &Reconciler{
			Client: fakeClient,
			scheme: scheme,
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
			})
		})

		Context("when there are active jobs", func() {
			BeforeEach(func() {
				activeJob := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "active-job",
						Namespace: "default",
						Labels:    RunnerMetadata(reconciler.req).Labels,
					},
					Status: batchv1.JobStatus{
						Active: 1,
					},
				}
				Expect(fakeClient.Create(ctx, activeJob)).To(Succeed())
			})

			It("should requeue", func() {
				result, err := reconciler.reconcileJob(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.RequeueAfter).To(BeNumerically(">", 0))
			})
		})

		Context("when job should run based on schedule", func() {
			BeforeEach(func() {
				gitRepo := &renovatev1beta1.GitRepo{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-repo",
						Namespace: instance.Namespace,
					},
					Spec: renovatev1beta1.GitRepoSpec{
						Name: "test/repository",
					},
				}
				Expect(fakeClient.Create(ctx, gitRepo)).To(Succeed())
			})

			It("should create a new job and update status", func() {
				_, err := reconciler.reconcileJob(ctx)
				Expect(err).NotTo(HaveOccurred())

				jobList := &batchv1.JobList{}
				Expect(fakeClient.List(ctx, jobList, client.InNamespace("default"))).To(Succeed())
				Expect(jobList.Items).To(HaveLen(1))

				job := jobList.Items[0]
				Expect(job.Name).To(HavePrefix("test-repo-"))
				Expect(job.Labels).To(Equal(map[string]string{
					renovatev1beta1.RenovatorLabel:   instance.Labels[renovatev1beta1.RenovatorLabel],
					"renovate.thegeeklab.de/gitrepo": "test-repo",
				}))

				updatedInstance := &renovatev1beta1.Runner{}
				Expect(fakeClient.Get(ctx, reconciler.req.NamespacedName, updatedInstance)).To(Succeed())
				Expect(updatedInstance.Status.LastScheduleTime).NotTo(BeNil())
			})
		})
	})

	Describe("updateJob", func() {
		It("should configure job with correct specifications", func() {
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
			}
			gitRepo := &renovatev1beta1.GitRepo{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo",
					Namespace: "default",
				},
			}
			reconciler.updateJob(job, gitRepo)

			Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
			mainContainer := job.Spec.Template.Spec.Containers[0]
			Expect(mainContainer.Name).To(Equal("renovate"))
			Expect(mainContainer.Image).To(Equal("renovate/renovate:latest"))

			expectedSA := metadata.GenericMetadata(reconciler.req).Name
			Expect(job.Spec.Template.Spec.ServiceAccountName).To(Equal(expectedSA))
		})
	})

	Describe("evaluateSchedule", func() {
		Context("when immediate execution annotation is present", func() {
			BeforeEach(func() {
				instance.Annotations = map[string]string{
					"renovate.thegeeklab.de/renovator-operation": "renovate",
				}
				Expect(fakeClient.Update(ctx, instance)).To(Succeed())
			})

			It("should return true for immediate execution", func() {
				shouldRun, _, err := reconciler.evaluateSchedule()
				Expect(err).NotTo(HaveOccurred())
				Expect(shouldRun).To(BeTrue())
			})
		})

		Context("when schedule is due", func() {
			It("should return true when last run time is zero", func() {
				shouldRun, _, err := reconciler.evaluateSchedule()
				Expect(err).NotTo(HaveOccurred())
				Expect(shouldRun).To(BeTrue())
			})
		})

		Context("when schedule is not yet due", func() {
			BeforeEach(func() {
				now := metav1.NewTime(time.Now())
				instance.Status.LastScheduleTime = &now
				Expect(fakeClient.Status().Update(ctx, instance)).To(Succeed())
			})

			It("should return false", func() {
				shouldRun, _, err := reconciler.evaluateSchedule()
				Expect(err).NotTo(HaveOccurred())
				Expect(shouldRun).To(BeFalse())
			})
		})
	})

	Describe("updateStatusAfterRun", func() {
		It("should update status and remove annotations", func() {
			instance.Annotations = map[string]string{
				"renovate.thegeeklab.de/operation": "renovate",
			}
			Expect(fakeClient.Update(ctx, instance)).To(Succeed())

			result, err := reconciler.updateStatusAfterRun(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))

			updatedInstance := &renovatev1beta1.Runner{}
			Expect(fakeClient.Get(ctx, reconciler.req.NamespacedName, updatedInstance)).To(Succeed())
			Expect(updatedInstance.Annotations).NotTo(HaveKey("renovate.thegeeklab.de/operation"))
			Expect(updatedInstance.Status.LastScheduleTime).NotTo(BeNil())
		})
	})
})
