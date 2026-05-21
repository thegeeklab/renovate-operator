package receiver

import "net/http"

// Receiver defines how a specific Git platform validates and parses incoming webhooks.
type Receiver interface {
	// GetAllowedUsers returns the list of users allowed to trigger Renovate runs.
	GetAllowedUsers(string, string) ([]string, error)

	// Validate checks the cryptographic signature of the webhook.
	// Returns an error if the signature is missing or invalid.
	Validate(req *http.Request, secretToken, body []byte) error

	// Parse parses the payload and headers to determine if
	// this specific event should trigger a Renovate run.
	// Returns (shouldTrigger, webhookUser, error).
	Parse(req *http.Request, body []byte) (bool, string, error)
}
