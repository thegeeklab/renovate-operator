package runner

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Reconciler Index Creation", func() {
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

	setupTest := func(repos ...string) {
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

		instance := &renovatev1beta1.Runner{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-runner",
				Namespace: "test-namespace",
			},
			Spec: renovatev1beta1.RunnerSpec{
				Instances: 1,
			},
		}

		var err error

		r, err = NewReconciler(
			context.Background(),
			fakeClient,
			scheme,
			instance,
			&renovatev1beta1.RenovateConfig{},
		)

		Expect(err).NotTo(HaveOccurred())
	}

	Context("NewReconciler", func() {
		It("should create index with correct number of repositories", func() {
			setupTest("repo1", "repo2", "repo3")
			Expect(r.index).To(HaveLen(3))
			Expect(r.indexCount).To(Equal(int32(3)))
		})

		It("should create index with correct repository names", func() {
			setupTest("repo1", "repo2")
			Expect(r.index[0].Repositories).To(Equal([]string{"repo1"}))
			Expect(r.index[1].Repositories).To(Equal([]string{"repo2"}))
		})

		It("should handle empty repository list", func() {
			setupTest()
			Expect(r.index).To(BeEmpty())
			Expect(r.indexCount).To(Equal(int32(0)))
		})
	})
})
