package parse

// Result holds the outcome of parsing a webhook payload.
type Result struct {
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
