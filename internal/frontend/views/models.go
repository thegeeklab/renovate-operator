package views

import "time"

type WebView struct {
	Name          string
	Namespace     string
	Renovator     string
	GitRepoCount  int
	RunnerName    string
	DiscoveryName string
}

type GitRepoInfo struct {
	Name      string
	Namespace string
	WebhookID string
	Ready     bool
	CreatedAt time.Time
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
