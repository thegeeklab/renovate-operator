package k8s

import (
	"errors"
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"
)

const (
	// DNS1123MaxLength is the maximum length for DNS-1123 subdomain names (253 characters).
	// This is a DNS protocol constraint for domain name labels.
	DNS1123MaxLength = 253

	// DNS1035LabelMaxLength is the maximum length for DNS-1035 labels (63 characters).
	// This is strictly enforced for Kubernetes Job names because they are mapped to K8s labels.
	DNS1035LabelMaxLength = 63
)

var (
	errInvalidName   = errors.New("invalid name")
	errInvalidSuffix = errors.New("suffix is too long")

	// invalidCharsRegex matches any sequence of characters that are not lowercase a-z or 0-9.
	// Compiling this globally prevents expensive recompilation on every function call.
	invalidCharsRegex = regexp.MustCompile(`[^a-z0-9]+`)
)

// SanitizeName sanitizes a string to create a valid Kubernetes object name.
// It follows DNS-1123 subdomain conventions:
// - Contains only lowercase alphanumeric characters and hyphens (-).
// - Starts and ends with an alphanumeric character.
// - No longer than 253 characters.
// - No consecutive hyphens.
// Returns an error if the input contains only invalid characters.
func SanitizeName(name string) (string, error) {
	if name == "" {
		return "", nil
	}

	// Check if the name contains only invalid characters
	if !hasValidCharacters(name) {
		return "", fmt.Errorf("name contains only invalid characters: %w", errInvalidName)
	}

	// Convert to lowercase
	name = strings.ToLower(name)

	// Replace ANY sequence of non-alphanumeric characters (including /, _, ., etc.)
	// with a single hyphen. This handles replacement and deduplication in one pass.
	name = invalidCharsRegex.ReplaceAllString(name, "-")

	// Ensure the name starts with an alphanumeric character
	if len(name) > 0 && !isAlphanumeric(name[0]) {
		name = "repo-" + strings.TrimLeft(name, "-")
	}

	// Ensure the name ends with an alphanumeric character
	if len(name) > 0 && !isAlphanumeric(name[len(name)-1]) {
		name = strings.TrimRight(name, "-") + "-repo"
	}

	// Truncate to maximum length (DNS1123MaxLength characters for DNS-1123 subdomain)
	if len(name) > DNS1123MaxLength {
		name = name[:DNS1123MaxLength]
		// Ensure it still ends with alphanumeric after truncation
		for len(name) > 0 && !isAlphanumeric(name[len(name)-1]) {
			name = name[:len(name)-1]
		}
	}

	return name, nil
}

// DeterministicName generates a valid Kubernetes name bounded by the 63-character limit (DNS-1035).
// If the combined length exceeds 63 characters, it safely truncates the base name
// and injects a hash of the original base name to prevent duplicate name collisions.
func DeterministicName(baseName, suffix string) (string, error) {
	return deterministicNameWithLimit(baseName, suffix, DNS1035LabelMaxLength)
}

// DeterministicSubdomainName generates a valid Kubernetes name bounded by the 253-character limit (DNS-1123).
// If the combined length exceeds 253 characters, it safely truncates the base name
// and injects a hash of the original base name to prevent duplicate name collisions.
func DeterministicSubdomainName(baseName, suffix string) (string, error) {
	return deterministicNameWithLimit(baseName, suffix, DNS1123MaxLength)
}

func deterministicNameWithLimit(baseName, suffix string, maxLength int) (string, error) {
	sanitizedBase, err := SanitizeName(baseName)
	if err != nil {
		return "", err
	}

	// If it fits perfectly, just concatenate and return
	if len(sanitizedBase)+len(suffix) <= maxLength {
		return fmt.Sprintf("%s%s", sanitizedBase, suffix), nil
	}

	// If it exceeds the limit, we must truncate and add a hash to avoid collisions.
	// We use the original baseName for the hash to maximize uniqueness before sanitization.
	h := fnv.New32a()
	h.Write([]byte(baseName))

	// Use %08x to pad with leading zeros, guaranteeing exactly 8 characters every time
	hashStr := fmt.Sprintf("%08x", h.Sum32())

	// Calculate how much space we have for the base name.
	// We need room for: "-<hash><suffix>"
	reservedLength := 1 + len(hashStr) + len(suffix)
	maxBaseLength := DNS1035LabelMaxLength - reservedLength

	if maxBaseLength <= 0 {
		return "", fmt.Errorf("failed to generate deterministic name: %w", errInvalidSuffix)
	}

	truncatedBase := sanitizedBase[:maxBaseLength]

	// Ensure truncation doesn't leave a trailing hyphen
	truncatedBase = strings.TrimRight(truncatedBase, "-")

	return fmt.Sprintf("%s-%s%s", truncatedBase, hashStr, suffix), nil
}

func hasValidCharacters(name string) bool {
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '/' || c == '_' || c == '.' {
			return true
		}
	}

	return false
}

func isAlphanumeric(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
}
