package gitea

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func generateValidSignature(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)

	return hex.EncodeToString(mac.Sum(nil))
}

var _ = Describe("Gitea Webhook Processor", func() {
	var (
		processor *Processor
		secret    []byte
	)

	BeforeEach(func() {
		processor = NewProcessor()
		secret = []byte("test-secret-token")
	})

	Describe("Validate", func() {
		It("should pass with a valid HMAC signature", func() {
			body := []byte(`{"hello": "world"}`)
			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Signature", generateValidSignature(secret, body))

			err := processor.Validate(req, secret, body)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail if the signature is invalid", func() {
			body := []byte(`{"hello": "world"}`)
			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Signature", "invalid-hacker-signature")

			err := processor.Validate(req, secret, body)
			Expect(err).To(MatchError(ErrInvalidSignature))
		})
	})

	Describe("Parse", func() {
		It("should trigger a run for a valid push on the default branch", func() {
			body, err := os.ReadFile("testdata/push_default.json")
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "push")

			shouldTrigger, err := processor.Parse(req, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(shouldTrigger).To(BeTrue())
		})
	})
})
