package main

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	gitlabreceiver "github.com/thegeeklab/renovate-operator/internal/receiver/gitlab"
)

func TestMainPackage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Manager Package Suite")
}

var _ = Describe("buildReceiverFactory", func() {
	It("creates a GitLab receiver", func() {
		platformReceiver := buildReceiverFactory()(renovatev1beta1.PlatformType_GITLAB)
		Expect(platformReceiver).To(BeAssignableToTypeOf(&gitlabreceiver.Receiver{}))
	})

	It("returns nil for unsupported platforms", func() {
		Expect(buildReceiverFactory()(renovatev1beta1.PlatformType("unsupported"))).To(BeNil())
	})
})
