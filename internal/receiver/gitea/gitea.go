package gitea

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
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
type pushPayload struct {
	Ref        string `json:"ref"`
	Repository struct {
		DefaultBranch string `json:"default_branch"`
	} `json:"repository"`
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

func (p *Receiver) Parse(req *http.Request, body []byte) (bool, error) {
	event := req.Header.Get("X-Gitea-Event")
	if event != "push" {
		return false, nil
	}

	var payload pushPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return false, err
	}

	expectedRef := "refs/heads/" + payload.Repository.DefaultBranch
	if payload.Ref != expectedRef {
		return false, nil
	}

	return true, nil
}
