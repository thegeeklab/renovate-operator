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
	// SortByLastUpdate sorts by last update time comparison.
	SortByLastUpdate SortBy = "last_update"
)

// SortItems sorts a slice in-place by name or date fields using accessor functions.
func SortItems[T any](items []T, sortBy SortBy, order SortOrder, nameFn func(T) string, dateFns ...func(T) time.Time) {
	slices.SortFunc(items, func(a, b T) int {
		var result int

		switch sortBy {
		case SortByDate:
			result = dateFns[0](a).Compare(dateFns[0](b))
		case SortByLastUpdate:
			if len(dateFns) > 1 {
				result = dateFns[1](a).Compare(dateFns[1](b))
			} else {
				result = dateFns[0](a).Compare(dateFns[0](b))
			}
		default:
			result = cmp.Compare(nameFn(a), nameFn(b))
		}

		if order == SortDesc {
			return -result
		}

		return result
	})
}
