package renovate

import (
	"strings"
)

func IsRenovateContent(description string) bool {
	if description == "" {
		return false
	}

	patternList := []string{
		"## Detected Dependencies",
		"<!-- rebase-check -->",
		"<!--renovate-debug:",
		"<!-- rebase-all-open-prs -->",
		"<!-- rebase-branch=",
		"<!-- approve-all-pending-prs -->",
		"<!-- approvePr-branch=",
		"<!-- approve-branch=",
		"<!-- recreate-branch=",
		"<!-- unschedule-branch=",
		"<!-- create-config-migration-pr -->",
		"<!-- create-all-awaiting-schedule-prs -->",
		"<!-- create-all-rate-limited-prs -->",
		"<!-- unlimit-branch=",
		"<!-- manual job -->",
	}

	for _, pattern := range patternList {
		if strings.Contains(description, pattern) {
			return true
		}
	}

	return false
}

func HasCheckboxBeenChecked(current string) bool {
	if current == "" {
		return false
	}

	return strings.Contains(current, "- [x]") ||
		strings.Contains(current, "- [X]")
}

func VerifyRenovateDescriptionChange(current string) bool {
	if !IsRenovateContent(current) {
		return false
	}

	return HasCheckboxBeenChecked(current)
}
