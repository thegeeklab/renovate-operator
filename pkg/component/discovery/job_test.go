package discovery

import (
	"context"

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

var _ = Describe("HandleCronJob", func() {
	var (
		fakeClient client.Client
		reconciler *Reconciler
		instance   *renovatev1beta1.Renovator
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

		instance = &renovatev1beta1.Renovator{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-renovator",
				Namespace: "test-namespace",
			},
			Spec: renovatev1beta1.RenovatorSpec{
				Discovery: renovatev1beta1.DiscoverySpec{
					Schedule: "* * * * *",
				},
			},
		}

		reconciler = &Reconciler{
			Client:   fakeClient,
			scheme:   scheme,
			req:      ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "test-namespace", Name: "test-renovator"}},
			instance: instance,
		}

		ctx = context.Background()
	})

	Context("when the cron job is created or updated", func() {
		It("should create or update the cron job and return no error", func() {
			// Execute
			result, err := reconciler.handleCronJob(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())

			job := &batchv1.CronJob{ObjectMeta: DiscoveryMetaData(reconciler.req)}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(job), job)).To(Succeed())

			jobExpected := job.DeepCopy()
			Expect(reconciler.updateCronJob(jobExpected)).To(Succeed())
			Expect(equality.Semantic.DeepEqual(jobExpected, job)).To(BeTrue())
		})
	})
})
