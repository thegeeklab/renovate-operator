package views

import "time"

type AuthInfo struct {
	Enabled       bool
	Authenticated bool
	Email         string
	Name          string
	Provider      string
	Providers     []AuthProviderInfo
}

type AuthProviderInfo struct {
	Name string
	Type string
}

type WebView struct {
	Name          string
	Namespace     string
	Renovator     string
	GitRepoCount  int
	RunnerName    string
	DiscoveryName string
}

type GitRepoInfo struct {
	Name               string
	FullName           string
	Namespace          string
	WebhookID          string
	LastRenovateAt     time.Time
	LastRenovateStatus string
	CreatedAt          time.Time
}

type JobInfo struct {
	Name      string
	Namespace string
	Runner    string
	Status    string
	CreatedAt time.Time
}

type GitRepoViewData struct {
	Repo GitRepoInfo
	Jobs []JobInfo
}

type JobLogData struct {
	JobName   string
	Namespace string
	Runner    string
	IsRunning bool
	Content   string
	Error     string
}
