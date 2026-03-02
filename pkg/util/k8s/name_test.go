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

	It("should handle dots in names (converting to hyphens)", func() {
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
		Expect(result).To(HaveLen(DNS1123MaxLength))
		Expect(result).To(HavePrefix("owner-very-long-repo-name-"))
		// After truncation, check it ends with an alphanumeric character
		Expect(result[len(result)-1:]).To(MatchRegexp(`[a-z0-9]`))
	})

	It("should handle complex repository URLs (converting dots to hyphens)", func() {
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

var _ = Describe("DeterministicName", func() {
	It("should append suffix without hashing if combined length is under 63", func() {
		result, err := DeterministicName("my-repo", "-12345")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("my-repo-12345"))
		Expect(len(result)).To(BeNumerically("<=", 63))
	})

	It("should append suffix without hashing if combined length is exactly 63", func() {
		base := strings.Repeat("a", 57) // 57 chars
		suffix := "-12345"              // 6 chars
		result, err := DeterministicName(base, suffix)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(base + suffix))
		Expect(result).To(HaveLen(63))
	})

	It("should hash and truncate if combined length exceeds 63 characters", func() {
		base := strings.Repeat("a", 60) // 60 chars
		suffix := "-12345"              // 6 chars, total 66

		result, err := DeterministicName(base, suffix)
		Expect(err).ToNot(HaveOccurred())
		// Length could be slightly less than 63 if truncation lands on a trailing hyphen
		Expect(len(result)).To(BeNumerically("<=", 63))
		Expect(result).To(HaveSuffix(suffix))

		expectedBaseLen := 63 - len(suffix) - 9 // 9 = length of "-<hash>"
		Expect(result).To(HavePrefix(strings.Repeat("a", expectedBaseLen)))
	})

	It("should yield completely different names for similarly long bases to avoid collisions", func() {
		base1 := "very-long-organization-name/very-long-repo-name-frontend-app"
		base2 := "very-long-organization-name/very-long-repo-name-backend-api"
		suffix := "-12345"

		result1, err1 := DeterministicName(base1, suffix)
		result2, err2 := DeterministicName(base2, suffix)

		Expect(err1).ToNot(HaveOccurred())
		Expect(err2).ToNot(HaveOccurred())

		Expect(len(result1)).To(BeNumerically("<=", 63))
		Expect(len(result2)).To(BeNumerically("<=", 63))

		// Even though they share a massive common prefix, the hash ensures they are distinct
		Expect(result1).ToNot(Equal(result2))
	})

	It("should cleanly trim trailing hyphens from the truncated base before appending the hash", func() {
		// We carefully craft a string where the truncation point falls exactly on a hyphen
		base := strings.Repeat("a", 49) + "-" + strings.Repeat("b", 20)
		suffix := "-123"

		result, err := DeterministicName(base, suffix)
		Expect(err).ToNot(HaveOccurred())

		// It should NOT look like "aaaaa---<hash>-123"
		Expect(result).ToNot(ContainSubstring("--"))
		Expect(len(result)).To(BeNumerically("<=", 63))
	})

	It("should return an error if the base name consists of entirely invalid characters", func() {
		result, err := DeterministicName("!@#$%", "-12345")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("invalid characters"))
		Expect(result).To(BeEmpty())
	})

	It("should return an error if the suffix itself is too long to fit", func() {
		base := "valid-base"
		suffix := "-" + strings.Repeat("1", 60) // 61 chars

		result, err := DeterministicName(base, suffix)
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError(errInvalidSuffix))
		Expect(result).To(BeEmpty())
	})
})
