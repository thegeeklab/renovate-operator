package renovator

import (
	"slices"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/util"
)

// GetRenovatorOperations returns the operations specified in the operation annotation.
// This function can be used for both Discovery and Renovator resources.
func GetRenovatorOperations(annotations map[string]string) []string {
	return util.SplitAndTrimString(
		annotations[renovatev1beta1.RenovatorOperation],
		renovatev1beta1.RenovatorOperationSeparator,
	)
}

// HasRenovatorOperationDiscover checks if a resource has the discover operation.
// This function can be used for both Discovery and Renovator resources.
func HasRenovatorOperationDiscover(annotations map[string]string) bool {
	operations := GetRenovatorOperations(annotations)

	return slices.Contains(operations, renovatev1beta1.OperationDiscover)
}

func RemoveRenovatorOperation(annotations map[string]string) map[string]string {
	if annotations == nil {
		annotations = make(map[string]string)
	}

	delete(annotations, renovatev1beta1.RenovatorOperation)

	return annotations
}
