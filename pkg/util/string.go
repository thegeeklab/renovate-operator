package util

import "strings"

// SplitAndTrimString returns a new slice from a string separated by the given separator with all empty entries removed.
func SplitAndTrimString(s, sep string) []string {
	if len(s) == 0 {
		return nil
	}

	result := strings.Split(s, sep)
	for i := range result {
		result[i] = strings.TrimSpace(result[i])
	}

	return result
}
