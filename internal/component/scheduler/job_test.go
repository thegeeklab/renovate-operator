package scheduler

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	cronjob "github.com/thegeeklab/renovate-operator/internal/resource/cronjob"
	"github.com/thegeeklab/renovate-operator/internal/webhook/v1beta1"
	. "github.com/thegeeklab/renovate-operator/internal/webhook/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Job Reconciliation", func() {
	var (
		fakeClient client.Client
		reconciler *Reconciler
		instance   *renovatev1beta1.Scheduler
		ctx        context.Context
		scheme     *runtime.Scheme
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
		Expect(renovatev1beta1.AddToScheme(scheme)).To(Succeed())

		fakeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			Build()

		instance = &renovatev1beta1.Scheduler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-scheduler",
				Namespace: "test-namespace",
			},
			Spec: renovatev1beta1.SchedulerSpec{
				JobSpec: renovatev1beta1.JobSpec{
					Schedule: "* * * * *",
				},
			},
		}
		sd := &SchedulerCustomDefaulter{}
		Expect(sd.Default(ctx, instance)).To(Succeed())
		Expect(fakeClient.Create(ctx, instance)).To(Succeed())

		// Create a RenovateConfig instance
		renovate := &renovatev1beta1.RenovateConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-renovate-config",
				Namespace: "test-namespace",
			},
		}
		rd := &v1beta1.RenovateConfigCustomDefaulter{}
		Expect(rd.Default(ctx, renovate)).To(Succeed())
		Expect(fakeClient.Create(ctx, renovate)).To(Succeed())

		// Create scheduler custom defaulter
		schedulerDefaulter := &v1beta1.SchedulerCustomDefaulter{}
		Expect(schedulerDefaulter.Default(ctx, instance)).To(Succeed())

		reconciler = &Reconciler{
			Client:   fakeClient,
			scheme:   scheme,
			req:      ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "test-namespace", Name: "test-scheduler"}},
			instance: instance,
			renovate: renovate,
		}

		reconciler = &Reconciler{
			Client:   fakeClient,
			scheme:   scheme,
			req:      ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "test-namespace", Name: "test-scheduler"}},
			instance: instance,
			renovate: renovate,
		}

		ctx = context.Background()
	})

	Context("when reconciling CronJob", func() {
		It("should create or update the cron job and return no error", func() {
			// Execute
			result, err := reconciler.reconcileCronJob(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())

			// Verify cron job was created
			job := &batchv1.CronJob{ObjectMeta: SchedulerMetadata(reconciler.req)}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(job), job)).To(Succeed())
			Expect(job.Name).To(Equal(SchedulerMetadata(reconciler.req).Name))
			Expect(job.Namespace).To(Equal(reconciler.req.Namespace))
		})
	})

	Context("when immediate renovate annotation is set", func() {
		It("should trigger handleImmediateRenovate", func() {
			// Set the annotation to trigger immediate renovate
			instance.Annotations = map[string]string{
				renovatev1beta1.RenovatorOperation: string(renovatev1beta1.OperationRenovate),
			}
			Expect(fakeClient.Update(ctx, instance)).To(Succeed())

			// Execute
			result, err := reconciler.reconcileCronJob(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())

			// Verify that the annotation was removed after handling
			updatedInstance := &renovatev1beta1.Scheduler{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(instance), updatedInstance)).To(Succeed())
			Expect(updatedInstance.Annotations).ToNot(HaveKey(renovatev1beta1.RenovatorOperation))

			// Verify that a renovate job was created
			jobList := &batchv1.JobList{}
			Expect(fakeClient.List(ctx, jobList, client.InNamespace(instance.Namespace))).To(Succeed())
			Expect(jobList.Items).ToNot(BeEmpty())

			// Additional verification: check that the job has the correct name pattern
			schedulerName := SchedulerName(reconciler.req)
			foundMatchingJob := false
			for _, job := range jobList.Items {
				if job.Name == schedulerName || strings.HasPrefix(job.Name, schedulerName+"-") {
					foundMatchingJob = true

					break
				}
			}
			Expect(foundMatchingJob).To(BeTrue(), "No job found with expected name pattern")
		})
	})

	Context("when active renovate jobs exist", func() {
		It("should requeue when active renovate jobs are found", func() {
			// Create an active renovate job
			activeJob := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      SchedulerName(reconciler.req) + "-active",
					Namespace: instance.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/instance": instance.Name,
						"app.kubernetes.io/name":     "scheduler",
					},
				},
				Spec: batchv1.JobSpec{},
				Status: batchv1.JobStatus{
					Active: 1,
				},
			}
			Expect(fakeClient.Create(ctx, activeJob)).To(Succeed())

			// Set the annotation to trigger immediate renovate
			instance.Annotations = map[string]string{
				renovatev1beta1.RenovatorOperation: string(renovatev1beta1.OperationRenovate),
			}
			Expect(fakeClient.Update(ctx, instance)).To(Succeed())

			// Execute
			result, err := reconciler.reconcileCronJob(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.RequeueAfter).To(Equal(cronjob.RequeueDelay))
		})
	})
})
