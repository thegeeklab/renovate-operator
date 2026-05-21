package fixtures

import _ "embed"

// HookPush is a sample Gitea push hook to the default branch.
//
//go:embed HookPush.json
var HookPush string

// HookPushBranch is a sample Gitea push hook where a new branch was created.
//
//go:embed HookPushBranch.json
var HookPushBranch string

// HookTag is a sample Gitea tag hook.
//
//go:embed HookTag.json
var HookTag string

// HookPullRequestRenovateChecked is a sample Gitea pull_request hook with renovate content and checked checkboxes.
//
//go:embed HookPullRequestRenovateChecked.json
var HookPullRequestRenovateChecked string

// HookPullRequestRenovateUnchecked is a sample Gitea pull_request hook with renovate content but unchecked checkboxes.
//
//go:embed HookPullRequestRenovateUnchecked.json
var HookPullRequestRenovateUnchecked string

// HookPullRequestRegular is a sample Gitea pull_request hook with regular PR content (no renovate markers).
//
//go:embed HookPullRequestRegular.json
var HookPullRequestRegular string

// HookPullRequestRenovateRebase is a sample Gitea pull_request hook with renovate rebase checkbox (unchecked).
//
//go:embed HookPullRequestRenovateRebase.json
var HookPullRequestRenovateRebase string
