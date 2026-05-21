package gitea

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"

	"code.gitea.io/sdk/gitea"
)

var (
	ErrInvalidSignature       = errors.New("invalid webhook signature")
	ErrMissingSignature       = errors.New("missing X-Gitea-Signature header")
	ErrMissingEndpointOrToken = errors.New("missing endpoint or token for identification")
)

type Receiver struct{}

func NewReceiver() *Receiver {
	return &Receiver{}
}

//nolint:tagliatelle // Gitea API uses snake_case
type pushPayload struct {
	Ref        string `json:"ref"`
	Repository struct {
		DefaultBranch string `json:"default_branch"`
	} `json:"repository"`
}

//nolint:tagliatelle // Gitea API uses snake_case
type pullRequestPayload struct {
	PullRequest struct {
		Number      int    `json:"number"`
		State       string `json:"state"`
		Title       string `json:"title"`
		Description string `json:"body"`
		User        struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"pull_request"`
}

func (p *Receiver) GetAllowedUsers(endpoint, token string) ([]string, error) {
	if endpoint == "" || token == "" {
		return nil, ErrMissingEndpointOrToken
	}

	client, err := gitea.NewClient(endpoint, gitea.SetToken(token))
	if err != nil {
		return nil, err
	}

	user, _, err := client.GetMyUserInfo()
	if err != nil {
		return nil, err
	}

	return []string{user.UserName}, nil
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

	if !verifyRenovateDescriptionChange(payload.PullRequest.Description) {
		return false, "", nil
	}

	return true, payload.PullRequest.User.Login, nil
}
