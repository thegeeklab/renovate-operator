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
			renovator := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator",
					Namespace: "default",
				},
				Spec: renovatev1beta1.RenovatorSpec{},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			result, err := reconciler.reconcileRenovateConfig(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())

			renovateConfig := &renovatev1beta1.RenovateConfig{}
			err = fakeClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: "test-renovator"}, renovateConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(renovateConfig.Labels).To(HaveKey(renovatev1beta1.LabelRenovator))
		})

		It("should update existing RenovateConfig resource", func() {
			renovator := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator",
					Namespace: "default",
				},
				Spec: renovatev1beta1.RenovatorSpec{},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			_, err = reconciler.reconcileRenovateConfig(ctx)
			Expect(err).NotTo(HaveOccurred())

			_, err = reconciler.reconcileRenovateConfig(ctx)
			Expect(err).NotTo(HaveOccurred())

			renovateConfig := &renovatev1beta1.RenovateConfig{}
			err = fakeClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: "test-renovator"}, renovateConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(renovateConfig.Labels).To(HaveKey(renovatev1beta1.LabelRenovator))
		})
	})

	Describe("updateRenovateConfig", func() {
		It("should update RenovateConfig spec and labels", func() {
			renovator := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator",
					Namespace: "default",
					UID:       "test-uid",
				},
				Spec: renovatev1beta1.RenovatorSpec{},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			renovateConfig := &renovatev1beta1.RenovateConfig{}

			err = reconciler.updateRenovateConfig(renovateConfig)
			Expect(err).NotTo(HaveOccurred())

			Expect(renovateConfig.Labels).To(HaveKey(renovatev1beta1.LabelRenovator))
			Expect(renovateConfig.Labels[renovatev1beta1.LabelRenovator]).To(Equal("test-uid"))
		})
	})

	Describe("reconcileRenovateConfigMap", func() {
		It("should create Renovate ConfigMap", func() {
			renovator := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator",
					Namespace: "default",
				},
				Spec: renovatev1beta1.RenovatorSpec{},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			result, err := reconciler.reconcileRenovateConfigMap(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())

			configMap := &corev1.ConfigMap{}
			err = fakeClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: "test-renovator-renovate-conf"}, configMap)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should update existing Renovate ConfigMap", func() {
			renovator := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator",
					Namespace: "default",
				},
				Spec: renovatev1beta1.RenovatorSpec{},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			_, err = reconciler.reconcileRenovateConfigMap(ctx)
			Expect(err).NotTo(HaveOccurred())

			_, err = reconciler.reconcileRenovateConfigMap(ctx)
			Expect(err).NotTo(HaveOccurred())

			configMap := &corev1.ConfigMap{}
			err = fakeClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: "test-renovator-renovate-conf"}, configMap)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("updateConfigMap", func() {
		It("should update ConfigMap with renovate config", func() {
			renovator := &renovatev1beta1.Renovator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator",
					Namespace: "default",
				},
				Spec: renovatev1beta1.RenovatorSpec{},
			}

			reconciler, err := NewReconciler(ctx, fakeClient, scheme, renovator)
			Expect(err).NotTo(HaveOccurred())

			configMap := &corev1.ConfigMap{}

			err = reconciler.updateConfigMap(configMap)
			Expect(err).NotTo(HaveOccurred())

			Expect(configMap.Data).NotTo(BeNil())
		})
	})
})
