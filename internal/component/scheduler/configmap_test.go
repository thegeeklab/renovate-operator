package scheduler

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/metadata"
	"github.com/thegeeklab/renovate-operator/internal/resource/renovate"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("ConfigMap Reconciliation", func() {
	var (
		fakeClient client.Client
		reconciler *Reconciler
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

		instance := &renovatev1beta1.Scheduler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-scheduler",
				Namespace: "test-namespace",
			},
		}
		Expect(fakeClient.Create(ctx, instance)).To(Succeed())

		reconciler = &Reconciler{
			Client: fakeClient,
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-scheduler",
					Namespace: "test-namespace",
				},
			},
			instance: instance,
			batches: []Batch{
				{
					Repositories: []string{"repo1"},
				},
				{
					Repositories: []string{"repo2"},
				},
			},
			batchesCount: int32(2),
		}

		ctx = context.Background()
	})

	Context("when reconciling ConfigMap", func() {
		It("should create or update the configmap with batches data", func() {
			// Execute
			result, err := reconciler.reconcileConfigMap(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())

			// Verify configmap was created
			cm := &corev1.ConfigMap{ObjectMeta: metadata.GenericMetadata(reconciler.req, ConfigMapSuffix)}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(cm), cm)).To(Succeed())
			Expect(cm.Name).To(Equal(metadata.GenericMetadata(reconciler.req, ConfigMapSuffix).Name))
			Expect(cm.Namespace).To(Equal(reconciler.req.Namespace))

			// Verify configmap contains batches data
			Expect(cm.Data).ToNot(BeEmpty())
			Expect(cm.Data).To(HaveKey(renovate.FilenameBatches))
			Expect(cm.Data[renovate.FilenameBatches]).ToNot(BeEmpty())
		})
	})

	Context("when updating ConfigMap", func() {
		It("should correctly serialize batches data", func() {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-scheduler-renovate-batch",
					Namespace: "test-namespace",
				},
				Data: make(map[string]string),
			}

			// Execute update
			err := reconciler.updateConfigMap(cm)
			Expect(err).ToNot(HaveOccurred())

			// Verify batches data was serialized correctly
			Expect(cm.Data).To(HaveKey(renovate.FilenameBatches))
			Expect(cm.Data[renovate.FilenameBatches]).ToNot(BeEmpty())

			// Verify the data can be deserialized back
			var batchesData []Batch
			err = json.Unmarshal([]byte(cm.Data[renovate.FilenameBatches]), &batchesData)
			Expect(err).ToNot(HaveOccurred())
			Expect(batchesData).To(HaveLen(2))
			Expect(batchesData[0].Repositories).To(Equal([]string{"repo1"}))
			Expect(batchesData[1].Repositories).To(Equal([]string{"repo2"}))
		})

		It("should handle empty batches gracefully", func() {
			reconciler.batches = []Batch{}
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-scheduler-renovate-batch",
					Namespace: "test-namespace",
				},
				Data: make(map[string]string),
			}

			// Execute update
			err := reconciler.updateConfigMap(cm)
			Expect(err).ToNot(HaveOccurred())

			// Verify configmap data contains empty array when batches are empty
			Expect(cm.Data).To(HaveKey(renovate.FilenameBatches))
			Expect(cm.Data[renovate.FilenameBatches]).To(Equal("[]"))
		})
	})
})
