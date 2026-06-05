package util

import (
	"cmp"
	"slices"
	"time"
)

// SortOrder defines the sorting direction.
type SortOrder string

const (
	// SortAsc sorts in ascending order.
	SortAsc SortOrder = "asc"
	// SortDesc sorts in descending order.
	SortDesc SortOrder = "desc"
)

// SortBy defines the sorting criteria.
type SortBy string

const (
	// SortByName sorts by string comparison.
	SortByName SortBy = "name"
	// SortByDate sorts by time comparison.
	SortByDate SortBy = "date"
)

// SortItems sorts a slice in-place by name or date fields using accessor functions.
func SortItems[T any](items []T, sortBy SortBy, order SortOrder, nameFn func(T) string, dateFn func(T) time.Time) {
	slices.SortFunc(items, func(a, b T) int {
		var result int

		switch sortBy {
		case SortByDate:
			result = dateFn(a).Compare(dateFn(b))
		default:
			result = cmp.Compare(nameFn(a), nameFn(b))
		}

		if order == SortDesc {
			return -result
		}

		return result
	})
}
