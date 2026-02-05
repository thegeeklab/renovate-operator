package discovery

import (
	"context"
	"encoding/json"

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

		return &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
				Labels:    map[string]string{renovatev1beta1.DiscoveryInstance: "test-discovery"},
			},
			Data: map[string]string{"repositories": string(repoData)},
		}
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
			},
		}

		fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()
		reconciler = &Reconciler{Client: fakeClient, scheme: scheme, instance: instance}
		ctx = context.Background()
	})

	Describe("reconcileGitRepos", func() {
		It("should successfully create GitRepos from discovery results", func() {
			cm := createDiscoveryCM("test-config", []string{"repo1", "repo2"})
			Expect(fakeClient.Create(ctx, cm)).To(Succeed())

			_, err := reconciler.reconcileGitRepos(ctx)
			Expect(err).ToNot(HaveOccurred())

			gitRepos := &renovatev1beta1.GitRepoList{}
			Expect(fakeClient.List(ctx, gitRepos)).To(Succeed())
			Expect(gitRepos.Items).To(HaveLen(2))
		})

		It("should update existing GitRepos when Discovery labels change", func() {
			existingRepo := newGitRepo("test-discovery-repo1", "repo1")
			existingRepo.Labels = map[string]string{renovatev1beta1.DiscoveryInstance: instance.Name}
			Expect(controllerutil.SetControllerReference(instance, existingRepo, scheme)).To(Succeed())
			Expect(fakeClient.Create(ctx, existingRepo)).To(Succeed())

			instance.Labels = map[string]string{renovatev1beta1.RenovatorLabel: "new-manager"}

			cm := createDiscoveryCM("test-config", []string{"repo1"})
			Expect(fakeClient.Create(ctx, cm)).To(Succeed())

			_, err := reconciler.reconcileGitRepos(ctx)
			Expect(err).ToNot(HaveOccurred())

			updated := &renovatev1beta1.GitRepo{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(existingRepo), updated)).To(Succeed())
			Expect(updated.Labels).To(HaveKeyWithValue(renovatev1beta1.RenovatorLabel, "new-manager"))
		})

		It("should handle invalid JSON data gracefully", func() {
			cm := createDiscoveryCM("bad-config", nil)
			cm.Data["repositories"] = "{not-json}"
			Expect(fakeClient.Create(ctx, cm)).To(Succeed())

			_, err := reconciler.reconcileGitRepos(ctx)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	DescribeTable("updateGitRepo Logic",
		func(setupInstance func(), expectedLabels map[string]string) {
			if setupInstance != nil {
				setupInstance()
			}
			repo := &renovatev1beta1.GitRepo{}
			Expect(reconciler.updateGitRepo(repo, "test-repo")).To(Succeed())

			for k, v := range expectedLabels {
				Expect(repo.Labels).To(HaveKeyWithValue(k, v))
			}
			Expect(repo.Spec.Name).To(Equal("test-repo"))
		},
		Entry("Standard labels", nil, map[string]string{
			renovatev1beta1.DiscoveryInstance: "test-discovery",
			renovatev1beta1.JobTypeLabelKey:   renovatev1beta1.JobTypeLabelValue,
		}),
		Entry("Renovator label propagation", func() {
			instance.Labels = map[string]string{renovatev1beta1.RenovatorLabel: "custom-app"}
		}, map[string]string{
			renovatev1beta1.RenovatorLabel: "custom-app",
		}),
	)

	Describe("pruneOrphanedRepos", func() {
		It("should delete orphaned GitRepos but keep discovered ones", func() {
			keep := newGitRepo("test-discovery-keep", "keep-me")
			keep.Labels = map[string]string{renovatev1beta1.DiscoveryInstance: instance.Name}
			Expect(controllerutil.SetControllerReference(instance, keep, scheme)).To(Succeed())

			orphan := newGitRepo("test-discovery-orphan", "delete-me")
			orphan.Labels = map[string]string{renovatev1beta1.DiscoveryInstance: instance.Name}
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
			externalRepo.Labels = map[string]string{renovatev1beta1.DiscoveryInstance: instance.Name}
			Expect(fakeClient.Create(ctx, externalRepo)).To(Succeed())

			Expect(reconciler.pruneOrphanedRepos(ctx, map[string]bool{})).To(Succeed())

			list := &renovatev1beta1.GitRepoList{}
			Expect(fakeClient.List(ctx, list)).To(Succeed())
			Expect(list.Items).To(HaveLen(1))
		})
	})
})
