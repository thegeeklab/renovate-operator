package renovator

import (
	"slices"
	"strings"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/pkg/util"
)

// GetRenovatorOperations returns the operations specified in the operation annotation.
func GetRenovatorOperations(annotations map[string]string) []string {
	if annotations == nil {
		return nil
	}

	return util.SplitAndTrimString(
		annotations[renovatev1beta1.RenovatorOperation],
		renovatev1beta1.RenovatorOperationSeparator,
	)
}

// HasRenovatorOperationDiscover checks if a resource has the discover operation.
func HasRenovatorOperationDiscover(annotations map[string]string) bool {
	return slices.Contains(GetRenovatorOperations(annotations), renovatev1beta1.OperationDiscover)
}

// HasRenovatorOperationRenovate checks if a resource has the renovate operation.
func HasRenovatorOperationRenovate(annotations map[string]string) bool {
	return slices.Contains(GetRenovatorOperations(annotations), renovatev1beta1.OperationRenovate)
}

// HasRenovatorOperation checks if a resource has any renovator operation annotation.
func HasRenovatorOperation(annotations map[string]string) bool {
	return len(GetRenovatorOperations(annotations)) > 0
}

// RemoveRenovatorOperation removes the renovator operation annotation from the given annotations map.
func RemoveRenovatorOperation(annotations map[string]string) map[string]string {
	if annotations == nil {
		return make(map[string]string)
	}

	delete(annotations, renovatev1beta1.RenovatorOperation)

	return annotations
}

// RemoveOperation removes a specific operation from the operation annotation list.
// If the list becomes empty after removal, the annotation key is deleted entirely.
// Other annotations are left intact. Returns the modified annotations map.
func RemoveOperation(annotations map[string]string, operation string) map[string]string {
	if annotations == nil {
		return make(map[string]string)
	}

	ops := GetRenovatorOperations(annotations)
	if len(ops) == 0 {
		return annotations
	}

	newOps := make([]string, 0, len(ops))
	for _, op := range ops {
		if op != operation {
			newOps = append(newOps, op)
		}
	}

	if len(newOps) == 0 {
		delete(annotations, renovatev1beta1.RenovatorOperation)
	} else {
		annotations[renovatev1beta1.RenovatorOperation] = strings.Join(newOps, renovatev1beta1.RenovatorOperationSeparator)
	}

	return annotations
}
