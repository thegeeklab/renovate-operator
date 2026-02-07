package discovery

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/metadata"
	cronjob "github.com/thegeeklab/renovate-operator/internal/resource/cronjob"
	v1beta1 "github.com/thegeeklab/renovate-operator/internal/webhook/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("ReconcileCronJob", func() {
	var (
		fakeClient client.Client
		reconciler *Reconciler
		instance   *renovatev1beta1.Discovery
		renovate   *renovatev1beta1.RenovateConfig
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

		instance = &renovatev1beta1.Discovery{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-renovator",
				Namespace: "test-namespace",
			},
			Spec: renovatev1beta1.DiscoverySpec{
				JobSpec: renovatev1beta1.JobSpec{
					Schedule: "* * * * *",
				},
				Filter: []string{"*"},
			},
		}
		dd := &v1beta1.DiscoveryCustomDefaulter{}
		Expect(dd.Default(ctx, instance)).To(Succeed())

		renovate = &renovatev1beta1.RenovateConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-renovate-config",
				Namespace: "test-namespace",
			},
		}
		rd := &v1beta1.RenovateConfigCustomDefaulter{}
		Expect(rd.Default(ctx, renovate)).To(Succeed())
		Expect(fakeClient.Create(ctx, renovate)).To(Succeed())

		reconciler = &Reconciler{
			Client:   fakeClient,
			scheme:   scheme,
			req:      ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "test-namespace", Name: "test-renovator"}},
			instance: instance,
			renovate: renovate,
		}

		ctx = context.Background()
	})

	Context("when the cron job is created or updated", func() {
		It("should create or update the cron job and return no error", func() {
			// Execute
			result, err := reconciler.reconcileCronJob(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())

			job := &batchv1.CronJob{ObjectMeta: DiscoveryMetadata(reconciler.req)}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(job), job)).To(Succeed())

			jobExpected := job.DeepCopy()
			jobExpected.ObjectMeta = DiscoveryMetadata(reconciler.req)
			Expect(reconciler.updateCronJob(jobExpected)).To(Succeed())
			Expect(equality.Semantic.DeepEqual(jobExpected.Spec, job.Spec)).To(BeTrue())
		})
	})

	Context("when immediate discovery annotation is set", func() {
		It("should trigger handleImmediateDiscovery", func() {
			// Set the annotation to trigger immediate discovery
			instance.Annotations = map[string]string{
				renovatev1beta1.RenovatorOperation: string(renovatev1beta1.OperationDiscover),
			}
			Expect(fakeClient.Create(ctx, instance)).To(Succeed())

			// Execute
			result, err := reconciler.reconcileCronJob(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())

			// Verify that the annotation was removed after handling
			updatedInstance := &renovatev1beta1.Discovery{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(instance), updatedInstance)).To(Succeed())
			Expect(updatedInstance.Annotations).ToNot(HaveKey(renovatev1beta1.RenovatorOperation))

			// Verify that a discovery job was created
			jobList := &batchv1.JobList{}
			Expect(fakeClient.List(ctx, jobList, client.InNamespace(instance.Namespace))).To(Succeed())
			Expect(jobList.Items).ToNot(BeEmpty())

			// Additional verification: check that the job has the correct name pattern
			discoveryName := DiscoveryName(reconciler.req)
			foundMatchingJob := false
			for _, job := range jobList.Items {
				if job.Name == discoveryName || strings.HasPrefix(job.Name, discoveryName+"-") {
					foundMatchingJob = true

					break
				}
			}
			Expect(foundMatchingJob).To(BeTrue(), "No job found with expected name pattern")
		})
	})

	Context("when active discovery jobs exist", func() {
		It("should requeue when active discovery jobs are found", func() {
			// Create an active discovery job
			activeJob := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DiscoveryName(reconciler.req) + "-active",
					Namespace: instance.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/instance": instance.Name,
						"app.kubernetes.io/name":     "discovery",
					},
				},
				Spec: batchv1.JobSpec{},
				Status: batchv1.JobStatus{
					Active: 1,
				},
			}
			Expect(fakeClient.Create(ctx, activeJob)).To(Succeed())

			// Set the annotation to trigger immediate discovery
			instance.Annotations = map[string]string{
				renovatev1beta1.RenovatorOperation: string(renovatev1beta1.OperationDiscover),
			}
			Expect(fakeClient.Create(ctx, instance)).To(Succeed())

			// Execute
			result, err := reconciler.reconcileCronJob(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.RequeueAfter).To(Equal(cronjob.RequeueDelay))
		})
	})

	Context("when updateCronJob is called", func() {
		It("should update the cron job spec correctly", func() {
			job := &batchv1.CronJob{
				ObjectMeta: DiscoveryMetadata(reconciler.req),
				Spec:       batchv1.CronJobSpec{},
			}

			// Execute
			err := reconciler.updateCronJob(job)
			Expect(err).ToNot(HaveOccurred())

			// Verify the cron job spec
			Expect(job.Spec.Schedule).To(Equal(instance.Spec.Schedule))
			Expect(job.Spec.ConcurrencyPolicy).To(Equal(batchv1.ForbidConcurrent))
			Expect(job.Spec.Suspend).To(Equal(instance.Spec.Suspend))

			// Verify the job template spec
			jobSpec := &job.Spec.JobTemplate.Spec
			Expect(jobSpec.Template.Spec.ServiceAccountName).To(Equal(metadata.GenericMetadata(reconciler.req).Name))
			Expect(jobSpec.Template.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyNever))

			// Verify volumes
			Expect(jobSpec.Template.Spec.Volumes).To(HaveLen(2))
			Expect(jobSpec.Template.Spec.Volumes[0].Name).To(Equal("renovate-tmp"))
			Expect(jobSpec.Template.Spec.Volumes[1].Name).To(Equal("renovate-config"))

			// Verify init containers
			Expect(jobSpec.Template.Spec.InitContainers).To(HaveLen(1))
			initContainer := jobSpec.Template.Spec.InitContainers[0]
			Expect(initContainer.Name).To(Equal("renovate-init"))
			Expect(initContainer.Image).To(Equal(renovate.Spec.Image))
			Expect(initContainer.Env).To(ContainElement(
				corev1.EnvVar{Name: "RENOVATE_AUTODISCOVER", Value: "true"},
			))
			Expect(initContainer.Env).To(ContainElement(
				corev1.EnvVar{Name: "RENOVATE_AUTODISCOVER_FILTER", Value: strings.Join(instance.Spec.Filter, ",")},
			))

			// Verify main container
			Expect(jobSpec.Template.Spec.Containers).To(HaveLen(1))
			mainContainer := jobSpec.Template.Spec.Containers[0]
			Expect(mainContainer.Name).To(Equal("renovate-discovery"))
			Expect(mainContainer.Command).To(Equal([]string{"/discovery"}))
			Expect(mainContainer.Env).To(ContainElement(
				corev1.EnvVar{Name: "DISCOVERY_INSTANCE_NAME", Value: instance.Name},
			))
			Expect(mainContainer.Env).To(ContainElement(
				corev1.EnvVar{Name: "DISCOVERY_INSTANCE_NAMESPACE", Value: instance.Namespace},
			))
		})
	})
})
