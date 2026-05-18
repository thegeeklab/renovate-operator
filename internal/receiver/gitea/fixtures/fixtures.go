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
