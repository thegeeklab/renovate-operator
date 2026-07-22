package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thegeeklab/renovate-operator/internal/receiver"
)

const renovateDescription = "## Detected Dependencies\n\n- [x] update dependency"

var _ = Describe("GitLab Webhook Receiver", func() {
	var gitlabReceiver *Receiver

	BeforeEach(func() {
		gitlabReceiver = NewReceiver()
	})

	Describe("Validate", func() {
		It("accepts a matching classic webhook token regardless of the body", func() {
			req := httptest.NewRequest(http.MethodPost, "/hook", bytes.NewBufferString("first body"))
			req.Header.Set("X-Gitlab-Token", "secret")

			Expect(gitlabReceiver.Validate(req, []byte("secret"), []byte("different body"))).To(Succeed())
		})

		It("rejects a missing token", func() {
			req := httptest.NewRequest(http.MethodPost, "/hook", nil)
			Expect(gitlabReceiver.Validate(req, []byte("secret"), nil)).To(MatchError(ErrMissingToken))
		})

		It("rejects an invalid token", func() {
			req := httptest.NewRequest(http.MethodPost, "/hook", nil)
			req.Header.Set("X-Gitlab-Token", "wrong")

			Expect(gitlabReceiver.Validate(req, []byte("secret"), nil)).To(MatchError(ErrInvalidToken))
		})
	})

	DescribeTable(
		"parses push hooks",
		func(ref string, expected receiver.ParseResult) {
			body := []byte(`{"ref":"` + ref + `","project":{"default_branch":"main"}}`)
			req := webhookRequest("Push Hook")

			result, err := gitlabReceiver.Parse(req, body)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(expected))
		},
		Entry("default branch", "refs/heads/main", receiver.ParseResult{ShouldTrigger: true}),
		Entry("non-default branch", "refs/heads/feature", receiver.ParseResult{}),
		Entry("tag", "refs/tags/v1.0.0", receiver.ParseResult{}),
	)

	DescribeTable(
		"parses editable hooks",
		func(event, action, description string, expected receiver.ParseResult) {
			body := []byte(`{"object_attributes":{"action":"` + action + `","description":` +
				jsonString(description) + `},"user":{"username":"renovate-bot"}}`)

			result, err := gitlabReceiver.Parse(webhookRequest(event), body)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(expected))
		},
		Entry(
			"updated merge request with checked checkbox",
			"Merge Request Hook",
			"update",
			renovateDescription,
			receiver.ParseResult{ShouldTrigger: true, RequireUserCheck: true, User: "renovate-bot"},
		),
		Entry("opened merge request", "Merge Request Hook", "open", renovateDescription, receiver.ParseResult{}),
		Entry("closed merge request", "Merge Request Hook", "close", renovateDescription, receiver.ParseResult{}),
		Entry(
			"unchecked merge request",
			"Merge Request Hook",
			"update",
			"## Detected Dependencies\n- [ ] update",
			receiver.ParseResult{},
		),
		Entry(
			"updated issue with checked checkbox",
			"Issue Hook",
			"update",
			renovateDescription,
			receiver.ParseResult{ShouldTrigger: true, RequireUserCheck: true, User: "renovate-bot"},
		),
		Entry("opened issue", "Issue Hook", "open", renovateDescription, receiver.ParseResult{}),
		Entry("closed issue", "Issue Hook", "close", renovateDescription, receiver.ParseResult{}),
		Entry("unchecked issue", "Issue Hook", "update", "## Detected Dependencies\n- [ ] update", receiver.ParseResult{}),
	)

	It("accepts an unknown event without triggering", func() {
		result, err := gitlabReceiver.Parse(webhookRequest("Pipeline Hook"), []byte(`not-json`))
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(receiver.ParseResult{}))
	})

	It("returns an error for malformed supported events", func() {
		result, err := gitlabReceiver.Parse(webhookRequest("Push Hook"), []byte(`not-json`))
		Expect(err).To(HaveOccurred())
		Expect(result).To(Equal(receiver.ParseResult{}))
	})
})

func webhookRequest(event string) *http.Request {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/hook", nil)
	req.Header.Set("X-Gitlab-Event", event)

	return req
}

func jsonString(value string) string {
	quoted, err := json.Marshal(value)
	Expect(err).NotTo(HaveOccurred())

	return string(quoted)
}
