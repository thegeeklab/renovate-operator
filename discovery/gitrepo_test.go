package discovery

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGitRepo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GitRepo Suite")
}

var _ = Describe("GitRepo", func() {
	Context("sanitizeRepoName", func() {
		It("should convert repository path to valid name", func() {
			Expect(sanitizeRepoName("owner/repo")).To(Equal("repo-owner-repo"))
		})

		It("should convert to lowercase", func() {
			Expect(sanitizeRepoName("Owner/Repo")).To(Equal("repo-owner-repo"))
		})

		It("should handle multiple slashes", func() {
			Expect(sanitizeRepoName("org/owner/repo")).To(Equal("repo-org-owner-repo"))
		})

		It("should handle empty string", func() {
			Expect(sanitizeRepoName("")).To(Equal("repo-"))
		})

		It("should handle string without slashes", func() {
			Expect(sanitizeRepoName("repository")).To(Equal("repo-repository"))
		})
	})
})

var _ = Describe("CreateGitRepo", func() {
	var (
		owner     *renovatev1beta1.Renovator
		namespace string
		repo      string
	)

	BeforeEach(func() {
		owner = &renovatev1beta1.Renovator{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-renovator",
				Namespace: "test-namespace",
				UID:       "test-uid",
			},
		}
		namespace = "test-namespace"
		repo = "owner/test-repo"
	})

	It("should create GitRepo with correct metadata", func() {
		gitRepo := CreateGitRepo(owner, namespace, repo)

		Expect(gitRepo.Name).To(Equal("repo-owner-test-repo"))
		Expect(gitRepo.Namespace).To(Equal(namespace))
		Expect(gitRepo.Spec.Name).To(Equal(repo))
	})

	It("should set correct owner reference", func() {
		gitRepo := CreateGitRepo(owner, namespace, repo)

		Expect(gitRepo.OwnerReferences).To(HaveLen(1))
		ownerRef := gitRepo.OwnerReferences[0]
		Expect(ownerRef.Name).To(Equal(owner.Name))
		Expect(ownerRef.UID).To(Equal(owner.UID))
		Expect(ownerRef.Kind).To(Equal("Renovator"))
		Expect(ownerRef.Controller).To(PointTo(BeTrue()))
	})
})
