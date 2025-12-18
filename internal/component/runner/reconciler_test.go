package runner

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("calculateOptimalBatchSize", func() {
	var r *Reconciler

	Context("when explicit batch size is provided", func() {
		BeforeEach(func() {
			r = &Reconciler{
				instance: &renovatev1beta1.Runner{
					Spec: renovatev1beta1.RunnerSpec{
						Instances: 2,
						BatchSize: 15,
					},
				},
			}
		})

		It("should use the explicit batch size", func() {
			result := r.calculateOptimalBatchSize(100)
			Expect(result).To(Equal(15))
		})
	})

	Context("when batch size is auto-calculated", func() {
		Context("with multiple instances and many repositories", func() {
			BeforeEach(func() {
				r = &Reconciler{
					instance: &renovatev1beta1.Runner{
						Spec: renovatev1beta1.RunnerSpec{
							Instances: 4,
							BatchSize: 0, // not set, should auto-calculate

						},
					},
				}
			})

			It("should calculate optimal batch size", func() {
				result := r.calculateOptimalBatchSize(120)
				Expect(result).To(Equal(10)) // 120 / (4 * 3) = 10
			})
		})

		Context("with batch size exceeding maximum cap", func() {
			BeforeEach(func() {
				r = &Reconciler{
					instance: &renovatev1beta1.Runner{
						Spec: renovatev1beta1.RunnerSpec{
							Instances: 1,
							BatchSize: 0,
						},
					},
				}
			})

			It("should cap batch size at 50", func() {
				result := r.calculateOptimalBatchSize(300)
				Expect(result).To(Equal(50)) // would be 100, but capped at 50
			})
		})

		Context("with very few repositories", func() {
			BeforeEach(func() {
				r = &Reconciler{
					instance: &renovatev1beta1.Runner{
						Spec: renovatev1beta1.RunnerSpec{
							Instances: 10,
							BatchSize: 0,
						},
					},
				}
			})

			It("should set minimum batch size of 1", func() {
				result := r.calculateOptimalBatchSize(5)
				Expect(result).To(Equal(1)) // would be 0, but minimum is 1
			})
		})

		Context("with single instance", func() {
			BeforeEach(func() {
				r = &Reconciler{
					instance: &renovatev1beta1.Runner{
						Spec: renovatev1beta1.RunnerSpec{
							Instances: 1,
							BatchSize: 0,
						},
					},
				}
			})

			It("should calculate appropriate batch size", func() {
				result := r.calculateOptimalBatchSize(30)
				Expect(result).To(Equal(10)) // 30 / (1 * 3) = 10
			})
		})
	})
})

var _ = Describe("createBatches", func() {
	var (
		scheme     *runtime.Scheme
		fakeClient client.Client
		r          *Reconciler
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
		Expect(renovatev1beta1.AddToScheme(scheme)).To(Succeed())
	})

	setupTest := func(strategy renovatev1beta1.RunnerStrategy, instances, batchSize int, repos ...string) {
		var gitRepos []client.Object
		for _, repo := range repos {
			gitRepos = append(gitRepos, &renovatev1beta1.GitRepo{
				ObjectMeta: metav1.ObjectMeta{
					Name:      repo,
					Namespace: "test-namespace",
				},
				Spec: renovatev1beta1.GitRepoSpec{Name: repo},
			})
		}

		fakeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(gitRepos...).
			Build()

		r = &Reconciler{
			Client: fakeClient,
			req: ctrl.Request{
				NamespacedName: client.ObjectKey{
					Name:      "test-renovator",
					Namespace: "test-namespace",
				},
			},
			instance: &renovatev1beta1.Runner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator",
					Namespace: "test-namespace",
				},
				Spec: renovatev1beta1.RunnerSpec{
					Strategy:  strategy,
					Instances: int32(instances),
					BatchSize: batchSize,
				},
			},
		}
	}

	Context("with NONE strategy", func() {
		BeforeEach(func() {
			setupTest(renovatev1beta1.RunnerStrategy_NONE, 3, 0, "repo1", "repo2", "repo3", "repo4", "repo5")
		})

		It("should create single batch with all repositories", func() {
			batches, err := r.createBatches(context.TODO())
			Expect(err).NotTo(HaveOccurred())
			Expect(batches).To(HaveLen(1))
			Expect(batches[0].Repositories).To(HaveLen(5))
		})
	})

	Context("with BATCH strategy and explicit size", func() {
		BeforeEach(func() {
			setupTest(renovatev1beta1.RunnerStrategy_BATCH, 2, 2, "repo1", "repo2", "repo3", "repo4", "repo5")
		})

		It("should create multiple batches with specified size", func() {
			batches, err := r.createBatches(context.TODO())
			Expect(err).NotTo(HaveOccurred())
			Expect(batches).To(HaveLen(3))
			Expect(batches[0].Repositories).To(HaveLen(2))
			Expect(batches[1].Repositories).To(HaveLen(2))
			Expect(batches[2].Repositories).To(HaveLen(1))
		})
	})

	Context("with BATCH strategy and auto-calculation", func() {
		BeforeEach(func() {
			repositories := []string{"repo1", "repo2", "repo3", "repo4", "repo5", "repo6"}
			var gitRepos []client.Object

			for _, repo := range repositories {
				gitRepo := &renovatev1beta1.GitRepo{
					ObjectMeta: metav1.ObjectMeta{
						Name:      repo,
						Namespace: "test-namespace",
					},
					Spec: renovatev1beta1.GitRepoSpec{
						Name: repo,
					},
				}
				gitRepos = append(gitRepos, gitRepo)
			}

			fakeClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(gitRepos...).
				Build()

			instance := &renovatev1beta1.Runner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator",
					Namespace: "test-namespace",
				},
				Spec: renovatev1beta1.RunnerSpec{
					Strategy:  renovatev1beta1.RunnerStrategy_BATCH,
					Instances: 2,
					BatchSize: 0, // auto-calculate
				},
			}

			r = &Reconciler{
				Client: fakeClient,
				req: ctrl.Request{
					NamespacedName: client.ObjectKey{
						Name:      "test-renovator",
						Namespace: "test-namespace",
					},
				},
				instance: instance,
			}
		})

		It("should create batches with auto-calculated size", func() {
			batches, err := r.createBatches(context.TODO())
			Expect(err).NotTo(HaveOccurred())
			Expect(batches).To(HaveLen(6)) // 6 repos / 1 per batch (6 / (2*3) = 1)

			for _, batch := range batches {
				Expect(batch.Repositories).To(HaveLen(1))
			}
		})
	})

	Context("with empty repository list", func() {
		BeforeEach(func() {
			fakeClient = fake.NewClientBuilder().
				WithScheme(scheme).
				Build()

			instance := &renovatev1beta1.Runner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-renovator",
					Namespace: "test-namespace",
				},
				Spec: renovatev1beta1.RunnerSpec{
					Strategy:  renovatev1beta1.RunnerStrategy_BATCH,
					Instances: 2,
					BatchSize: 5,
				},
			}

			r = &Reconciler{
				Client: fakeClient,
				req: ctrl.Request{
					NamespacedName: client.ObjectKey{
						Name:      "test-renovator",
						Namespace: "test-namespace",
					},
				},
				instance: instance,
			}
		})

		It("should return empty batch list", func() {
			batches, err := r.createBatches(context.TODO())
			Expect(err).NotTo(HaveOccurred())
			Expect(batches).To(BeEmpty())
		})
	})
})
