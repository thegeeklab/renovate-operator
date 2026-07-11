package util

// EmptyIfNil returns an empty slice if the input is nil.
func EmptyIfNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}

	return s
}

// ContainsAll returns true if a contains all elements of b.
func ContainsAll[T comparable](a, b []T) bool {
	set := make(map[T]struct{}, len(a))
	for _, item := range a {
		set[item] = struct{}{}
	}

	for _, item := range b {
		if _, ok := set[item]; !ok {
			return false
		}
	}

	return true
}
