package view

import (
	"time"

	"github.com/dustin/go-humanize"
)

// humanizeTime wraps humanize.Time so templates don't need to import the package directly.
func humanizeTime(t time.Time) string {
	return humanize.Time(t)
}
