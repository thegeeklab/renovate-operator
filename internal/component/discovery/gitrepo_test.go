package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/mock"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/internal/metrics"
	"github.com/thegeeklab/renovate-operator/internal/provider"
	"github.com/thegeeklab/renovate-operator/internal/provider/factory"
	"github.com/thegeeklab/renovate-operator/internal/provider/mocks"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("GitRepo Reconciliation", func() {
	var (
		fakeClient client.Client
		reconciler *Reconciler
		instance   *renovatev1beta1.Discovery
		ctx        context.Context
		scheme     *runtime.Scheme
		mockMgr    *mocks.ProviderManager
	)

	createDiscoveryCM := func(name string, repos []string) *corev1.ConfigMap {
		repoData, _ := json.Marshal(repos)
		cm := &corev1.ConfigMap{
			Name:      name,
			Namespace: "default",
			Data:      map[string]string{"repositories": string(repoData)},
		}
		Expect(controllerutil.SetControllerReference(instance, cm, scheme)).To(Succeed())

		return cm
	}

	newGitRepo := func(name, specName string) *renovatev1beta1.GitRepo {
		return &renovatev1beta1.GitRepo{
			Name:      name,
			Namespace: "default",
			Spec:      renovatev1beta1.GitRepoSpec{Name: specName},
		}
	}

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(renovatev1beta1.AddToScheme(scheme)).To(Succeed())
		Expect(corev1.AddToScheme(scheme)).To(Succeed())

		instance = &renovatev1beta1.Discovery{
			Name:      "test-discovery",
			Namespace: "default",
			UID:       "test-uid",
			Labels: map[string]string{
				renovatev1beta1.LabelRenovator: "test-renovator",
			},
		}

		fakeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(instance).
			WithStatusSubresource(instance).
			Build()
		reconciler = &Reconciler{
			Client:        fakeClient,
			scheme:        scheme,
			instance:      instance,
			eventRecorder: &events.FakeRecorder{},
		}
		ctx = context.Background()

		mockMgr = mocks.NewProviderManager(GinkgoT())
		reconciler.providerFactory = func(
			context.Context, factory.PlatformConfig,
		) (provider.ProviderManager, error) {
			return mockMgr, nil
		}
	})

	AfterEach(func() {
		mockMgr.AssertExpectations(GinkgoT())
	})

	Describe("reconcileGitRepos", func() {
		It("should successfully create GitRepos with inherited labels", func() {
			cm := createDiscoveryCM("test-config", []string{"repo1"})
			Expect(fakeClient.Create(ctx, cm)).To(Succeed())

			_, err := reconciler.reconcileGitRepos(ctx)
			Expect(err).ToNot(HaveOccurred())

			gitRepos := &renovatev1beta1.GitRepoList{}
			Expect(fakeClient.List(ctx, gitRepos)).To(Succeed())
			Expect(gitRepos.Items).To(HaveLen(1))

			repo := gitRepos.Items[0]
			Expect(repo.Spec.Name).To(Equal("repo1"))
			Expect(repo.Labels).To(HaveKeyWithValue(renovatev1beta1.LabelRenovator, "test-renovator"))
			Expect(metav1.IsControlledBy(&repo, instance)).To(BeTrue())
		})

		It("should skip ConfigMaps not controlled by the instance", func() {
			cm := &corev1.ConfigMap{
				Name: "stranger-danger", Namespace: "default",
				Data: map[string]string{"repositories": `["repo1"]`},
			}
			Expect(fakeClient.Create(ctx, cm)).To(Succeed())

			_, err := reconciler.reconcileGitRepos(ctx)
			Expect(err).ToNot(HaveOccurred())

			gitRepos := &renovatev1beta1.GitRepoList{}
			Expect(fakeClient.List(ctx, gitRepos)).To(Succeed())
			Expect(gitRepos.Items).To(BeEmpty())
		})

		It("should handle errors when listing ConfigMaps", func() {
			mockClient := &mockErrorClient{}
			reconciler := &Reconciler{Client: mockClient, scheme: scheme, instance: instance}

			_, err := reconciler.reconcileGitRepos(ctx)
			Expect(err).To(HaveOccurred())
		})

		It("should handle errors when creating or updating GitRepos", func() {
			cm := createDiscoveryCM("test-config", []string{"repo1"})
			Expect(fakeClient.Create(ctx, cm)).To(Succeed())

			mockClient := &mockErrorClient{}
			reconciler := &Reconciler{Client: mockClient, scheme: scheme, instance: instance}

			_, err := reconciler.reconcileGitRepos(ctx)
			Expect(err).To(HaveOccurred())
		})

		It("should handle errors when pruning orphaned repos", func() {
			cm := createDiscoveryCM("test-config", []string{"repo1"})
			Expect(fakeClient.Create(ctx, cm)).To(Succeed())

			mockClient := &mockErrorClient{}
			reconciler := &Reconciler{Client: mockClient, scheme: scheme, instance: instance}

			_, err := reconciler.reconcileGitRepos(ctx)
			Expect(err).To(HaveOccurred())
		})

		It("should handle invalid JSON in ConfigMap", func() {
			cm := &corev1.ConfigMap{
				Name:      "test-config",
				Namespace: "default",
				Data:      map[string]string{"repositories": `invalid-json`},
			}
			Expect(controllerutil.SetControllerReference(instance, cm, scheme)).To(Succeed())
			Expect(fakeClient.Create(ctx, cm)).To(Succeed())

			_, err := reconciler.reconcileGitRepos(ctx)
			Expect(err).ToNot(HaveOccurred())

			gitRepos := &renovatev1beta1.GitRepoList{}
			Expect(fakeClient.List(ctx, gitRepos)).To(Succeed())
			Expect(gitRepos.Items).To(BeEmpty())
		})

		It("should handle missing 'repositories' key in ConfigMap", func() {
			cm := &corev1.ConfigMap{
				Name:      "test-config",
				Namespace: "default",
				Data:      map[string]string{"other-key": `value`},
			}
			Expect(controllerutil.SetControllerReference(instance, cm, scheme)).To(Succeed())
			Expect(fakeClient.Create(ctx, cm)).To(Succeed())

			_, err := reconciler.reconcileGitRepos(ctx)
			Expect(err).ToNot(HaveOccurred())

			gitRepos := &renovatev1beta1.GitRepoList{}
			Expect(fakeClient.List(ctx, gitRepos)).To(Succeed())
			Expect(gitRepos.Items).To(BeEmpty())
		})

		Context("when the discovery result is empty or invalid", func() {
			It("should mark DiscoveryFailed when the result is empty", func() {
				cm := createDiscoveryCM("test-config", []string{})
				Expect(fakeClient.Create(ctx, cm)).To(Succeed())

				_, err := reconciler.reconcileGitRepos(ctx)
				Expect(err).ToNot(HaveOccurred())

				failedCond := instance.GetCondition(renovatev1beta1.DiscoveryConditionDiscoveryFailed)
				Expect(failedCond).NotTo(BeNil())
				Expect(failedCond.Status).To(Equal(metav1.ConditionTrue))
				Expect(failedCond.Reason).To(Equal(renovatev1beta1.ReasonEmptyDiscoveryResult))

				Expect(instance.GetCondition(renovatev1beta1.DiscoveryConditionDiscoveryCompleted)).To(BeNil())

				gitRepos := &renovatev1beta1.GitRepoList{}
				Expect(fakeClient.List(ctx, gitRepos)).To(Succeed())
				Expect(gitRepos.Items).To(BeEmpty())
			})

			It("should not prune existing GitRepos when the result is empty", func() {
				existing := newGitRepo("test-discovery-existing", "existing-repo")
				Expect(controllerutil.SetControllerReference(instance, existing, scheme)).To(Succeed())
				Expect(fakeClient.Create(ctx, existing)).To(Succeed())

				cm := createDiscoveryCM("test-config", []string{})
				Expect(fakeClient.Create(ctx, cm)).To(Succeed())

				_, err := reconciler.reconcileGitRepos(ctx)
				Expect(err).ToNot(HaveOccurred())

				gitRepos := &renovatev1beta1.GitRepoList{}
				Expect(fakeClient.List(ctx, gitRepos)).To(Succeed())
				Expect(gitRepos.Items).To(HaveLen(1))
				Expect(gitRepos.Items[0].Spec.Name).To(Equal("existing-repo"))
			})

			It("should mark DiscoveryFailed when the 'repositories' key is missing", func() {
				cm := &corev1.ConfigMap{
					Name:      "test-config",
					Namespace: "default",
					Data:      map[string]string{"other-key": `value`},
				}
				Expect(controllerutil.SetControllerReference(instance, cm, scheme)).To(Succeed())
				Expect(fakeClient.Create(ctx, cm)).To(Succeed())

				_, err := reconciler.reconcileGitRepos(ctx)
				Expect(err).ToNot(HaveOccurred())

				failedCond := instance.GetCondition(renovatev1beta1.DiscoveryConditionDiscoveryFailed)
				Expect(failedCond).NotTo(BeNil())
				Expect(failedCond.Reason).To(Equal(renovatev1beta1.ReasonInvalidDiscoveryResult))
			})

			It("should mark DiscoveryFailed when the result JSON is invalid", func() {
				cm := &corev1.ConfigMap{
					Name:      "test-config",
					Namespace: "default",
					Data:      map[string]string{"repositories": `invalid-json`},
				}
				Expect(controllerutil.SetControllerReference(instance, cm, scheme)).To(Succeed())
				Expect(fakeClient.Create(ctx, cm)).To(Succeed())

				_, err := reconciler.reconcileGitRepos(ctx)
				Expect(err).ToNot(HaveOccurred())

				failedCond := instance.GetCondition(renovatev1beta1.DiscoveryConditionDiscoveryFailed)
				Expect(failedCond).NotTo(BeNil())
				Expect(failedCond.Reason).To(Equal(renovatev1beta1.ReasonInvalidDiscoveryResult))
			})

			It("should record a discovery failure metric when the result is empty", func() {
				reg := prometheus.NewRegistry()
				reconciler.metrics = metrics.New(reg, reg, 5000)

				cm := createDiscoveryCM("test-config", []string{})
				Expect(fakeClient.Create(ctx, cm)).To(Succeed())

				_, err := reconciler.reconcileGitRepos(ctx)
				Expect(err).ToNot(HaveOccurred())

				//nolint:lll
				expected := `
					# HELP renovate_operator_discovery_job_failures_total Total number of failed discovery Kubernetes Jobs by failure reason.
					# TYPE renovate_operator_discovery_job_failures_total counter
					renovate_operator_discovery_job_failures_total{discovery="test-discovery",namespace="default",reason="EmptyDiscoveryResult",renovator="test-renovator"} 1
				`

				err = testutil.GatherAndCompare(
					reg, strings.NewReader(expected), "renovate_operator_discovery_job_failures_total",
				)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should transition from DiscoveryCompleted to DiscoveryFailed on empty result", func() {
				instance.SetCondition(
					renovatev1beta1.DiscoveryConditionDiscoveryCompleted,
					metav1.ConditionTrue,
					"JobSucceeded", "Discovery job completed successfully",
				)
				Expect(fakeClient.Status().Update(ctx, instance)).To(Succeed())

				cm := createDiscoveryCM("test-config", []string{})
				Expect(fakeClient.Create(ctx, cm)).To(Succeed())

				_, err := reconciler.reconcileGitRepos(ctx)
				Expect(err).ToNot(HaveOccurred())

				Expect(instance.GetCondition(renovatev1beta1.DiscoveryConditionDiscoveryCompleted)).To(BeNil())

				failedCond := instance.GetCondition(renovatev1beta1.DiscoveryConditionDiscoveryFailed)
				Expect(failedCond).NotTo(BeNil())
				Expect(failedCond.Status).To(Equal(metav1.ConditionTrue))
			})
		})

		Context("when skipForks is enabled", func() {
			BeforeEach(func() {
				skipForks := true
				instance.Spec.SkipForks = &skipForks

				reconciler.renovate = &renovatev1beta1.RenovateConfig{
					Name: "test-config", Namespace: "default",
					Spec: renovatev1beta1.RenovateConfigSpec{
						Platform: renovatev1beta1.PlatformSpec{
							Type: "stub",
							Token: corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									Key:  "token",
									Name: "platform-secret",
								},
							},
						},
					},
				}

				tokenSecret := &corev1.Secret{
					Name: "platform-secret", Namespace: "default",
					Data: map[string][]byte{"token": []byte("test-token")},
				}
				Expect(fakeClient.Create(ctx, tokenSecret)).To(Succeed())
			})

			It("should not create GitRepos for forked repositories and should not prune non-forks", func() {
				mockMgr.On("ListRepos", mock.Anything, provider.ListReposOptions{SkipForks: true}).
					Return([]provider.Repo{
						{Name: "real-repo", IsFork: false},
					}, nil).
					Once()

				cm := createDiscoveryCM("test-config", []string{"real-repo", "forked-repo"})
				Expect(fakeClient.Create(ctx, cm)).To(Succeed())

				_, err := reconciler.reconcileGitRepos(ctx)
				Expect(err).ToNot(HaveOccurred())

				gitRepos := &renovatev1beta1.GitRepoList{}
				Expect(fakeClient.List(ctx, gitRepos)).To(Succeed())
				Expect(gitRepos.Items).To(HaveLen(1))
				Expect(gitRepos.Items[0].Spec.Name).To(Equal("real-repo"))
			})
		})
	})

	Describe("filterRepos", func() {
		It("should return the input unchanged when skipForks is disabled and no topics are set", func() {
			repos := []string{"a", "b", "c"}
			result, err := reconciler.filterRepos(ctx, repos)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(repos))
		})

		It("should exclude forked repositories when skipForks is enabled", func() {
			skipForks := true
			reconciler.instance.Spec.SkipForks = &skipForks

			reconciler.renovate = &renovatev1beta1.RenovateConfig{
				Name: "test-config", Namespace: "default",
				Spec: renovatev1beta1.RenovateConfigSpec{
					Platform: renovatev1beta1.PlatformSpec{
						Type: "stub",
						Token: corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								Key:  "token",
								Name: "platform-secret",
							},
						},
					},
				},
			}

			tokenSecret := &corev1.Secret{
				Name: "platform-secret", Namespace: "default",
				Data: map[string][]byte{"token": []byte("test-token")},
			}
			Expect(fakeClient.Create(ctx, tokenSecret)).To(Succeed())

			mockMgr.On("ListRepos", mock.Anything, provider.ListReposOptions{SkipForks: true}).
				Return([]provider.Repo{
					{Name: "real", IsFork: false},
					{Name: "another", IsFork: false},
				}, nil).
				Once()

			result, err := reconciler.filterRepos(ctx, []string{"real", "forked", "another"})
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal([]string{"real", "another"}))
		})

		It("should filter repositories by topics when topics are set", func() {
			reconciler.instance.Spec.Topics = []string{"renovate", "production"}

			reconciler.renovate = &renovatev1beta1.RenovateConfig{
				Name: "test-config", Namespace: "default",
				Spec: renovatev1beta1.RenovateConfigSpec{
					Platform: renovatev1beta1.PlatformSpec{
						Type: "stub",
						Token: corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								Key:  "token",
								Name: "platform-secret",
							},
						},
					},
				},
			}

			tokenSecret := &corev1.Secret{
				Name: "platform-secret", Namespace: "default",
				Data: map[string][]byte{"token": []byte("test-token")},
			}
			Expect(fakeClient.Create(ctx, tokenSecret)).To(Succeed())

			mockMgr.On("ListRepos", mock.Anything, provider.ListReposOptions{
				Topics: []string{"renovate", "production"},
			}).Return([]provider.Repo{
				{Name: "matching-repo"},
			}, nil).Once()

			result, err := reconciler.filterRepos(ctx, []string{"matching-repo", "non-matching-repo"})
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal([]string{"matching-repo"}))
		})

		It("should apply both skipForks and topics filters together", func() {
			skipForks := true
			reconciler.instance.Spec.SkipForks = &skipForks
			reconciler.instance.Spec.Topics = []string{"renovate"}

			reconciler.renovate = &renovatev1beta1.RenovateConfig{
				Name: "test-config", Namespace: "default",
				Spec: renovatev1beta1.RenovateConfigSpec{
					Platform: renovatev1beta1.PlatformSpec{
						Type: "stub",
						Token: corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								Key:  "token",
								Name: "platform-secret",
							},
						},
					},
				},
			}

			tokenSecret := &corev1.Secret{
				Name: "platform-secret", Namespace: "default",
				Data: map[string][]byte{"token": []byte("test-token")},
			}
			Expect(fakeClient.Create(ctx, tokenSecret)).To(Succeed())

			mockMgr.On("ListRepos", mock.Anything, provider.ListReposOptions{
				SkipForks: true,
				Topics:    []string{"renovate"},
			}).Return([]provider.Repo{
				{Name: "matching-repo", IsFork: false},
			}, nil).Once()

			result, err := reconciler.filterRepos(ctx, []string{"matching-repo", "non-matching-repo"})
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal([]string{"matching-repo"}))
		})

		It("should exclude repos marked for deletion when skipPendingDeletion is enabled", func() {
			skipPending := true
			reconciler.instance.Spec.GitLab = &renovatev1beta1.GitLabOptions{
				SkipPendingDeletion: &skipPending,
			}

			reconciler.renovate = &renovatev1beta1.RenovateConfig{
				Name: "test-config", Namespace: "default",
				Spec: renovatev1beta1.RenovateConfigSpec{
					Platform: renovatev1beta1.PlatformSpec{
						Type: "stub",
						Token: corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								Key:  "token",
								Name: "platform-secret",
							},
						},
					},
				},
			}

			tokenSecret := &corev1.Secret{
				Name: "platform-secret", Namespace: "default",
				Data: map[string][]byte{"token": []byte("test-token")},
			}
			Expect(fakeClient.Create(ctx, tokenSecret)).To(Succeed())

			mockMgr.On("ListRepos", mock.Anything, provider.ListReposOptions{
				SkipPendingDeletion: true,
			}).Return([]provider.Repo{
				{Name: "active-repo"},
			}, nil).Once()

			result, err := reconciler.filterRepos(ctx, []string{"active-repo", "pending-delete-repo"})
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal([]string{"active-repo"}))
		})
	})

	Describe("updateGitRepo", func() {
		It("should propagate specific labels from discovery instance to GitRepo", func() {
			repo := &renovatev1beta1.GitRepo{}
			err := reconciler.updateGitRepo(repo, "my-repo")
			Expect(err).ToNot(HaveOccurred())

			Expect(repo.Spec.Name).To(Equal("my-repo"))
			Expect(repo.Labels).To(HaveKeyWithValue(renovatev1beta1.LabelRenovator, "test-renovator"))
		})

		It("should handle missing labels on discovery instance gracefully", func() {
			reconciler.instance.Labels = nil
			repo := &renovatev1beta1.GitRepo{}

			err := reconciler.updateGitRepo(repo, "my-repo")
			Expect(err).ToNot(HaveOccurred())
			Expect(repo.Labels).To(Not(HaveKey(renovatev1beta1.LabelRenovator)))
		})
	})

	Describe("pruneOrphanedRepos", func() {
		It("should delete orphaned GitRepos but keep discovered ones", func() {
			keep := newGitRepo("test-discovery-keep", "keep-me")
			Expect(controllerutil.SetControllerReference(instance, keep, scheme)).To(Succeed())

			orphan := newGitRepo("test-discovery-orphan", "delete-me")
			Expect(controllerutil.SetControllerReference(instance, orphan, scheme)).To(Succeed())

			Expect(fakeClient.Create(ctx, keep)).To(Succeed())
			Expect(fakeClient.Create(ctx, orphan)).To(Succeed())

			discovered := map[string]bool{"keep-me": true}
			Expect(reconciler.pruneOrphanedRepos(ctx, discovered)).To(Succeed())

			list := &renovatev1beta1.GitRepoList{}
			Expect(fakeClient.List(ctx, list)).To(Succeed())
			Expect(list.Items).To(HaveLen(1))
			Expect(list.Items[0].Spec.Name).To(Equal("keep-me"))
		})

		It("should ignore GitRepos not owned by the Discovery instance", func() {
			externalRepo := newGitRepo("other-controller-repo", "some-repo")
			Expect(fakeClient.Create(ctx, externalRepo)).To(Succeed())

			Expect(reconciler.pruneOrphanedRepos(ctx, map[string]bool{})).To(Succeed())

			list := &renovatev1beta1.GitRepoList{}
			Expect(fakeClient.List(ctx, list)).To(Succeed())
			Expect(list.Items).To(HaveLen(1))
		})

		It("should handle errors when listing existing GitRepos", func() {
			mockClient := &mockErrorClient{}
			reconciler := &Reconciler{Client: mockClient, scheme: scheme, instance: instance}

			err := reconciler.pruneOrphanedRepos(ctx, map[string]bool{})
			Expect(err).To(HaveOccurred())
		})

		It("should handle errors when deleting orphaned GitRepos", func() {
			orphan := newGitRepo("test-discovery-orphan", "delete-me")
			Expect(controllerutil.SetControllerReference(instance, orphan, scheme)).To(Succeed())
			Expect(fakeClient.Create(ctx, orphan)).To(Succeed())

			mockClient := &mockErrorClient{}
			reconciler := &Reconciler{Client: mockClient, scheme: scheme, instance: instance}

			err := reconciler.pruneOrphanedRepos(ctx, map[string]bool{})
			Expect(err).To(HaveOccurred())

			list := &renovatev1beta1.GitRepoList{}
			Expect(fakeClient.List(ctx, list)).To(Succeed())
			Expect(list.Items).To(HaveLen(1))
		})
	})
})

type mockErrorClient struct {
	client.Client
}

func (m *mockErrorClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return errors.New("simulated error")
}

func (m *mockErrorClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	return errors.New("simulated error")
}

func (m *mockErrorClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	return errors.New("simulated error")
}

func (m *mockErrorClient) Get(
	ctx context.Context,
	key client.ObjectKey,
	obj client.Object,
	opts ...client.GetOption,
) error {
	return errors.New("simulated error")
}
