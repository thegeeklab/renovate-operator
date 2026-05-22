package receiver

import "net/http"

// ParseResult holds the outcome of parsing a webhook payload.
type ParseResult struct {
	// ShouldTrigger indicates whether this event should trigger a Renovate run.
	ShouldTrigger bool
	// RequireUserCheck indicates that the triggering user must be verified
	// against the platform's bot identity before the run is allowed.
	// When false the event originates from a trusted source (e.g. a push)
	// and no user check is needed.
	RequireUserCheck bool
	// User is the login of the user who triggered the event.
	// Only meaningful when RequireUserCheck is true.
	User string
}

// Receiver defines how a specific Git platform validates and parses incoming webhooks.
type Receiver interface {
	// Validate checks the cryptographic signature of the webhook.
	// Returns an error if the signature is missing or invalid.
	Validate(req *http.Request, secretToken, body []byte) error

	// Parse parses the payload and headers to determine if
	// this specific event should trigger a Renovate run.
	// Implementations must set RequireUserCheck to true whenever the trigger
	// originates from a user-editable payload (e.g. a pull_request edit), so
	// the server enforces bot-identity verification before allowing the run.
	Parse(req *http.Request, body []byte) (ParseResult, error)
}
