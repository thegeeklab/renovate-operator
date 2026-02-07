package discovery

import (
	"context"
	"encoding/json"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
	)

	createDiscoveryCM := func(name string, repos []string) *corev1.ConfigMap {
		repoData, _ := json.Marshal(repos)
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
			},
			Data: map[string]string{"repositories": string(repoData)},
		}
		Expect(controllerutil.SetControllerReference(instance, cm, scheme)).To(Succeed())

		return cm
	}

	newGitRepo := func(name, specName string) *renovatev1beta1.GitRepo {
		return &renovatev1beta1.GitRepo{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
			},
			Spec: renovatev1beta1.GitRepoSpec{Name: specName},
		}
	}

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(renovatev1beta1.AddToScheme(scheme)).To(Succeed())
		Expect(corev1.AddToScheme(scheme)).To(Succeed())

		instance = &renovatev1beta1.Discovery{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-discovery",
				Namespace: "default",
				UID:       "test-uid",
				Labels: map[string]string{
					renovatev1beta1.RenovatorLabel: "test-renovator",
				},
			},
		}

		fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()
		reconciler = &Reconciler{Client: fakeClient, scheme: scheme, instance: instance}
		ctx = context.Background()
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
			Expect(repo.Labels).To(HaveKeyWithValue(renovatev1beta1.RenovatorLabel, "test-renovator"))
			Expect(metav1.IsControlledBy(&repo, instance)).To(BeTrue())
		})

		It("should skip ConfigMaps not controlled by the instance", func() {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "stranger-danger", Namespace: "default"},
				Data:       map[string]string{"repositories": `["repo1"]`},
			}
			Expect(fakeClient.Create(ctx, cm)).To(Succeed())

			_, err := reconciler.reconcileGitRepos(ctx)
			Expect(err).ToNot(HaveOccurred())

			gitRepos := &renovatev1beta1.GitRepoList{}
			Expect(fakeClient.List(ctx, gitRepos)).To(Succeed())
			Expect(gitRepos.Items).To(BeEmpty())
		})

		It("should handle errors when listing ConfigMaps", func() {
			// Create a new fake client that simulates an error
			mockClient := &mockErrorClient{}
			reconciler := &Reconciler{Client: mockClient, scheme: scheme, instance: instance}

			_, err := reconciler.reconcileGitRepos(ctx)
			Expect(err).To(HaveOccurred())
		})

		It("should handle errors when creating or updating GitRepos", func() {
			cm := createDiscoveryCM("test-config", []string{"repo1"})
			Expect(fakeClient.Create(ctx, cm)).To(Succeed())

			// Simulate an error by using a mock client that will fail during creation
			mockClient := &mockErrorClient{}
			reconciler := &Reconciler{Client: mockClient, scheme: scheme, instance: instance}

			_, err := reconciler.reconcileGitRepos(ctx)
			Expect(err).To(HaveOccurred())
		})

		It("should handle errors when pruning orphaned repos", func() {
			cm := createDiscoveryCM("test-config", []string{"repo1"})
			Expect(fakeClient.Create(ctx, cm)).To(Succeed())

			// Simulate an error by using a mock client that will fail during pruning
			mockClient := &mockErrorClient{}
			reconciler := &Reconciler{Client: mockClient, scheme: scheme, instance: instance}

			_, err := reconciler.reconcileGitRepos(ctx)
			Expect(err).To(HaveOccurred())
		})

		It("should handle invalid JSON in ConfigMap", func() {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config",
					Namespace: "default",
				},
				Data: map[string]string{"repositories": `invalid-json`},
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
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config",
					Namespace: "default",
				},
				Data: map[string]string{"other-key": `value`},
			}
			Expect(controllerutil.SetControllerReference(instance, cm, scheme)).To(Succeed())
			Expect(fakeClient.Create(ctx, cm)).To(Succeed())

			_, err := reconciler.reconcileGitRepos(ctx)
			Expect(err).ToNot(HaveOccurred())

			gitRepos := &renovatev1beta1.GitRepoList{}
			Expect(fakeClient.List(ctx, gitRepos)).To(Succeed())
			Expect(gitRepos.Items).To(BeEmpty())
		})
	})

	Describe("updateGitRepo", func() {
		It("should propagate specific labels from discovery instance to GitRepo", func() {
			repo := &renovatev1beta1.GitRepo{}
			err := reconciler.updateGitRepo(repo, "my-repo")
			Expect(err).ToNot(HaveOccurred())

			Expect(repo.Spec.Name).To(Equal("my-repo"))
			Expect(repo.Labels).To(HaveKeyWithValue(renovatev1beta1.RenovatorLabel, "test-renovator"))
		})

		It("should handle missing labels on discovery instance gracefully", func() {
			reconciler.instance.Labels = nil
			repo := &renovatev1beta1.GitRepo{}

			err := reconciler.updateGitRepo(repo, "my-repo")
			Expect(err).ToNot(HaveOccurred())
			Expect(repo.Labels).To(Not(HaveKey(renovatev1beta1.RenovatorLabel)))
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
			// Create a mock client that simulates an error
			mockClient := &mockErrorClient{}
			reconciler := &Reconciler{Client: mockClient, scheme: scheme, instance: instance}

			err := reconciler.pruneOrphanedRepos(ctx, map[string]bool{})
			Expect(err).To(HaveOccurred())
		})

		It("should handle errors when deleting orphaned GitRepos", func() {
			orphan := newGitRepo("test-discovery-orphan", "delete-me")
			Expect(controllerutil.SetControllerReference(instance, orphan, scheme)).To(Succeed())
			Expect(fakeClient.Create(ctx, orphan)).To(Succeed())

			// Simulate an error by using a mock client that will fail during deletion
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

// mockErrorClient simulates a client that returns errors for testing purposes.
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
