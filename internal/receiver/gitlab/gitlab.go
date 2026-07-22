package gitlab

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/thegeeklab/renovate-operator/internal/receiver"
)

var (
	ErrInvalidToken = errors.New("invalid X-Gitlab-Token header")
	ErrMissingToken = errors.New("missing X-Gitlab-Token header")
)

// Receiver validates and parses classic GitLab project webhooks.
type Receiver struct{}

var _ receiver.Receiver = (*Receiver)(nil)

// NewReceiver creates a GitLab webhook receiver.
func NewReceiver() *Receiver {
	return &Receiver{}
}

func (p *Receiver) Validate(req *http.Request, secretToken, body []byte) error {
	token := req.Header.Get("X-Gitlab-Token")
	if token == "" {
		return ErrMissingToken
	}

	if subtle.ConstantTimeCompare([]byte(token), secretToken) != 1 {
		return ErrInvalidToken
	}

	return nil
}

func (p *Receiver) Parse(req *http.Request, body []byte) (receiver.ParseResult, error) {
	switch req.Header.Get("X-Gitlab-Event") {
	case "Push Hook":
		return p.parsePushEvent(body)
	case "Merge Request Hook", "Issue Hook":
		return p.parseEditableEvent(body)
	default:
		return receiver.ParseResult{}, nil
	}
}

//nolint:tagliatelle // GitLab API uses snake_case.
type pushPayload struct {
	Ref     string `json:"ref"`
	Project struct {
		DefaultBranch string `json:"default_branch"`
	} `json:"project"`
}

func (p *Receiver) parsePushEvent(body []byte) (receiver.ParseResult, error) {
	payload := &pushPayload{}
	if err := json.Unmarshal(body, payload); err != nil {
		return receiver.ParseResult{}, err
	}

	return receiver.ParseResult{
		ShouldTrigger: payload.Ref == "refs/heads/"+payload.Project.DefaultBranch,
	}, nil
}

//nolint:tagliatelle // GitLab API uses snake_case.
type editablePayload struct {
	ObjectAttributes struct {
		Action      string `json:"action"`
		Description string `json:"description"`
	} `json:"object_attributes"`
	User struct {
		Username string `json:"username"`
	} `json:"user"`
}

func (p *Receiver) parseEditableEvent(body []byte) (receiver.ParseResult, error) {
	payload := &editablePayload{}
	if err := json.Unmarshal(body, payload); err != nil {
		return receiver.ParseResult{}, err
	}

	if payload.ObjectAttributes.Action != "update" ||
		!receiver.IsRenovateCheckboxChecked(payload.ObjectAttributes.Description) {
		return receiver.ParseResult{}, nil
	}

	return receiver.ParseResult{
		ShouldTrigger:    true,
		RequireUserCheck: true,
		User:             payload.User.Username,
	}, nil
}
