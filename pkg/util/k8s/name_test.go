package k8s

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/validation"
)

var _ = Describe("truncateWithHash", func() {
	It("should truncate and append hash within maxLen", func() {
		result := truncateWithHash(strings.Repeat("a", 100), "original", 30, "-")
		Expect(len(result)).To(BeNumerically("<=", 30))
		Expect(result).To(HavePrefix(strings.Repeat("a", 30-1-hashLength)))
	})

	It("should trim specified trailing characters", func() {
		result := truncateWithHash(strings.Repeat("a", 50)+"-_.", "original", 30, "-_.")
		lastCharOfTruncated := result[len(result)-hashLength-2]
		Expect(lastCharOfTruncated).NotTo(Equal(byte('-')))
		Expect(lastCharOfTruncated).NotTo(Equal(byte('_')))
		Expect(lastCharOfTruncated).NotTo(Equal(byte('.')))
	})

	It("should hash the original input, not the truncated string", func() {
		result1 := truncateWithHash(strings.Repeat("a", 50), "original1", 30, "-")
		result2 := truncateWithHash(strings.Repeat("a", 50), "original2", 30, "-")
		Expect(result1).NotTo(Equal(result2))
	})

	It("should be deterministic", func() {
		result1 := truncateWithHash(strings.Repeat("a", 100), "test", 30, "-")
		result2 := truncateWithHash(strings.Repeat("a", 100), "test", 30, "-")
		Expect(result1).To(Equal(result2))
	})

	It("should handle empty string", func() {
		result := truncateWithHash("", "original", 30, "-")
		Expect(len(result)).To(BeNumerically("<=", 30))
		Expect(result).To(HaveLen(hashLength))
	})

	It("should handle string exactly at maxLen", func() {
		s := strings.Repeat("a", 30)
		result := truncateWithHash(s, "original", 30, "-")
		Expect(len(result)).To(BeNumerically("<=", 30))
	})

	It("should handle string at maxLen-1", func() {
		s := strings.Repeat("a", 29)
		result := truncateWithHash(s, "original", 30, "-")
		Expect(len(result)).To(BeNumerically("<=", 30))
	})

	It("should handle string with all trimChars at end", func() {
		s := strings.Repeat("a", 50) + "---"
		result := truncateWithHash(s, "original", 30, "-")
		Expect(len(result)).To(BeNumerically("<=", 30))
		// After trimming, the last 'a' should be followed by the hash separator
		Expect(result).To(HaveSuffix("-" + result[len(result)-hashLength:]))
	})
})

var _ = Describe("SanitizeSubdomain", func() {
	It("should convert repository path to valid name", func() {
		result, err := SanitizeSubdomain("owner/repo")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("owner-repo"))
	})

	It("should convert to lowercase", func() {
		result, err := SanitizeSubdomain("Owner/Repo")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("owner-repo"))
	})

	It("should handle multiple slashes", func() {
		result, err := SanitizeSubdomain("org/owner/repo")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("org-owner-repo"))
	})

	It("should handle empty string", func() {
		result, err := SanitizeSubdomain("")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(BeEmpty())
	})

	It("should handle string without slashes", func() {
		result, err := SanitizeSubdomain("repository")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("repository"))
	})

	It("should remove invalid characters", func() {
		result, err := SanitizeSubdomain("owner@repo#test")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("owner-repo-test"))
	})

	It("should trim leading dashes", func() {
		result, err := SanitizeSubdomain("-invalid-start")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("invalid-start"))
	})

	It("should trim trailing dashes", func() {
		result, err := SanitizeSubdomain("invalid-end-")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("invalid-end"))
	})

	It("should collapse consecutive hyphens", func() {
		result, err := SanitizeSubdomain("owner--repo")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("owner-repo"))
	})

	It("should handle mixed case and special characters", func() {
		result, err := SanitizeSubdomain("Owner/Repo_Name-123")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("owner-repo-name-123"))
	})

	It("should preserve dots in names", func() {
		result, err := SanitizeSubdomain("owner.repo.name")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("owner.repo.name"))
	})

	It("should handle underscores in names", func() {
		result, err := SanitizeSubdomain("owner_repo_name")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("owner-repo-name"))
	})

	It("should truncate very long names to 253 characters", func() {
		longName := "owner/" + strings.Repeat("very-long-repo-name-", 20)
		result, err := SanitizeSubdomain(longName)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(HaveLen(validation.DNS1123SubdomainMaxLength))
		Expect(result).To(HavePrefix("owner-very-long-repo-name-"))
		Expect(result[len(result)-1:]).To(MatchRegexp(`[a-z0-9]`))
	})

	It("should handle complex repository URLs", func() {
		result, err := SanitizeSubdomain("https://github.com/owner/repo.git")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("https-github.com-owner-repo.git"))
	})

	It("should return error for names with only invalid characters", func() {
		result, err := SanitizeSubdomain("!@#$%^&*()")
		Expect(err).To(HaveOccurred())
		Expect(result).To(BeEmpty())
	})

	It("should produce valid DNS-1123 subdomains", func() {
		inputs := []string{
			"owner/repo",
			"Owner/Repo",
			"org/owner/repo",
			"repository",
			"owner@repo#test",
			"-invalid-start",
			"invalid-end-",
			"owner--repo",
			"Owner/Repo_Name-123",
			"owner.repo.name",
			"owner_repo_name",
			"https://github.com/owner/repo.git",
			"owner/" + strings.Repeat("very-long-repo-name-", 20),
		}

		for _, input := range inputs {
			result, err := SanitizeSubdomain(input)
			Expect(err).ToNot(HaveOccurred())
			Expect(validation.IsDNS1123Subdomain(result)).To(BeEmpty(), "input %q produced invalid subdomain %q", input, result)
		}
	})

	It("should handle names with only dots", func() {
		result, err := SanitizeSubdomain("...")
		Expect(err).To(HaveOccurred())
		Expect(result).To(BeEmpty())
	})

	It("should handle names with consecutive dots", func() {
		result, err := SanitizeSubdomain("repo..name")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("repo.name"))
		Expect(validation.IsDNS1123Subdomain(result)).To(BeEmpty())
	})

	It("should handle names with dots at edges", func() {
		result, err := SanitizeSubdomain(".repo.")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("repo"))
		Expect(validation.IsDNS1123Subdomain(result)).To(BeEmpty())
	})

	It("should handle names with mixed dots and dashes at edges", func() {
		result, err := SanitizeSubdomain("-.repo.-")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("repo"))
		Expect(validation.IsDNS1123Subdomain(result)).To(BeEmpty())
	})

	It("should handle names with spaces", func() {
		result, err := SanitizeSubdomain("repo name")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("repo-name"))
		Expect(validation.IsDNS1123Subdomain(result)).To(BeEmpty())
	})

	It("should handle names with unicode characters", func() {
		result, err := SanitizeSubdomain("repo-名前")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("repo"))
		Expect(validation.IsDNS1123Subdomain(result)).To(BeEmpty())
	})
})

var _ = Describe("SanitizeLabel", func() {
	It("should return short names unchanged", func() {
		Expect(SanitizeLabel("my-repo")).To(Equal("my-repo"))
	})

	It("should return names at exactly 63 characters unchanged", func() {
		name := strings.Repeat("a", 63)
		Expect(SanitizeLabel(name)).To(Equal(name))
		Expect(SanitizeLabel(name)).To(HaveLen(63))
	})

	It("should truncate names exceeding 63 characters with a hash suffix", func() {
		name := strings.Repeat("a", 100)
		result := SanitizeLabel(name)
		Expect(len(result)).To(BeNumerically("<=", 63))
		Expect(result).To(HavePrefix(strings.Repeat("a", validation.DNS1035LabelMaxLength-1-hashLength)))
	})

	It("should be deterministic", func() {
		name := strings.Repeat("b", 100)
		Expect(SanitizeLabel(name)).To(Equal(SanitizeLabel(name)))
	})

	It("should produce different values for different long inputs", func() {
		name1 := strings.Repeat("a", 80) + "x"
		name2 := strings.Repeat("a", 80) + "y"
		Expect(SanitizeLabel(name1)).NotTo(Equal(SanitizeLabel(name2)))
	})

	It("should not end with a trailing hyphen before the hash", func() {
		name := strings.Repeat("a", 53) + "-" + strings.Repeat("b", 20)
		result := SanitizeLabel(name)
		Expect(len(result)).To(BeNumerically("<=", 63))
		Expect(result).NotTo(ContainSubstring("--"))
	})

	It("should trim trailing dots from short names", func() {
		result := SanitizeLabel("my-repo.")
		Expect(result).To(Equal("my-repo"))
	})

	It("should trim leading dots from short names", func() {
		result := SanitizeLabel(".my-repo")
		Expect(result).To(Equal("my-repo"))
	})

	It("should trim trailing dots from long names before truncation", func() {
		name := strings.Repeat("a", 60) + "."
		result := SanitizeLabel(name)
		Expect(len(result)).To(BeNumerically("<=", 63))
		Expect(result[len(result)-1:]).To(MatchRegexp(`[a-zA-Z0-9]`))
	})

	It("should trim leading and trailing non-alphanumeric characters", func() {
		result := SanitizeLabel("-_.repo._-")
		Expect(result).To(Equal("repo"))
	})

	It("should handle names that are entirely non-alphanumeric", func() {
		result := SanitizeLabel("---")
		Expect(result).To(BeEmpty())
	})

	It("should trim trailing underscores and hyphens from truncated names", func() {
		name := strings.Repeat("a", 50) + "-_" + strings.Repeat("b", 20)
		result := SanitizeLabel(name)
		Expect(len(result)).To(BeNumerically("<=", 63))
		Expect(result[len(result)-1:]).To(MatchRegexp(`[a-zA-Z0-9]`))
	})

	It("should produce valid DNS-1035 labels", func() {
		inputs := []string{
			"my-repo",
			strings.Repeat("a", 63),
			strings.Repeat("a", 100),
			strings.Repeat("b", 100),
			strings.Repeat("a", 80) + "x",
			strings.Repeat("a", 53) + "-" + strings.Repeat("b", 20),
			"my-repo.",
			".my-repo",
			strings.Repeat("a", 60) + ".",
			"-_.repo._-",
			strings.Repeat("a", 50) + "-_" + strings.Repeat("b", 20),
		}

		for _, input := range inputs {
			result := SanitizeLabel(input)
			if result == "" {
				continue
			}

			Expect(validation.IsDNS1035Label(result)).To(BeEmpty(), "input %q produced invalid label %q", input, result)
		}
	})

	It("should handle names with only underscores and dots", func() {
		result := SanitizeLabel("___...")
		Expect(result).To(BeEmpty())
	})

	It("should handle names with mixed separators that become empty", func() {
		result := SanitizeLabel("-_.-_.-_")
		Expect(result).To(BeEmpty())
	})

	It("should handle names with spaces", func() {
		result := SanitizeLabel("repo name")
		Expect(result).To(Equal("repo-name"))
		Expect(validation.IsDNS1035Label(result)).To(BeEmpty())
	})

	It("should handle names with unicode characters", func() {
		result := SanitizeLabel("repo-名前")
		Expect(result).To(Equal("repo"))
		Expect(validation.IsDNS1035Label(result)).To(BeEmpty())
	})

	It("should handle names with special chars in middle", func() {
		result := SanitizeLabel("repo@name#test")
		Expect(result).To(Equal("repo-name-test"))
		Expect(validation.IsDNS1035Label(result)).To(BeEmpty())
	})

	It("should handle names with multiple consecutive separators", func() {
		result := SanitizeLabel("repo--__..name")
		Expect(result).To(Equal("repo-name"))
		Expect(validation.IsDNS1035Label(result)).To(BeEmpty())
	})
})

var _ = Describe("DeterministicSubdomain", func() {
	It("should append suffix without hashing if combined length fits", func() {
		result, err := DeterministicSubdomain("my-repo", "-webhook-secret")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("my-repo-webhook-secret"))
	})

	It("should preserve dots in the base name", func() {
		result, err := DeterministicSubdomain("org.team.repo", "-secret")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("org.team.repo-secret"))
	})

	It("should hash and truncate if combined length exceeds 253 characters", func() {
		base := strings.Repeat("a", 240)
		suffix := "-webhook-secret"

		result, err := DeterministicSubdomain(base, suffix)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(result)).To(BeNumerically("<=", validation.DNS1123SubdomainMaxLength))
		Expect(result).To(HaveSuffix(suffix))
	})

	It("should produce different results for different long bases", func() {
		base1 := strings.Repeat("a", 200) + "x"
		base2 := strings.Repeat("a", 200) + "y"
		suffix := "-secret"

		result1, err1 := DeterministicSubdomain(base1, suffix)
		result2, err2 := DeterministicSubdomain(base2, suffix)

		Expect(err1).ToNot(HaveOccurred())
		Expect(err2).ToNot(HaveOccurred())
		Expect(result1).NotTo(Equal(result2))
	})

	It("should return an error if the suffix is too long to fit", func() {
		base := "valid"
		suffix := "-" + strings.Repeat("1", 250)

		result, err := DeterministicSubdomain(base, suffix)
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError(errInvalidSuffix))
		Expect(result).To(BeEmpty())
	})

	It("should return an error for entirely invalid base name", func() {
		result, err := DeterministicSubdomain("!@#$%", "-secret")
		Expect(err).To(HaveOccurred())
		Expect(result).To(BeEmpty())
	})

	It("should cleanly trim trailing hyphens from the truncated base", func() {
		base := strings.Repeat("a", 200) + "-" + strings.Repeat("b", 50)
		suffix := "-secret"

		result, err := DeterministicSubdomain(base, suffix)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).ToNot(ContainSubstring("--"))
		Expect(len(result)).To(BeNumerically("<=", validation.DNS1123SubdomainMaxLength))
	})

	It("should produce valid DNS-1123 subdomains", func() {
		inputs := []struct {
			base, suffix string
		}{
			{"my-repo", "-webhook-secret"},
			{"org.team.repo", "-secret"},
			{strings.Repeat("a", 240), "-webhook-secret"},
			{strings.Repeat("a", 200) + "x", "-secret"},
			{strings.Repeat("a", 200) + "-" + strings.Repeat("b", 50), "-secret"},
		}

		for _, input := range inputs {
			result, err := DeterministicSubdomain(input.base, input.suffix)
			Expect(err).ToNot(HaveOccurred())
			Expect(validation.IsDNS1123Subdomain(result)).To(BeEmpty(),
				"base %q suffix %q produced invalid subdomain %q", input.base, input.suffix, result)
		}
	})

	It("should handle base with only dots", func() {
		result, err := DeterministicSubdomain("...", "-secret")
		Expect(err).To(HaveOccurred())
		Expect(result).To(BeEmpty())
	})

	It("should handle combined length exactly at 253", func() {
		base := strings.Repeat("a", 240)
		suffix := strings.Repeat("b", 13)
		result, err := DeterministicSubdomain(base, suffix)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(HaveLen(253))
		Expect(validation.IsDNS1123Subdomain(result)).To(BeEmpty())
	})

	It("should handle suffix that makes it exactly 253", func() {
		base := strings.Repeat("a", 240)
		suffix := "-secret"
		result, err := DeterministicSubdomain(base, suffix)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(result)).To(BeNumerically("<=", 253))
		Expect(validation.IsDNS1123Subdomain(result)).To(BeEmpty())
	})

	It("should handle base with consecutive dots", func() {
		result, err := DeterministicSubdomain("repo..name", "-secret")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("repo.name-secret"))
		Expect(validation.IsDNS1123Subdomain(result)).To(BeEmpty())
	})
})
