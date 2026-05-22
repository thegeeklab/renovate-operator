package gitea

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thegeeklab/renovate-operator/internal/receiver"
	"github.com/thegeeklab/renovate-operator/internal/receiver/gitea/fixtures"
)

func generateValidSignature(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)

	return hex.EncodeToString(mac.Sum(nil))
}

var _ = Describe("Gitea Webhook Receiver", func() {
	var (
		Receiver *Receiver
		secret   []byte
	)

	BeforeEach(func() {
		Receiver = NewReceiver()
		secret = []byte("test-secret-token")
	})

	Describe("Validate", func() {
		It("should pass with a valid HMAC signature", func() {
			body := []byte(`{"hello": "world"}`)
			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Signature", generateValidSignature(secret, body))

			err := Receiver.Validate(req, secret, body)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail if the signature is missing", func() {
			body := []byte(`{"hello": "world"}`)
			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))

			err := Receiver.Validate(req, secret, body)
			Expect(err).To(MatchError(ErrMissingSignature))
		})

		It("should fail if the signature is invalid", func() {
			body := []byte(`{"hello": "world"}`)
			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Signature", "invalid-hacker-signature")

			err := Receiver.Validate(req, secret, body)
			Expect(err).To(MatchError(ErrInvalidSignature))
		})
	})

	Describe("Parse", func() {
		It("should trigger a run for a valid push on the default branch", func() {
			body := []byte(fixtures.HookPush)

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "push")

			result, err := Receiver.Parse(req, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(receiver.ParseResult{ShouldTrigger: true}))
		})

		It("should NOT trigger a run for a push to a non-default branch", func() {
			body := []byte(fixtures.HookPushBranch)

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "push")

			result, err := Receiver.Parse(req, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(receiver.ParseResult{}))
		})

		It("should NOT trigger a run for a tag event", func() {
			body := []byte(fixtures.HookTag)

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "push")

			result, err := Receiver.Parse(req, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(receiver.ParseResult{}))
		})

		It("should NOT trigger a run for a non-push event type", func() {
			body := []byte(fixtures.HookPush)

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "issue")

			result, err := Receiver.Parse(req, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(receiver.ParseResult{}))
		})

		It("should return an error if the JSON payload is malformed", func() {
			body := []byte(`{this-is-not-valid-json`)

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "push")

			result, err := Receiver.Parse(req, body)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid character"))
			Expect(result.ShouldTrigger).To(BeFalse())
		})

		It("should return an error if the body is empty", func() {
			body := []byte("")

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "push")

			result, err := Receiver.Parse(req, body)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unexpected end of JSON input"))
			Expect(result.ShouldTrigger).To(BeFalse())
		})

		It("should trigger a run for renovate PR with checked checkbox", func() {
			body := []byte(fixtures.HookPullRequestRenovateChecked)

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "pull_request")

			result, err := Receiver.Parse(req, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.ShouldTrigger).To(BeTrue())
			Expect(result.RequireUserCheck).To(BeTrue())
			Expect(result.User).NotTo(BeEmpty())
		})

		It("should NOT trigger a run for renovate PR with unchecked checkboxes", func() {
			body := []byte(fixtures.HookPullRequestRenovateUnchecked)

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "pull_request")

			result, err := Receiver.Parse(req, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(receiver.ParseResult{}))
		})

		It("should NOT trigger a run for regular PR without renovate markers", func() {
			body := []byte(fixtures.HookPullRequestRegular)

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "pull_request")

			result, err := Receiver.Parse(req, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(receiver.ParseResult{}))
		})

		It("should NOT trigger a run for renovate PR with unchecked rebase checkbox", func() {
			body := []byte(fixtures.HookPullRequestRenovateRebase)

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "pull_request")

			result, err := Receiver.Parse(req, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(receiver.ParseResult{}))
		})

		It("should trigger a run for a Renovate dependency dashboard issue with a checked checkbox", func() {
			body := []byte(fixtures.HookIssueRenovateChecked)

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "issues")

			result, err := Receiver.Parse(req, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.ShouldTrigger).To(BeTrue())
			Expect(result.RequireUserCheck).To(BeTrue())
			Expect(result.User).NotTo(BeEmpty())
		})

		It("should NOT trigger a run for a Renovate dependency dashboard issue with unchecked checkboxes", func() {
			body := []byte(fixtures.HookIssueRenovateUnchecked)

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "issues")

			result, err := Receiver.Parse(req, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(receiver.ParseResult{}))
		})

		It("should NOT trigger a run for pull_request with action 'opened'", func() {
			body := []byte(fixtures.HookPullRequestRenovateOpened)

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "pull_request")

			result, err := Receiver.Parse(req, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(receiver.ParseResult{}))
		})

		It("should NOT trigger a run for pull_request with action 'closed'", func() {
			body := []byte(fixtures.HookPullRequestRenovateClosed)

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "pull_request")

			result, err := Receiver.Parse(req, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(receiver.ParseResult{}))
		})

		It("should NOT trigger a run for issues with action 'opened'", func() {
			body := []byte(fixtures.HookIssueRenovateOpened)

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "issues")

			result, err := Receiver.Parse(req, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(receiver.ParseResult{}))
		})

		It("should NOT trigger a run for issues with action 'closed'", func() {
			body := []byte(fixtures.HookIssueRenovateClosed)

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "issues")

			result, err := Receiver.Parse(req, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(receiver.ParseResult{}))
		})
	})
})
