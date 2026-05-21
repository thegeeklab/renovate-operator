package gitea

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/thegeeklab/renovate-operator/internal/receiver/renovate"
)

var (
	ErrInvalidSignature = errors.New("invalid webhook signature")
	ErrMissingSignature = errors.New("missing X-Gitea-Signature header")
)

type Receiver struct{}

func NewReceiver() *Receiver {
	return &Receiver{}
}

//nolint:tagliatelle // Gitea API uses snake_case
type pushRepository struct {
	DefaultBranch string `json:"default_branch"`
}

type pushPayload struct {
	Ref        string         `json:"ref"`
	Repository pushRepository `json:"repository"`
}

type pullRequestUser struct {
	Login string `json:"login"`
}

type pullRequest struct {
	Description string          `json:"body"`
	User        pullRequestUser `json:"user"`
}

//nolint:tagliatelle // Gitea API uses snake_case
type pullRequestPayload struct {
	Action      string      `json:"action"`
	PullRequest pullRequest `json:"pull_request"`
}

func (p *Receiver) Validate(req *http.Request, secretToken, body []byte) error {
	signature := req.Header.Get("X-Gitea-Signature")
	if signature == "" {
		return ErrMissingSignature
	}

	mac := hmac.New(sha256.New, secretToken)
	mac.Write(body)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expectedMAC), []byte(signature)) {
		return ErrInvalidSignature
	}

	return nil
}

func (p *Receiver) Parse(req *http.Request, body []byte) (bool, string, error) {
	event := req.Header.Get("X-Gitea-Event")

	switch event {
	case "push":
		return p.parsePushEvent(body)
	case "pull_request":
		return p.parsePullRequestEvent(body)
	default:
		return false, "", nil
	}
}

func (p *Receiver) parsePushEvent(body []byte) (bool, string, error) {
	var payload pushPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return false, "", err
	}

	expectedRef := "refs/heads/" + payload.Repository.DefaultBranch

	return payload.Ref == expectedRef, "", nil
}

func (p *Receiver) parsePullRequestEvent(body []byte) (bool, string, error) {
	var payload pullRequestPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return false, "", err
	}

	if payload.Action != "edited" {
		return false, "", nil
	}

	if !renovate.VerifyRenovateDescriptionChange(payload.PullRequest.Description) {
		return false, "", nil
	}

	return true, payload.PullRequest.User.Login, nil
}
