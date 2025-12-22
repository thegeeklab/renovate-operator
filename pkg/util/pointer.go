package util

func PtrIsNonZero(ptr *int32) bool {
	return ptr != nil && *ptr != 0
}
