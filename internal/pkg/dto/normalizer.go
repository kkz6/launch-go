package dto

import "strings"

// Normalizable is implemented by request DTOs that need pre-validation normalization.
// The Normalize method is called after parsing but before validation.
type Normalizable interface {
	Normalize()
}

// NormalizeEmptyStringPtr converts empty string pointers to nil.
// This is useful for optional fields where an empty string should be treated as "not provided".
func NormalizeEmptyStringPtr(s **string) {
	if s != nil && *s != nil && strings.TrimSpace(**s) == "" {
		*s = nil
	}
}

// NormalizeTrimPtr trims whitespace from a string pointer.
func NormalizeTrimPtr(s **string) {
	if s != nil && *s != nil {
		trimmed := strings.TrimSpace(**s)
		*s = &trimmed
	}
}

// NormalizeLowercasePtr converts a string pointer to lowercase.
func NormalizeLowercasePtr(s **string) {
	if s != nil && *s != nil {
		lower := strings.ToLower(**s)
		*s = &lower
	}
}

// NormalizeEmailPtr normalizes an email pointer (trim and lowercase).
func NormalizeEmailPtr(s **string) {
	if s != nil && *s != nil {
		normalized := strings.ToLower(strings.TrimSpace(**s))
		if normalized == "" {
			*s = nil
		} else {
			*s = &normalized
		}
	}
}

// NormalizeTrim trims whitespace from a string.
func NormalizeTrim(s *string) {
	if s != nil {
		*s = strings.TrimSpace(*s)
	}
}

// NormalizeLowercase converts a string to lowercase.
func NormalizeLowercase(s *string) {
	if s != nil {
		*s = strings.ToLower(*s)
	}
}

// NormalizeEmail normalizes an email (trim and lowercase).
func NormalizeEmail(s *string) {
	if s != nil {
		*s = strings.ToLower(strings.TrimSpace(*s))
	}
}
