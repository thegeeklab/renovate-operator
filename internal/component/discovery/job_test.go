package discovery

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/metadata"
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
		instance   *renovatev1beta1.Discovery
		renovate   *renovatev1beta1.RenovateConfig
		ctx        context.Context
		scheme     *runtime.Scheme
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
			},
			Spec: renovatev1beta1.DiscoverySpec{
				JobSpec: renovatev1beta1.JobSpec{
					Schedule: "*/5 * * * *",
				},
				Filter: []string{"*"},
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

		_ = (&DiscoveryCustomDefaulter{}).Default(ctx, instance)
		_ = (&RenovateConfigCustomDefaulter{}).Default(ctx, renovate)

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

		Context("when there are active jobs", func() {
			BeforeEach(func() {
				activeJob := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "active-job",
						Namespace: "default",
						Labels:    DiscoveryMetadata(reconciler.req).Labels,
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
				Expect(job.Name).To(HavePrefix("test-discovery-"))
				Expect(job.Namespace).To(Equal("default"))
				Expect(job.Labels).To(Equal(DiscoveryMetadata(reconciler.req).Labels))
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

			err := reconciler.updateJob(job)
			Expect(err).NotTo(HaveOccurred())

			Expect(job.Spec.Template.Spec.InitContainers).To(HaveLen(1))
			Expect(job.Spec.Template.Spec.InitContainers[0].Name).To(Equal("renovate-init"))

			Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(job.Spec.Template.Spec.Containers[0].Name).To(Equal("renovate-discovery"))

			expectedServiceAccountName := metadata.GenericMetadata(reconciler.req).Name
			Expect(job.Spec.Template.Spec.ServiceAccountName).To(Equal(expectedServiceAccountName))
		})
	})

	Describe("evaluateSchedule", func() {
		Context("when immediate execution annotation is present", func() {
			BeforeEach(func() {
				instance.Annotations = map[string]string{
					"renovate.thegeeklab.de/renovator-operation": "discover",
				}
				Expect(fakeClient.Update(ctx, instance)).To(Succeed())
			})

			It("should return true for immediate execution", func() {
				shouldRun, _, err := reconciler.evaluateSchedule()
				Expect(err).NotTo(HaveOccurred())
				Expect(shouldRun).To(BeTrue())
			})
		})

		Context("when schedule is valid and job should run", func() {
			It("should return true when last run time is zero", func() {
				shouldRun, _, err := reconciler.evaluateSchedule()
				Expect(err).NotTo(HaveOccurred())
				Expect(shouldRun).To(BeTrue())
			})
		})

		Context("when schedule is valid but job should not run yet", func() {
			BeforeEach(func() {
				now := metav1.NewTime(time.Now())
				instance.Status.LastScheduleTime = &now
				Expect(fakeClient.Status().Update(ctx, instance)).To(Succeed())
			})

			It("should return false when current time is before next scheduled time", func() {
				shouldRun, _, err := reconciler.evaluateSchedule()
				Expect(err).NotTo(HaveOccurred())
				Expect(shouldRun).To(BeFalse())
			})
		})
	})
})
