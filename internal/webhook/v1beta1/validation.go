package v1beta1

import (
	"fmt"
	"time"
)

// validateTimezone returns an error if tz is not a valid IANA timezone name.
// An empty string is considered valid (caller will use local time).
func validateTimezone(tz string) error {
	if tz == "" {
		return nil
	}

	if _, err := time.LoadLocation(tz); err != nil {
		return fmt.Errorf("invalid timezone %q: %w", tz, err)
	}

	return nil
}
