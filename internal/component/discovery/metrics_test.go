package discovery

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/metrics"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("ReconcileMetrics", func() {
	var (
		ctx        context.Context
		reconciler *Reconciler
		instance   *renovatev1beta1.Discovery
		reg        *prometheus.Registry
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme := runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
		Expect(renovatev1beta1.AddToScheme(scheme)).To(Succeed())

		instance = &renovatev1beta1.Discovery{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-discovery",
				Namespace: "default",
				Labels: map[string]string{
					renovatev1beta1.LabelRenovator: "renovator-id",
				},
			},
		}

		reg = prometheus.NewRegistry()

		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(instance).Build()

		reconciler = &Reconciler{
			Client:   fakeClient,
			instance: instance,
			metrics:  metrics.New(reg, reg, 100),
		}
	})

	metricCount := func(name string) int {
		families, err := reg.Gather()
		Expect(err).NotTo(HaveOccurred())

		for _, mf := range families {
			if mf.GetName() == name {
				return len(mf.GetMetric())
			}
		}

		return 0
	}

	It("adds the metrics finalizer on reconcile", func() {
		_, err := reconciler.reconcileMetrics(ctx)
		Expect(err).NotTo(HaveOccurred())

		Expect(controllerutil.ContainsFinalizer(instance, renovatev1beta1.FinalizerMetricsCleanup)).To(BeTrue())
	})

	It("is idempotent for finalizer addition", func() {
		_, err := reconciler.reconcileMetrics(ctx)
		Expect(err).NotTo(HaveOccurred())

		_, err = reconciler.reconcileMetrics(ctx)
		Expect(err).NotTo(HaveOccurred())

		Expect(controllerutil.ContainsFinalizer(instance, renovatev1beta1.FinalizerMetricsCleanup)).To(BeTrue())
	})

	It("releases discovery metrics on deletion", func() {
		_, err := reconciler.reconcileMetrics(ctx)
		Expect(err).NotTo(HaveOccurred())

		reconciler.metrics.SetDiscoveryRepositories("default", "renovator-id", "test-discovery", 25)
		Expect(metricCount("renovate_operator_discovery_repositories")).To(Equal(1))

		instance.DeletionTimestamp = &metav1.Time{Time: time.Now()}
		_, err = reconciler.reconcileMetrics(ctx)
		Expect(err).NotTo(HaveOccurred())

		Expect(controllerutil.ContainsFinalizer(instance, renovatev1beta1.FinalizerMetricsCleanup)).To(BeFalse())
		Expect(metricCount("renovate_operator_discovery_repositories")).To(Equal(0))
	})

	It("does nothing on deletion when finalizer is already removed", func() {
		_, err := reconciler.reconcileMetrics(ctx)
		Expect(err).NotTo(HaveOccurred())

		instance.DeletionTimestamp = &metav1.Time{Time: time.Now()}
		_, err = reconciler.reconcileMetrics(ctx)
		Expect(err).NotTo(HaveOccurred())

		Expect(controllerutil.ContainsFinalizer(instance, renovatev1beta1.FinalizerMetricsCleanup)).To(BeFalse())

		_, err = reconciler.reconcileMetrics(ctx)
		Expect(err).NotTo(HaveOccurred())
	})
})
