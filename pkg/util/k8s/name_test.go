package k8s

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SanitizeName", func() {
	It("should convert repository path to valid name", func() {
		result, err := SanitizeName("owner/repo")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("owner-repo"))
	})

	It("should convert to lowercase", func() {
		result, err := SanitizeName("Owner/Repo")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("owner-repo"))
	})

	It("should handle multiple slashes", func() {
		result, err := SanitizeName("org/owner/repo")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("org-owner-repo"))
	})

	It("should handle empty string", func() {
		result, err := SanitizeName("")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(BeEmpty())
	})

	It("should handle string without slashes", func() {
		result, err := SanitizeName("repository")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("repository"))
	})

	It("should remove invalid characters", func() {
		result, err := SanitizeName("owner@repo#test")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("owner-repo-test"))
	})

	It("should handle names starting with invalid characters", func() {
		result, err := SanitizeName("-invalid-start")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("repo-invalid-start"))
	})

	It("should handle names ending with invalid characters", func() {
		result, err := SanitizeName("invalid-end-")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("invalid-end-repo"))
	})

	It("should handle consecutive hyphens", func() {
		result, err := SanitizeName("owner--repo")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("owner-repo"))
	})

	It("should handle mixed case and special characters", func() {
		result, err := SanitizeName("Owner/Repo_Name-123")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("owner-repo-name-123"))
	})

	It("should handle dots in names", func() {
		result, err := SanitizeName("owner.repo.name")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("owner-repo-name"))
	})

	It("should handle underscores in names", func() {
		result, err := SanitizeName("owner_repo_name")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("owner-repo-name"))
	})

	It("should truncate very long names", func() {
		longName := "owner/" + strings.Repeat("very-long-repo-name-", 20)
		result, err := SanitizeName(longName)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(result)).To(BeNumerically("<=", 253))
		Expect(result).To(HavePrefix("owner-very-long-repo-name-"))
		Expect(result).To(HaveSuffix("repo"))
	})

	It("should handle complex repository URLs", func() {
		result, err := SanitizeName("https://github.com/owner/repo.git")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("https-github-com-owner-repo-git"))
	})

	It("should return error for names with only invalid characters", func() {
		result, err := SanitizeName("!@#$%^&*()")
		Expect(err).To(HaveOccurred())
		Expect(result).To(BeEmpty())
	})
})
