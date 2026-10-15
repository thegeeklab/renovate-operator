package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	gitlabreceiver "github.com/thegeeklab/renovate-operator/internal/receiver/gitlab"
)

var errTestNotReady = errors.New("test not ready")

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

var _ = Describe("allReadyzChecks", func() {
	It("reports ready when there are no checks", func() {
		req := httptest.NewRequest(http.MethodGet, "/readyz", nil)

		Expect(allReadyzChecks()(req)).To(Succeed())
	})

	It("reports ready when all checks pass", func() {
		req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		check := allReadyzChecks(
			func(*http.Request) error { return nil },
			func(*http.Request) error { return nil },
		)

		Expect(check(req)).To(Succeed())
	})

	It("reports not ready when any check fails", func() {
		req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		check := allReadyzChecks(
			func(*http.Request) error { return nil },
			func(*http.Request) error { return errTestNotReady },
		)

		Expect(check(req)).To(MatchError(errTestNotReady))
	})
})
