package discovery

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDiscovery(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Discovery Suite")
}

var _ = Describe("parseEnv", func() {
	BeforeEach(func() {
		os.Clearenv()
	})

	It("should return environment variable value when set", func() {
		os.Setenv("TEST_VAR", "test-value")
		value, err := parseEnv("TEST_VAR")
		Expect(err).NotTo(HaveOccurred())
		Expect(value).To(Equal("test-value"))
	})

	It("should return error when environment variable is not set", func() {
		value, err := parseEnv("NONEXISTENT_VAR")
		Expect(err).To(MatchError(ErrEnvVarNotDefined))
		Expect(value).To(BeEmpty())
	})

	It("should handle empty environment variable value", func() {
		os.Setenv("EMPTY_VAR", "")
		value, err := parseEnv("EMPTY_VAR")
		Expect(err).NotTo(HaveOccurred())
		Expect(value).To(BeEmpty())
	})
})
