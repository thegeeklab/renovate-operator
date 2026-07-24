package fixtures

import _ "embed"

// HookPush is a sample GitHub push hook to the default branch.
//
//go:embed HookPush.json
var HookPush string

// HookPushBranch is a sample GitHub push hook where a new branch was created.
//
//go:embed HookPushBranch.json
var HookPushBranch string

// HookTag is a sample GitHub tag hook.
//
//go:embed HookTag.json
var HookTag string

// HookPullRequestRenovateChecked is a sample GitHub pull_request hook with renovate content and checked checkboxes.
//
//go:embed HookPullRequestRenovateChecked.json
var HookPullRequestRenovateChecked string

// HookPullRequestRenovateUnchecked is a sample GitHub pull_request hook with renovate content but unchecked checkboxes.
//
//go:embed HookPullRequestRenovateUnchecked.json
var HookPullRequestRenovateUnchecked string

// HookPullRequestRegular is a sample GitHub pull_request hook with regular PR content (no renovate markers).
//
//go:embed HookPullRequestRegular.json
var HookPullRequestRegular string

// HookPullRequestRenovateRebase is a sample GitHub pull_request hook with renovate rebase checkbox (unchecked).
//
//go:embed HookPullRequestRenovateRebase.json
var HookPullRequestRenovateRebase string

// HookIssueRenovateChecked is a sample GitHub issues hook for the Renovate dependency
// dashboard with a checked checkbox.
//
//go:embed HookIssueRenovateChecked.json
var HookIssueRenovateChecked string

// HookIssueRenovateUnchecked is a sample GitHub issues hook for the Renovate dependency dashboard
// with unchecked checkboxes.
//
//go:embed HookIssueRenovateUnchecked.json
var HookIssueRenovateUnchecked string

// HookPullRequestRenovateOpened is a sample GitHub pull_request hook with action "opened".
//
//go:embed HookPullRequestRenovateOpened.json
var HookPullRequestRenovateOpened string

// HookPullRequestRenovateClosed is a sample GitHub pull_request hook with action "closed".
//
//go:embed HookPullRequestRenovateClosed.json
var HookPullRequestRenovateClosed string

// HookPullRequestRenovateMerged is a sample GitHub pull_request hook with action "closed" and merged=true.
//
//go:embed HookPullRequestRenovateMerged.json
var HookPullRequestRenovateMerged string

// HookIssueRenovateOpened is a sample GitHub issues hook with action "opened".
//
//go:embed HookIssueRenovateOpened.json
var HookIssueRenovateOpened string

// HookIssueRenovateClosed is a sample GitHub issues hook with action "closed".
//
//go:embed HookIssueRenovateClosed.json
var HookIssueRenovateClosed string
