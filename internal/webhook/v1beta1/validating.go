package v1beta1

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
)

var (
	ErrInvalidTimezone = errors.New("invalid timezone")
	ErrInvalidPath     = errors.New("path must be absolute")
)

// validateTimezone returns an error if tz is not a valid IANA timezone name.
// An empty string is considered valid (caller will use local time).
func validateTimezone(tz string) error {
	if tz == "" {
		return nil
	}

	if _, err := time.LoadLocation(tz); err != nil {
		return fmt.Errorf("%w %q: %w", ErrInvalidTimezone, tz, err)
	}

	return nil
}

// validateScratchVolumePath validates that the scratch volume path is absolute.
// Returns nil if scratch is nil or path is empty (will be defaulted).
func validateScratchVolumePath(scratch *renovatev1beta1.ScratchVolumeSpec) error {
	if scratch == nil || scratch.Path == "" {
		return nil
	}

	if !filepath.IsAbs(scratch.Path) || strings.HasPrefix(scratch.Path, "//") {
		return fmt.Errorf("%w: %q", ErrInvalidPath, scratch.Path)
	}

	return nil
}
