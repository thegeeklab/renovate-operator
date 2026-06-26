package fixtures

import _ "embed"

// WarnAndError contains logs with warnings and errors.
//
//go:embed WarnAndError.json
var WarnAndError string

// DuplicateWarnings contains duplicate warning messages.
//
//go:embed DuplicateWarnings.json
var DuplicateWarnings string

// PRCreated contains logs for PR creation.
//
//go:embed PRCreated.json
var PRCreated string

// PRUpdated contains logs for PR update.
//
//go:embed PRUpdated.json
var PRUpdated string

// PRAutomerged contains logs for PR automerge.
//
//go:embed PRAutomerged.json
var PRAutomerged string

// PRUnchanged contains logs for unchanged PR.
//
//go:embed PRUnchanged.json
var PRUnchanged string

// BranchesInfoExtended contains logs for branches info.
//
//go:embed BranchesInfoExtended.json
var BranchesInfoExtended string

// MixedLevels contains logs with different log levels.
//
//go:embed MixedLevels.json
var MixedLevels string

// HTMLInMessage contains a log line with HTML in the message.
//
//go:embed HTMLInMessage.json
var HTMLInMessage string

// MixedJSONAndPlain contains a mix of JSON and plain text lines.
//
//go:embed MixedJSONAndPlain.json
var MixedJSONAndPlain string

// SortedPRs contains logs with multiple PRs that should be sorted.
//
//go:embed SortedPRs.json
var SortedPRs string

// PRURLsFromGitPush contains logs with PR URLs from git push.
//
//go:embed PRURLsFromGitPush.json
var PRURLsFromGitPush string

// RepoFinishedDisabledByConfig contains a repository finished log with disabled-by-config result.
//
//go:embed RepoFinishedDisabledByConfig.json
var RepoFinishedDisabledByConfig string

// RepoFinishedDisabledClosedOnboarding contains a repository finished log with disabled-closed-onboarding result.
//
//go:embed RepoFinishedDisabledClosedOnboarding.json
var RepoFinishedDisabledClosedOnboarding string

// RepoFinishedDisabledNoConfig contains a repository finished log with disabled-no-config result.
//
//go:embed RepoFinishedDisabledNoConfig.json
var RepoFinishedDisabledNoConfig string

// RepoFinishedOnboarding contains a repository finished log with onboarding status.
//
//go:embed RepoFinishedOnboarding.json
var RepoFinishedOnboarding string

// RepoFinishedUnknown contains a repository finished log with no result or status.
//
//go:embed RepoFinishedUnknown.json
var RepoFinishedUnknown string

// RepoFinishedDone contains a repository finished log with done result.
//
//go:embed RepoFinishedDone.json
var RepoFinishedDone string
