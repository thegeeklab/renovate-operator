package discovery

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/metadata"
	"github.com/thegeeklab/renovate-operator/pkg/reconciler"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("CronJob Reconciliation", func() {
	var (
		r              *discoveryReconciler
		ctx            context.Context
		mockKubeClient client.Client
		mockScheme     *runtime.Scheme
		mockInstance   *renovatev1beta1.Renovator
		mockReq        ctrl.Request
	)

	BeforeEach(func() {
		// Initialize mocks and test setup
		ctx = context.Background()
		mockScheme = runtime.NewScheme()
		Expect(batchv1.AddToScheme(mockScheme)).To(Succeed())
		Expect(corev1.AddToScheme(mockScheme)).To(Succeed())
		Expect(renovatev1beta1.AddToScheme(mockScheme)).To(Succeed())

		mockInstance = &renovatev1beta1.Renovator{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-renovator",
				Namespace: "test-namespace",
			},
			Spec: renovatev1beta1.RenovatorSpec{
				Discovery: renovatev1beta1.DiscoverySpec{
					Schedule: "0 * * * *",
				},
			},
		}
		mockReq = ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-renovator",
				Namespace: "test-namespace",
			},
		}

		mockKubeClient = fake.NewClientBuilder().WithScheme(mockScheme).WithObjects(mockInstance).Build()

		r = &discoveryReconciler{
			GenericReconciler: &reconciler.GenericReconciler{
				KubeClient: mockKubeClient,
				Scheme:     mockScheme,
				Req:        mockReq,
			},
			instance: mockInstance,
		}
	})

	Describe("reconcileCronJob", func() {
		Context("when immediate discovery is requested", func() {
			BeforeEach(func() {
				r.instance.Annotations = map[string]string{
					renovatev1beta1.AnnotationOperation: string(renovatev1beta1.OperationDiscover),
				}
			})

			It("should handle immediate discovery", func() {
				_, err := r.reconcileCronJob(ctx)
				Expect(err).NotTo(HaveOccurred())

				// Verify that the annotation was removed
				Expect(r.instance.Annotations).NotTo(HaveKey(renovatev1beta1.AnnotationOperation))

				// Verify that a job was created
				jobList := &batchv1.JobList{}
				Expect(mockKubeClient.List(ctx, jobList, client.InNamespace(r.instance.Namespace))).To(Succeed())
				Expect(jobList.Items).To(HaveLen(1))
			})
		})

		Context("when scheduled discovery is requested", func() {
			It("should handle scheduled discovery", func() {
				_, err := r.reconcileCronJob(ctx)
				Expect(err).NotTo(HaveOccurred())

				// Verify that a CronJob was created
				cronJobList := &batchv1.CronJobList{}
				Expect(mockKubeClient.List(ctx, cronJobList, client.InNamespace(r.instance.Namespace))).To(Succeed())
				Expect(cronJobList.Items).To(HaveLen(1))
				Expect(cronJobList.Items[0].Spec.Schedule).To(Equal("0 * * * *"))
			})
		})
	})

	Describe("isDiscoverOperationRequested", func() {
		Context("when discovery annotation is present", func() {
			BeforeEach(func() {
				r.instance.Annotations = map[string]string{
					renovatev1beta1.AnnotationOperation: string(renovatev1beta1.OperationDiscover),
				}
			})

			It("should return true", func() {
				Expect(r.isDiscoverOperationRequested()).To(BeTrue())
			})
		})

		Context("when discovery annotation is not present", func() {
			It("should return false", func() {
				Expect(r.isDiscoverOperationRequested()).To(BeFalse())
			})
		})
	})

	Describe("handleImmediateDiscovery", func() {
		Context("when a discovery job is already running", func() {
			BeforeEach(func() {
				// Mock a running job
				existingJob := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      metadata.DiscoveryName(r.Req),
						Namespace: r.instance.Namespace,
					},
					Status: batchv1.JobStatus{
						Active: 1,
					},
				}
				Expect(mockKubeClient.Create(ctx, existingJob)).To(Succeed())
			})

			It("should return a requeue result", func() {
				result, err := r.handleImmediateDiscovery(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.RequeueAfter).To(Equal(time.Minute))
			})
		})

		Context("when no discovery job is running", func() {
			It("should create a new job and remove the annotation", func() {
				_, err := r.handleImmediateDiscovery(ctx)
				Expect(err).NotTo(HaveOccurred())

				// Verify that the annotation was removed
				Expect(r.instance.Annotations).NotTo(HaveKey(renovatev1beta1.AnnotationOperation))

				// Verify that a job was created
				jobList := &batchv1.JobList{}
				Expect(mockKubeClient.List(ctx, jobList, client.InNamespace(r.instance.Namespace))).To(Succeed())
				Expect(jobList.Items).To(HaveLen(1))

				// Verify job properties
				job := jobList.Items[0]
				Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
				Expect(job.Spec.Template.Spec.Containers[0].Name).To(Equal("renovate-discovery"))
				Expect(job.Spec.Template.Spec.Containers[0].Image).To(Equal(r.instance.Spec.Image))
			})
		})
	})

	Describe("handleScheduledDiscovery", func() {
		It("should create a CronJob", func() {
			_, err := r.handleScheduledDiscovery(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Verify that a CronJob was created
			cronJobList := &batchv1.CronJobList{}
			Expect(mockKubeClient.List(ctx, cronJobList, client.InNamespace(r.instance.Namespace))).To(Succeed())
			Expect(cronJobList.Items).To(HaveLen(1))

			// Verify CronJob properties
			cronJob := cronJobList.Items[0]
			Expect(cronJob.Spec.Schedule).To(Equal("0 * * * *"))
			Expect(cronJob.Spec.ConcurrencyPolicy).To(Equal(batchv1.ForbidConcurrent))
			Expect(cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Name).To(Equal("renovate-discovery"))
		})
	})

	Describe("hasRunningDiscoveryJob", func() {
		Context("when a discovery job is running", func() {
			BeforeEach(func() {
				// Mock a running job
				existingJob := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      metadata.DiscoveryName(r.Req),
						Namespace: r.instance.Namespace,
					},
					Status: batchv1.JobStatus{
						Active: 1,
					},
				}
				Expect(mockKubeClient.Create(ctx, existingJob)).To(Succeed())
			})

			It("should return true", func() {
				hasRunning, err := r.hasRunningDiscoveryJob(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(hasRunning).To(BeTrue())
			})
		})

		Context("when no discovery job is running", func() {
			It("should return false", func() {
				hasRunning, err := r.hasRunningDiscoveryJob(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(hasRunning).To(BeFalse())
			})
		})
	})

	Describe("createDiscoveryJob", func() {
		It("should create a valid Job", func() {
			job, err := r.createDiscoveryJob()
			Expect(err).NotTo(HaveOccurred())
			Expect(job).NotTo(BeNil())

			// Verify job properties
			Expect(job.ObjectMeta.Name).To(Equal(metadata.DiscoveryName(r.Req)))
			Expect(job.ObjectMeta.Namespace).To(Equal(r.instance.Namespace))
			Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(job.Spec.Template.Spec.Containers[0].Name).To(Equal("renovate-discovery"))
		})
	})

	Describe("removeDiscoveryAnnotation", func() {
		BeforeEach(func() {
			r.instance.Annotations = map[string]string{
				renovatev1beta1.AnnotationOperation: string(renovatev1beta1.OperationDiscover),
			}
		})

		It("should remove the discovery annotation", func() {
			err := r.removeDiscoveryAnnotation(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(r.instance.Annotations).NotTo(HaveKey(renovatev1beta1.AnnotationOperation))
		})
	})

	Describe("createCronJob", func() {
		It("should create a valid CronJob", func() {
			cronJob, err := r.createCronJob()
			Expect(err).NotTo(HaveOccurred())
			Expect(cronJob).NotTo(BeNil())

			// Verify CronJob properties
			Expect(cronJob.ObjectMeta.Name).To(Equal(metadata.DiscoveryName(r.Req)))
			Expect(cronJob.ObjectMeta.Namespace).To(Equal(r.instance.Namespace))
			Expect(cronJob.Spec.Schedule).To(Equal("0 * * * *"))
			Expect(cronJob.Spec.ConcurrencyPolicy).To(Equal(batchv1.ForbidConcurrent))
			Expect(cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Name).To(Equal("renovate-discovery"))
		})
	})

	Describe("createJobSpec", func() {
		It("should create a valid JobSpec", func() {
			spec := r.createJobSpec()
			Expect(spec).NotTo(BeNil())

			// Verify JobSpec properties
			Expect(spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(spec.Template.Spec.Containers[0].Name).To(Equal("renovate-discovery"))
			Expect(spec.Template.Spec.ServiceAccountName).To(Equal(metadata.GenericMetaData(r.Req).Name))
			Expect(spec.Template.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyNever))
		})
	})
})
