package factory_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thegeeklab/renovate-operator/internal/provider/factory"
)

func TestProviderFactory(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Provider Factory Suite")
}

var _ = Describe("DefaultProviderFactory", func() {
	It("creates a GitLab provider", func() {
		providerManager, err := factory.DefaultProviderFactory(context.Background(), factory.PlatformConfig{
			Type:     "gitlab",
			Endpoint: "https://gitlab.example.com",
			Token:    "test-token",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(providerManager).NotTo(BeNil())
	})

	It("rejects unsupported providers", func() {
		providerManager, err := factory.DefaultProviderFactory(context.Background(), factory.PlatformConfig{
			Type: "unsupported",
		})
		Expect(providerManager).To(BeNil())
		Expect(err).To(MatchError(factory.ErrNotImplemented))
	})
})
