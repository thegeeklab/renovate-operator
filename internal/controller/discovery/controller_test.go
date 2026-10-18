package discovery

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/thegeeklab/renovate-operator/internal/webhook/v1beta1"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Discovery Controller", func() {
	var (
		ctx                context.Context
		reconciler         *Reconciler
		typeNamespacedName types.NamespacedName
	)

	BeforeEach(func() {
		ctx = context.Background()
		reconciler = &Reconciler{
			Client:        k8sClient,
			Scheme:        k8sClient.Scheme(),
			EventRecorder: &events.FakeRecorder{},
		}
	})

	Context("When reconciling a resource via ConfigRef", func() {
		const resourceName = "test-discovery-ref"

		BeforeEach(func() {
			typeNamespacedName = types.NamespacedName{
				Name:      resourceName,
				Namespace: "default",
			}

			config := &renovatev1beta1.RenovateConfig{
				Name:      "test-config-ref",
				Namespace: "default",
				Spec: renovatev1beta1.RenovateConfigSpec{
					Platform: renovatev1beta1.PlatformSpec{
						Type: "github",
						Token: corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								Name: "token",
								Key:  "key",
							},
						},
						Endpoint: "https://api.github.com/",
					},
				},
			}
			rcd := &RenovateConfigCustomDefaulter{}
			Expect(rcd.Default(ctx, config)).To(Succeed())
			Expect(k8sClient.Create(ctx, config)).To(Succeed())

			resource := &renovatev1beta1.Discovery{
				Name:      resourceName,
				Namespace: "default",
				Spec: renovatev1beta1.DiscoverySpec{
					ConfigRef: "test-config-ref",
					JobSpec: renovatev1beta1.JobSpec{
						Schedule: renovatev1beta1.DefaultSchedule,
					},
					Filter: []string{"org/repo1", "org/repo2"},
				},
			}
			dd := &DiscoveryCustomDefaulter{}
			Expect(dd.Default(ctx, resource)).To(Succeed())
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			resource := &renovatev1beta1.Discovery{
				Name:      resourceName,
				Namespace: "default",
			}
			_ = k8sClient.Delete(ctx, resource)

			if err := k8sClient.Get(ctx, client.ObjectKey{Name: resourceName, Namespace: "default"}, resource); err == nil {
				if controllerutil.ContainsFinalizer(resource, renovatev1beta1.FinalizerMetricsCleanup) {
					controllerutil.RemoveFinalizer(resource, renovatev1beta1.FinalizerMetricsCleanup)
					_ = k8sClient.Update(ctx, resource)
				}
			}

			config := &renovatev1beta1.RenovateConfig{
				Name:      "test-config-ref",
				Namespace: "default",
			}
			_ = k8sClient.Delete(ctx, config)
		})

		It("should successfully reconcile the resource", func() {
			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))
		})
	})

	Context("When reconciling via Labels", func() {
		const (
			discoveryName = "test-discovery-label"
			configName    = "test-config-label"
			labelValue    = "renovator-01"
		)

		BeforeEach(func() {
			config := &renovatev1beta1.RenovateConfig{
				Name:      configName,
				Namespace: "default",
				Labels: map[string]string{
					renovatev1beta1.LabelRenovator: labelValue,
				},
				Spec: renovatev1beta1.RenovateConfigSpec{
					Platform: renovatev1beta1.PlatformSpec{
						Type: "github",
						Token: corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								Name: "token",
								Key:  "key",
							},
						},
					},
				},
			}
			rcd := &RenovateConfigCustomDefaulter{}
			Expect(rcd.Default(ctx, config)).To(Succeed())
			Expect(k8sClient.Create(ctx, config)).To(Succeed())

			resource := &renovatev1beta1.Discovery{
				Name:      discoveryName,
				Namespace: "default",
				Labels: map[string]string{
					renovatev1beta1.LabelRenovator: labelValue,
				},
				Spec: renovatev1beta1.DiscoverySpec{
					JobSpec: renovatev1beta1.JobSpec{
						Schedule: renovatev1beta1.DefaultSchedule,
					},
					Filter: []string{"org/repo1"},
				},
			}
			dd := &DiscoveryCustomDefaulter{}
			Expect(dd.Default(ctx, resource)).To(Succeed())
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			resource := &renovatev1beta1.Discovery{
				Name:      discoveryName,
				Namespace: "default",
			}
			_ = k8sClient.Delete(ctx, resource)

			if err := k8sClient.Get(ctx, client.ObjectKey{Name: discoveryName, Namespace: "default"}, resource); err == nil {
				if controllerutil.ContainsFinalizer(resource, renovatev1beta1.FinalizerMetricsCleanup) {
					controllerutil.RemoveFinalizer(resource, renovatev1beta1.FinalizerMetricsCleanup)
					_ = k8sClient.Update(ctx, resource)
				}
			}

			config := &renovatev1beta1.RenovateConfig{
				Name:      configName,
				Namespace: "default",
			}
			_ = k8sClient.Delete(ctx, config)
		})

		It("should resolve RenovateConfig via labels", func() {
			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				Name: discoveryName, Namespace: "default",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))
		})
	})

	Context("When materializing discovered repositories", func() {
		const (
			discoveryName = "test-discovery-repos"
			configName    = "test-config-repos"
			resultName    = "discovery-result"
		)

		discoveryKey := types.NamespacedName{Name: discoveryName, Namespace: "default"}

		BeforeEach(func() {
			config := &renovatev1beta1.RenovateConfig{
				Name:      configName,
				Namespace: "default",
				Spec: renovatev1beta1.RenovateConfigSpec{
					Platform: renovatev1beta1.PlatformSpec{
						Type: "github",
						Token: corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								Name: "token",
								Key:  "key",
							},
						},
						Endpoint: "https://api.github.com/",
					},
				},
			}
			rcd := &RenovateConfigCustomDefaulter{}
			Expect(rcd.Default(ctx, config)).To(Succeed())
			Expect(k8sClient.Create(ctx, config)).To(Succeed())

			resource := &renovatev1beta1.Discovery{
				Name:      discoveryName,
				Namespace: "default",
				Spec: renovatev1beta1.DiscoverySpec{
					ConfigRef: configName,
					JobSpec: renovatev1beta1.JobSpec{
						Schedule: renovatev1beta1.DefaultSchedule,
					},
				},
			}
			dd := &DiscoveryCustomDefaulter{}
			Expect(dd.Default(ctx, resource)).To(Succeed())
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			created := &renovatev1beta1.Discovery{}
			Expect(k8sClient.Get(ctx, discoveryKey, created)).To(Succeed())

			result := &corev1.ConfigMap{
				Name:      resultName,
				Namespace: "default",
				Data: map[string]string{
					"repositories": `["testorg/repo-a", "testorg/repo-b"]`,
				},
			}
			Expect(controllerutil.SetControllerReference(created, result, k8sClient.Scheme())).To(Succeed())
			Expect(k8sClient.Create(ctx, result)).To(Succeed())
		})

		AfterEach(func() {
			resource := &renovatev1beta1.Discovery{
				Name:      discoveryName,
				Namespace: "default",
			}
			_ = k8sClient.Delete(ctx, resource)

			if err := k8sClient.Get(ctx, discoveryKey, resource); err == nil {
				if controllerutil.ContainsFinalizer(resource, renovatev1beta1.FinalizerMetricsCleanup) {
					controllerutil.RemoveFinalizer(resource, renovatev1beta1.FinalizerMetricsCleanup)
					_ = k8sClient.Update(ctx, resource)
				}
			}

			config := &renovatev1beta1.RenovateConfig{
				Name:      configName,
				Namespace: "default",
			}
			_ = k8sClient.Delete(ctx, config)

			result := &corev1.ConfigMap{
				Name:      resultName,
				Namespace: "default",
			}
			_ = k8sClient.Delete(ctx, result)

			gitRepos := &renovatev1beta1.GitRepoList{}
			if err := k8sClient.List(ctx, gitRepos, client.InNamespace("default")); err == nil {
				for i := range gitRepos.Items {
					_ = k8sClient.Delete(ctx, &gitRepos.Items[i])
				}
			}
		})

		It("should create GitRepo resources for discovered repositories", func() {
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: discoveryKey})
			Expect(err).NotTo(HaveOccurred())

			gitRepos := &renovatev1beta1.GitRepoList{}
			Expect(k8sClient.List(ctx, gitRepos, client.InNamespace("default"))).To(Succeed())

			discovered := make(map[string]bool, len(gitRepos.Items))
			for i := range gitRepos.Items {
				discovered[gitRepos.Items[i].Spec.Name] = true
			}

			Expect(discovered).To(HaveKey("testorg/repo-a"))
			Expect(discovered).To(HaveKey("testorg/repo-b"))
		})
	})

	It("should handle missing RenovateConfig resource gracefully", func() {
		mockClient := &mockErrorClient{Client: k8sClient}
		errorReconciler := &Reconciler{Client: mockClient, Scheme: k8sClient.Scheme(), EventRecorder: &events.FakeRecorder{}}

		result, err := errorReconciler.Reconcile(ctx, reconcile.Request{
			Name: "missing-config-discovery", Namespace: "default",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(reconcile.Result{}))
	})

	It("should handle missing Discovery resource gracefully", func() {
		nonExistentName := types.NamespacedName{
			Name:      "non-existent-discovery",
			Namespace: "default",
		}

		result, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: nonExistentName,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(reconcile.Result{}))
	})

	It("removes the metrics finalizer on deletion without a RenovateConfig", func() {
		const name = "test-discovery-deleting"

		key := types.NamespacedName{Name: name, Namespace: "default"}

		resource := &renovatev1beta1.Discovery{
			Name:      name,
			Namespace: "default",
			Spec: renovatev1beta1.DiscoverySpec{
				JobSpec: renovatev1beta1.JobSpec{Schedule: renovatev1beta1.DefaultSchedule},
			},
		}
		controllerutil.AddFinalizer(resource, renovatev1beta1.FinalizerMetricsCleanup)
		Expect(k8sClient.Create(ctx, resource)).To(Succeed())

		DeferCleanup(func() {
			got := &renovatev1beta1.Discovery{}
			if err := k8sClient.Get(ctx, key, got); err == nil {
				if controllerutil.ContainsFinalizer(got, renovatev1beta1.FinalizerMetricsCleanup) {
					controllerutil.RemoveFinalizer(got, renovatev1beta1.FinalizerMetricsCleanup)
					_ = k8sClient.Update(ctx, got)
				}

				_ = k8sClient.Delete(ctx, got)
			}
		})

		Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

		result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(reconcile.Result{}))

		got := &renovatev1beta1.Discovery{}
		if getErr := k8sClient.Get(ctx, key, got); getErr == nil {
			Expect(controllerutil.ContainsFinalizer(got, renovatev1beta1.FinalizerMetricsCleanup)).To(BeFalse())
		} else {
			Expect(api_errors.IsNotFound(getErr)).To(BeTrue())
		}
	})
})

// mockErrorClient is a mock client that returns errors for testing.
type mockErrorClient struct {
	client.Client
}

func (m *mockErrorClient) Get(
	ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption,
) error {
	if _, ok := obj.(*renovatev1beta1.RenovateConfig); ok {
		return api_errors.NewNotFound(renovatev1beta1.GroupVersion.WithResource("renovateconfigs").GroupResource(), key.Name)
	}

	return m.Client.Get(ctx, key, obj, opts...)
}
