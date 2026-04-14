package kernelref

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	urlRefPattern = regexp.MustCompile(`(?i)\bhttps?://(?:www\.)?kaggle\.com/(?:code|kernels)/([A-Za-z0-9][A-Za-z0-9._-]*)/([A-Za-z0-9][A-Za-z0-9._-]*)(?:[/?#].*)?\b`)
	rawRefPattern = regexp.MustCompile(`(?i)\b([A-Za-z0-9][A-Za-z0-9._-]*)/([A-Za-z0-9][A-Za-z0-9._-]*)\b`)
)

// Normalize converts a kernel reference or Kaggle URL into the canonical owner/kernel-slug form.
func Normalize(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("kernel identity is required")
	}

	if ref, ok := extractURLRef(trimmed); ok {
		return ref, nil
	}
	if ref, ok := extractRawRef(trimmed); ok {
		return ref, nil
	}

	return "", fmt.Errorf("kernel identity %q is not a canonical Kaggle kernel reference", trimmed)
}

// ExtractFromText finds a canonical kernel reference in free-form CLI output.
func ExtractFromText(output string) (string, error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return "", fmt.Errorf("kernel identity not found in push output")
	}

	matches := make(map[string]struct{})
	for _, line := range strings.Split(trimmed, "\n") {
		if ref, ok := extractURLRef(line); ok {
			matches[ref] = struct{}{}
			continue
		}
		for _, ref := range extractRawRefs(line) {
			matches[ref] = struct{}{}
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("kernel identity not found in push output")
	case 1:
		for ref := range matches {
			return ref, nil
		}
	default:
		candidates := make([]string, 0, len(matches))
		for ref := range matches {
			candidates = append(candidates, ref)
		}
		return "", fmt.Errorf("kernel identity is ambiguous: %s", strings.Join(candidates, ", "))
	}

	return "", fmt.Errorf("kernel identity not found in push output")
}

func extractURLRef(value string) (string, bool) {
	matches := urlRefPattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(matches) != 3 {
		return "", false
	}
	return matches[1] + "/" + matches[2], true
}

func extractRawRef(value string) (string, bool) {
	matches := rawRefPattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(matches) != 3 {
		return "", false
	}
	return matches[1] + "/" + matches[2], true
}

func extractRawRefs(value string) []string {
	matches := rawRefPattern.FindAllStringSubmatch(strings.TrimSpace(value), -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) != 3 {
			continue
		}
		out = append(out, match[1]+"/"+match[2])
	}
	return out
}
