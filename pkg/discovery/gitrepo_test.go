package discovery

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
		gitRepo, err := CreateGitRepo(owner, namespace, repo)
		Expect(err).ToNot(HaveOccurred())

		Expect(gitRepo.Name).To(Equal("owner-test-repo"))
		Expect(gitRepo.Namespace).To(Equal(namespace))
		Expect(gitRepo.Spec.Name).To(Equal(repo))
	})

	It("should set correct owner reference", func() {
		gitRepo, err := CreateGitRepo(owner, namespace, repo)
		Expect(err).ToNot(HaveOccurred())

		Expect(gitRepo.OwnerReferences).To(HaveLen(1))
		ownerRef := gitRepo.OwnerReferences[0]
		Expect(ownerRef.Name).To(Equal(owner.Name))
		Expect(ownerRef.UID).To(Equal(owner.UID))
		Expect(ownerRef.Kind).To(Equal("Renovator"))
		Expect(ownerRef.Controller).To(PointTo(BeTrue()))
	})
})
