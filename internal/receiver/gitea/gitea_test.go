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

			shouldTrigger, _, err := Receiver.Parse(req, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(shouldTrigger).To(BeTrue())
		})

		It("should NOT trigger a run for a push to a non-default branch", func() {
			body := []byte(fixtures.HookPushBranch)

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "push")

			shouldTrigger, _, err := Receiver.Parse(req, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(shouldTrigger).To(BeFalse())
		})

		It("should NOT trigger a run for a tag event", func() {
			body := []byte(fixtures.HookTag)

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "push")

			shouldTrigger, _, err := Receiver.Parse(req, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(shouldTrigger).To(BeFalse())
		})

		It("should NOT trigger a run for a non-push event type", func() {
			body := []byte(fixtures.HookPush)

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "issue")

			shouldTrigger, _, err := Receiver.Parse(req, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(shouldTrigger).To(BeFalse())
		})

		It("should return an error if the JSON payload is malformed", func() {
			body := []byte(`{this-is-not-valid-json`)

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "push")

			shouldTrigger, _, err := Receiver.Parse(req, body)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid character"))
			Expect(shouldTrigger).To(BeFalse())
		})

		It("should return an error if the body is empty", func() {
			body := []byte("")

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "push")

			shouldTrigger, _, err := Receiver.Parse(req, body)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unexpected end of JSON input"))
			Expect(shouldTrigger).To(BeFalse())
		})

		It("should trigger a run for renovate PR with checked checkbox", func() {
			body := []byte(fixtures.HookPullRequestRenovateChecked)

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "pull_request")

			shouldTrigger, _, err := Receiver.Parse(req, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(shouldTrigger).To(BeTrue())
		})

		It("should NOT trigger a run for renovate PR with unchecked checkboxes", func() {
			body := []byte(fixtures.HookPullRequestRenovateUnchecked)

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "pull_request")

			shouldTrigger, _, err := Receiver.Parse(req, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(shouldTrigger).To(BeFalse())
		})

		It("should NOT trigger a run for regular PR without renovate markers", func() {
			body := []byte(fixtures.HookPullRequestRegular)

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "pull_request")

			shouldTrigger, _, err := Receiver.Parse(req, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(shouldTrigger).To(BeFalse())
		})

		It("should NOT trigger a run for renovate PR with unchecked rebase checkbox", func() {
			body := []byte(fixtures.HookPullRequestRenovateRebase)

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			req.Header.Set("X-Gitea-Event", "pull_request")

			shouldTrigger, _, err := Receiver.Parse(req, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(shouldTrigger).To(BeFalse())
		})
	})

	Describe("isRenovateContent", func() {
		It("should return true for content with ## Detected Dependencies", func() {
			Expect(isRenovateContent("## Detected Dependencies")).To(BeTrue())
		})

		It("should return true for content with <!-- rebase-check -->", func() {
			Expect(isRenovateContent("<!-- rebase-check -->")).To(BeTrue())
		})

		It("should return true for content with <!--renovate-debug:-->", func() {
			Expect(isRenovateContent("<!--renovate-debug:test-->")).To(BeTrue())
		})

		It("should return true for content with <!-- rebase-all-open-prs -->", func() {
			Expect(isRenovateContent("<!-- rebase-all-open-prs -->")).To(BeTrue())
		})

		It("should return true for content with <!-- rebase-branch=", func() {
			Expect(isRenovateContent("<!-- rebase-branch=main -->")).To(BeTrue())
		})

		It("should return true for content with <!-- approve-all-pending-prs -->", func() {
			Expect(isRenovateContent("<!-- approve-all-pending-prs -->")).To(BeTrue())
		})

		It("should return true for content with <!-- approvePr-branch=", func() {
			Expect(isRenovateContent("<!-- approvePr-branch=main -->")).To(BeTrue())
		})

		It("should return true for content with <!-- approve-branch=", func() {
			Expect(isRenovateContent("<!-- approve-branch=main -->")).To(BeTrue())
		})

		It("should return true for content with <!-- recreate-branch=", func() {
			Expect(isRenovateContent("<!-- recreate-branch=main -->")).To(BeTrue())
		})

		It("should return true for content with <!-- unschedule-branch=", func() {
			Expect(isRenovateContent("<!-- unschedule-branch=main -->")).To(BeTrue())
		})

		It("should return true for content with <!-- create-config-migration-pr -->", func() {
			Expect(isRenovateContent("<!-- create-config-migration-pr -->")).To(BeTrue())
		})

		It("should return true for content with <!-- create-all-awaiting-schedule-prs -->", func() {
			Expect(isRenovateContent("<!-- create-all-awaiting-schedule-prs -->")).To(BeTrue())
		})

		It("should return true for content with <!-- create-all-rate-limited-prs -->", func() {
			Expect(isRenovateContent("<!-- create-all-rate-limited-prs -->")).To(BeTrue())
		})

		It("should return true for content with <!-- unlimit-branch=", func() {
			Expect(isRenovateContent("<!-- unlimit-branch=main -->")).To(BeTrue())
		})

		It("should return true for content with <!-- manual job -->", func() {
			Expect(isRenovateContent("<!-- manual job -->")).To(BeTrue())
		})

		It("should return false for empty content", func() {
			Expect(isRenovateContent("")).To(BeFalse())
		})

		It("should return false for content without renovate markers", func() {
			Expect(isRenovateContent("This is a regular PR description")).To(BeFalse())
		})
	})

	Describe("hasCheckboxBeenChecked", func() {
		It("should return true for content with lowercase - [x]", func() {
			Expect(hasCheckboxBeenChecked("- [x] golangci/golangci-lint")).To(BeTrue())
		})

		It("should return true for content with uppercase - [X]", func() {
			Expect(hasCheckboxBeenChecked("- [X] golangci/golangci-lint")).To(BeTrue())
		})

		It("should return false for content with unchecked - [ ]", func() {
			Expect(hasCheckboxBeenChecked("- [ ] golangci/golangci-lint")).To(BeFalse())
		})

		It("should return false for empty content", func() {
			Expect(hasCheckboxBeenChecked("")).To(BeFalse())
		})
	})

	Describe("verifyRenovateDescriptionChange", func() {
		It("should return true for renovate content with checked checkbox", func() {
			content := "## Detected Dependencies\r\n\r\n- [x] golangci/golangci-lint"
			Expect(verifyRenovateDescriptionChange(content)).To(BeTrue())
		})

		It("should return false for renovate content without checked checkbox", func() {
			content := "## Detected Dependencies\r\n\r\n- [ ] golangci/golangci-lint"
			Expect(verifyRenovateDescriptionChange(content)).To(BeFalse())
		})

		It("should return false for regular content with checked checkbox", func() {
			content := "Regular PR body\r\n\r\n- [x] some-task"
			Expect(verifyRenovateDescriptionChange(content)).To(BeFalse())
		})

		It("should return false for empty content", func() {
			Expect(verifyRenovateDescriptionChange("")).To(BeFalse())
		})
	})
})
