// Package viewmodel contains the data contracts shared between the backend
// (data factory, API, web handlers) and the templ-based view layer. The types
// in this package are persistence-agnostic: they carry the data a view needs
// to render but are not tied to Kubernetes resource definitions or JSON
// serialization tags.
package viewmodel

import "time"

// Status represents the lifecycle state of a renovate run.
type Status string

const (
	StatusUnknown   Status = "Unknown"
	StatusSucceeded Status = "Succeeded"
	StatusRunning   Status = "Running"
	StatusFailed    Status = "Failed"
)

// Label returns the human-readable label for the status.
func (s Status) Label() string {
	switch s {
	case StatusSucceeded, StatusRunning, StatusFailed:
		return string(s)
	default:
		return string(StatusUnknown)
	}
}

// BadgeClass returns the Tailwind class set used to render the status as a
// rounded pill (badge).
func (s Status) BadgeClass() string {
	const base = "inline-flex items-center rounded-full px-2 py-1 text-xs font-medium ring-1 ring-inset "

	switch s {
	case StatusSucceeded:
		return base + "bg-green-50 text-green-700 ring-green-600/20"
	case StatusRunning:
		return base + "bg-blue-50 text-blue-700 ring-blue-600/20"
	case StatusFailed:
		return base + "bg-red-50 text-red-700 ring-red-600/10"
	default:
		return base + "bg-gray-50 text-gray-600 ring-gray-500/10"
	}
}

// LeftBorderClass returns the Tailwind border-color class for the left edge
// of a list item that conveys the status of a Renovator/GitRepo/Job.
func (s Status) LeftBorderClass() string {
	switch s {
	case StatusSucceeded:
		return "border-l-green-500"
	case StatusRunning:
		return "border-l-blue-500"
	case StatusFailed:
		return "border-l-red-500"
	default:
		return "border-l-gray-300"
	}
}

// AuthInfo describes the current authentication state for a request and is
// rendered into the layout shell so the navbar can show the user and
// optionally the logout form.
type AuthInfo struct {
	Enabled       bool
	Authenticated bool
	Name          string
	Provider      string
	Providers     []AuthProviderInfo
	CSRFToken     string
}

// AuthProviderInfo describes a single OAuth/OIDC provider that the user can
// sign in with on the login page.
type AuthProviderInfo struct {
	Name        string
	DisplayName string
	IconURL     string
}

// GitRepoFieldLabel defines the human-readable label for a repository field
// as shown on repository cards. These are the single source of truth for
// field names used in both card display and the sort dropdown.
type GitRepoFieldLabel string

const (
	// GitRepoFieldName is the display label for the repository name field.
	GitRepoFieldName GitRepoFieldLabel = "Name"
	// GitRepoFieldCreated is the display label for the creation date field.
	GitRepoFieldCreated GitRepoFieldLabel = "Created"
	// GitRepoFieldLastRun is the display label for the last run date field.
	GitRepoFieldLastRun GitRepoFieldLabel = "Last run"
)

// WebView is the summary card for a single Renovator in the dashboard
// accordion list.
type WebView struct {
	Name          string
	Namespace     string
	Renovator     string
	GitRepoCount  int
	RunnerName    string
	DiscoveryName string
}

// GitRepoInfo is the view-layer representation of a GitRepo.
type GitRepoInfo struct {
	Name               string    `json:"name"`
	FullName           string    `json:"fullName"`
	Namespace          string    `json:"namespace"`
	WebhookID          string    `json:"webhookId"`
	LastRenovateAt     time.Time `json:"lastRenovateAt"`
	LastRenovateStatus Status    `json:"lastRenovateStatus"`
	CreatedAt          time.Time `json:"createdAt"`
	RenovatorUID       string    `json:"renovatorUid"`
}

// JobInfo is the view-layer representation of a Kubernetes Job.
type JobInfo struct {
	Name      string    `json:"name"`
	Namespace string    `json:"namespace"`
	Runner    string    `json:"runner"`
	Status    Status    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

// GitRepoViewData bundles a single GitRepo with its associated jobs for the
// gitrepo detail view.
type GitRepoViewData struct {
	Repo GitRepoInfo
	Jobs []JobInfo
}

// DashboardData carries either the renovator list or search results for the
// dashboard template. Exactly one of Renovators or SearchResults is populated.
type DashboardData struct {
	SearchQuery   string
	Renovators    []WebView
	SearchResults []GitRepoInfo
}

// JobLogData is the payload rendered by the JobLogs template, including the
// log content and metadata describing whether the job is still running.
type JobLogData struct {
	JobName   string
	Namespace string
	Runner    string
	IsRunning bool
	Content   string
	Message   string
}
