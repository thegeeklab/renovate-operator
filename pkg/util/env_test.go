package util

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("parseEnv", func() {
	BeforeEach(func() {
		os.Clearenv()
	})

	It("should return environment variable value when set", func() {
		os.Setenv("TEST_VAR", "test-value")

		value, err := ParseEnv("TEST_VAR")
		Expect(err).NotTo(HaveOccurred())
		Expect(value).To(Equal("test-value"))
	})

	It("should return error when environment variable is not set", func() {
		value, err := ParseEnv("NONEXISTENT_VAR")
		Expect(err).To(MatchError(ErrEnvVarNotDefined))
		Expect(value).To(BeEmpty())
	})

	It("should handle empty environment variable value", func() {
		os.Setenv("EMPTY_VAR", "")

		value, err := ParseEnv("EMPTY_VAR")
		Expect(err).NotTo(HaveOccurred())
		Expect(value).To(BeEmpty())
	})
})
