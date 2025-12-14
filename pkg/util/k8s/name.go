package k8s

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// DNS1123MaxLength is the maximum length for DNS-1123 subdomain names (253 characters).
// This is a DNS protocol constraint for domain name labels.
const DNS1123MaxLength = 253

var errInvalidName = errors.New("invalid name")

// SanitizeName sanitizes a string to create a valid Kubernetes object name.
// It follows DNS-1123 subdomain conventions:
// - Contains only lowercase alphanumeric characters, hyphens (-), and dots (.).
// - Starts and ends with an alphanumeric character.
// - No longer than 253 characters.
// - No consecutive hyphens or dots.
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

	// Replace common separators with hyphens, but preserve dots
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "_", "-")

	// Replace other invalid characters with hyphens
	// Keep only alphanumeric characters, hyphens, and dots
	validChars := regexp.MustCompile(`[^a-z0-9\-\.]`)
	name = validChars.ReplaceAllString(name, "-")

	// Replace consecutive hyphens with a single hyphen
	consecutiveHyphens := regexp.MustCompile(`-+`)
	name = consecutiveHyphens.ReplaceAllString(name, "-")

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
		if len(name) > 0 && !isAlphanumeric(name[len(name)-1]) {
			name = name[:len(name)-1]
		}
	}

	return name, nil
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
