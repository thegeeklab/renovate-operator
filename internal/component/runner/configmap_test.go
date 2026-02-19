package runner

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

		instance := &renovatev1beta1.Runner{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-runner",
				Namespace: "test-namespace",
			},
		}
		Expect(fakeClient.Create(ctx, instance)).To(Succeed())

		reconciler = &Reconciler{
			Client: fakeClient,
			req: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-runner",
					Namespace: "test-namespace",
				},
			},
			instance: instance,
			index: []JobData{
				{
					Repositories: []string{"repo1"},
				},
				{
					Repositories: []string{"repo2"},
				},
			},
			indexCount: int32(2),
		}

		ctx = context.Background()
	})

	Context("when reconciling ConfigMap", func() {
		It("should create or update the configmap with index data", func() {
			// Execute
			result, err := reconciler.reconcileConfigMap(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())

			// Verify configmap was created
			cm := &corev1.ConfigMap{ObjectMeta: metadata.GenericMetadata(reconciler.req, ConfigMapSuffix)}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(cm), cm)).To(Succeed())
			Expect(cm.Name).To(Equal(metadata.GenericMetadata(reconciler.req, ConfigMapSuffix).Name))
			Expect(cm.Namespace).To(Equal(reconciler.req.Namespace))

			// Verify configmap contains index data
			Expect(cm.Data).ToNot(BeEmpty())
			Expect(cm.Data).To(HaveKey(renovate.FilenameIndex))
			Expect(cm.Data[renovate.FilenameIndex]).ToNot(BeEmpty())
		})
	})

	Context("when updating ConfigMap", func() {
		It("should correctly serialize index data", func() {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-runner-renovate-index",
					Namespace: "test-namespace",
				},
				Data: make(map[string]string),
			}

			// Execute update
			err := reconciler.updateConfigMap(cm)
			Expect(err).ToNot(HaveOccurred())

			// Verify index data was serialized correctly
			Expect(cm.Data).To(HaveKey(renovate.FilenameIndex))
			Expect(cm.Data[renovate.FilenameIndex]).ToNot(BeEmpty())

			// Verify the data can be deserialized back
			var index []JobData

			err = json.Unmarshal([]byte(cm.Data[renovate.FilenameIndex]), &index)
			Expect(err).ToNot(HaveOccurred())
			Expect(index).To(HaveLen(2))
			Expect(index[0].Repositories).To(Equal([]string{"repo1"}))
			Expect(index[1].Repositories).To(Equal([]string{"repo2"}))
		})

		It("should handle empty index gracefully", func() {
			reconciler.index = []JobData{}
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-runner-renovate-index",
					Namespace: "test-namespace",
				},
				Data: make(map[string]string),
			}

			// Execute update
			err := reconciler.updateConfigMap(cm)
			Expect(err).ToNot(HaveOccurred())

			// Verify configmap data contains empty array when index is empty
			Expect(cm.Data).To(HaveKey(renovate.FilenameIndex))
			Expect(cm.Data[renovate.FilenameIndex]).To(Equal("[]"))
		})
	})
})
