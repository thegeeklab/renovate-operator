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

			shouldTrigger, err := Receiver.Parse(req, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(shouldTrigger).To(BeTrue())
		})

		It("should NOT trigger a run for a push to a non-default branch", func() {
			body := []byte(fixtures.HookPushBranch)

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "push")

			shouldTrigger, err := Receiver.Parse(req, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(shouldTrigger).To(BeFalse())
		})

		It("should NOT trigger a run for a tag event", func() {
			body := []byte(fixtures.HookTag)

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "push")

			shouldTrigger, err := Receiver.Parse(req, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(shouldTrigger).To(BeFalse())
		})

		It("should NOT trigger a run for a non-push event type", func() {
			body := []byte(fixtures.HookPush)

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "issue")

			shouldTrigger, err := Receiver.Parse(req, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(shouldTrigger).To(BeFalse())
		})

		It("should return an error if the JSON payload is malformed", func() {
			body := []byte(`{this-is-not-valid-json`)

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "push")

			shouldTrigger, err := Receiver.Parse(req, body)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid character"))
			Expect(shouldTrigger).To(BeFalse())
		})

		It("should return an error if the body is empty", func() {
			body := []byte("")

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "push")

			shouldTrigger, err := Receiver.Parse(req, body)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unexpected end of JSON input"))
			Expect(shouldTrigger).To(BeFalse())
		})
	})
})
