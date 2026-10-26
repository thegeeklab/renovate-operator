package renovator

import (
	"encoding/json"
	"errors"
	"fmt"

	jsonpatch "github.com/evanphx/json-patch/v5"
)

var errConfigNotObject = errors.New("config must be a JSON object")

// mergeRenovateConfig deep-merges the user-provided configuration over the
// operator-managed base configuration using JSON merge patch semantics: objects
// are merged recursively and all other values (including arrays) are replaced.
// A null value removes the corresponding key from the merged result.
//
// The user-provided configuration must be a JSON object; any other value is
// rejected to avoid silently replacing the entire generated configuration.
func mergeRenovateConfig(base, overlay []byte) ([]byte, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(overlay, &obj); err != nil {
		return nil, fmt.Errorf("%w: %w", errConfigNotObject, err)
	}

	if obj == nil {
		return nil, fmt.Errorf("%w, not null", errConfigNotObject)
	}

	merged, err := jsonpatch.MergePatch(base, overlay)
	if err != nil {
		return nil, fmt.Errorf("merge config: %w", err)
	}

	return merged, nil
}
