package k8s

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	hashLength = 10
)

var (
	errInvalidName   = errors.New("invalid name")
	errInvalidSuffix = errors.New("suffix is too long")

	dnsDisallowedChars = regexp.MustCompile(`[^-.a-z0-9]+`)
	dotsAndDashes      = regexp.MustCompile(`[.-]{2,}`)
	labelDisallowed    = regexp.MustCompile(`[^-_.a-z0-9]+`)
	labelSeparators    = regexp.MustCompile(`[-_.]{2,}`)
)

func truncateWithHash(s, original string, maxLen int, trimChars string) string {
	hash := sha256.Sum256([]byte(original))
	hashStr := hex.EncodeToString(hash[:])[:hashLength]
	maxBaseLength := maxLen - 1 - len(hashStr)

	if len(s) == 0 {
		return hashStr
	}

	if len(s) <= maxBaseLength {
		return s + "-" + hashStr
	}

	truncated := strings.TrimRight(s[:maxBaseLength], trimChars)

	return truncated + "-" + hashStr
}

// SanitizeSubdomain sanitizes a string to create a valid DNS-1123 subdomain.
func SanitizeSubdomain(name string) (string, error) {
	if name == "" {
		return "", nil
	}

	res := strings.ToLower(name)
	res = dnsDisallowedChars.ReplaceAllString(res, "-")
	res = dotsAndDashes.ReplaceAllStringFunc(res, func(s string) string {
		if strings.ContainsRune(s, '.') {
			return "."
		}

		return "-"
	})
	res = strings.Trim(res, "-.")

	if len(res) > validation.DNS1123SubdomainMaxLength {
		res = truncateWithHash(res, name, validation.DNS1123SubdomainMaxLength, "-.")
	}

	if res == "" || len(validation.IsDNS1123Subdomain(res)) > 0 {
		return "", fmt.Errorf("name is not a valid kubernetes DNS name: %w", errInvalidName)
	}

	return res, nil
}

// DeterministicSubdomain generates a valid Kubernetes name bounded by the 253-character limit (DNS-1123).
// If the combined length exceeds 253 characters, it safely truncates the base name
// and injects a SHA-256 hash of the original base name to prevent duplicate name collisions.
func DeterministicSubdomain(baseName, suffix string) (string, error) {
	sanitizedBase, err := SanitizeSubdomain(baseName)
	if err != nil {
		return "", err
	}

	if len(sanitizedBase)+len(suffix) <= validation.DNS1123SubdomainMaxLength {
		return sanitizedBase + suffix, nil
	}

	maxLen := validation.DNS1123SubdomainMaxLength - len(suffix)

	if maxLen <= 1+hashLength {
		return "", fmt.Errorf("failed to generate deterministic name: %w", errInvalidSuffix)
	}

	return truncateWithHash(sanitizedBase, baseName, maxLen, "-") + suffix, nil
}

// SanitizeLabel sanitizes a string to create a valid Kubernetes label value.
func SanitizeLabel(name string) string {
	if name == "" {
		return ""
	}

	res := strings.ToLower(name)
	res = labelDisallowed.ReplaceAllString(res, "-")
	res = labelSeparators.ReplaceAllString(res, "-")
	res = strings.Trim(res, "-_.")

	if len(res) > validation.LabelValueMaxLength {
		res = truncateWithHash(res, name, validation.LabelValueMaxLength, "-_.")
	}

	if len(validation.IsValidLabelValue(res)) > 0 {
		return ""
	}

	return res
}
