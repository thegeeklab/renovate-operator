package discovery

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/equality"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
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
		instance.Default()

		renovate = &renovatev1beta1.RenovateConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-renovate-config",
				Namespace: "test-namespace",
			},
		}
		renovate.Default()
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
})
