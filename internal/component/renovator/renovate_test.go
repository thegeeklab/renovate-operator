package renovator

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Renovator Renovate Functions", func() {
	var (
		ctx        context.Context
		scheme     *runtime.Scheme
		fakeClient client.Client
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(renovatev1beta1.AddToScheme(scheme)).To(Succeed())
		Expect(corev1.SchemeBuilder.AddToScheme(scheme)).To(Succeed())
		fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()
	})

	Describe("reconcileRenovateConfig", func() {
		It("should create RenovateConfig resource", func() {
			// Create a Renovator instance
			renovator := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator",
					Namespace: "default",
				},
				Spec: renovatev1beta1.RenovatorSpec{},
			}

			// Create a Reconciler
			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			// Call reconcileRenovateConfig
			result, err := reconciler.reconcileRenovateConfig(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())

			// Verify RenovateConfig was created
			renovateConfig := &renovatev1beta1.RenovateConfig{}
			err = fakeClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: "test-renovator"}, renovateConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(renovateConfig.Labels).To(HaveKey(renovatev1beta1.RenovatorLabel))
		})

		It("should update existing RenovateConfig resource", func() {
			// Create a Renovator instance
			renovator := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator",
					Namespace: "default",
				},
				Spec: renovatev1beta1.RenovatorSpec{},
			}

			// Create a Reconciler
			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			// Call reconcileRenovateConfig twice
			_, err = reconciler.reconcileRenovateConfig(ctx)
			Expect(err).NotTo(HaveOccurred())

			_, err = reconciler.reconcileRenovateConfig(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Verify RenovateConfig was updated
			renovateConfig := &renovatev1beta1.RenovateConfig{}
			err = fakeClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: "test-renovator"}, renovateConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(renovateConfig.Labels).To(HaveKey(renovatev1beta1.RenovatorLabel))
		})
	})

	Describe("updateRenovateConfig", func() {
		It("should update RenovateConfig spec and labels", func() {
			// Create a Renovator instance
			renovator := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator",
					Namespace: "default",
					UID:       "test-uid",
				},
				Spec: renovatev1beta1.RenovatorSpec{},
			}

			// Create a Reconciler
			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			// Create a RenovateConfig instance
			renovateConfig := &renovatev1beta1.RenovateConfig{}

			// Call updateRenovateConfig
			err = reconciler.updateRenovateConfig(renovateConfig)
			Expect(err).NotTo(HaveOccurred())

			// Verify labels and spec were updated
			Expect(renovateConfig.Labels).To(HaveKey(renovatev1beta1.RenovatorLabel))
			Expect(renovateConfig.Labels[renovatev1beta1.RenovatorLabel]).To(Equal("test-uid"))
		})
	})

	Describe("reconcileRenovateConfigMap", func() {
		It("should create Renovate ConfigMap", func() {
			// Create a Renovator instance
			renovator := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator",
					Namespace: "default",
				},
				Spec: renovatev1beta1.RenovatorSpec{},
			}

			// Create a Reconciler
			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			// Call reconcileRenovateConfigMap
			result, err := reconciler.reconcileRenovateConfigMap(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())

			// Verify ConfigMap was created
			configMap := &corev1.ConfigMap{}
			err = fakeClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: "test-renovator-renovate-conf"}, configMap)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should update existing Renovate ConfigMap", func() {
			// Create a Renovator instance
			renovator := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator",
					Namespace: "default",
				},
				Spec: renovatev1beta1.RenovatorSpec{},
			}

			// Create a Reconciler
			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			// Call reconcileRenovateConfigMap twice
			_, err = reconciler.reconcileRenovateConfigMap(ctx)
			Expect(err).NotTo(HaveOccurred())

			_, err = reconciler.reconcileRenovateConfigMap(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Verify ConfigMap was updated
			configMap := &corev1.ConfigMap{}
			err = fakeClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: "test-renovator-renovate-conf"}, configMap)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("updateConfigMap", func() {
		It("should update ConfigMap with renovate config", func() {
			// Create a Renovator instance
			renovator := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator",
					Namespace: "default",
				},
				Spec: renovatev1beta1.RenovatorSpec{},
			}

			// Create a Reconciler
			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			// Create a ConfigMap instance
			configMap := &corev1.ConfigMap{}

			// Call updateConfigMap
			err = reconciler.updateConfigMap(configMap)
			Expect(err).NotTo(HaveOccurred())

			// Verify ConfigMap data was updated
			Expect(configMap.Data).NotTo(BeNil())
		})
	})
})
