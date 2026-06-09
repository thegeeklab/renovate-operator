package util

// EmptyIfNil returns an empty slice if the input is nil.
func EmptyIfNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}

	return s
}
