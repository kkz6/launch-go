// Package utils provides common utility functions for string manipulation,
// map operations, and slice operations.
package xutil

import (
	"strings"
	"unicode"
)

// Truncate shortens a string to the specified maximum length.
// If the string is longer than max, it is truncated and "..." is appended.
// The returned string will be at most max characters (including "...").
//
// Example:
//
//	Truncate("Hello World", 8) // Returns "Hello..."
//	Truncate("Hi", 10)         // Returns "Hi"
func Truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// ToSnakeCase converts a string to snake_case.
// It handles camelCase, PascalCase, and spaces.
//
// Example:
//
//	ToSnakeCase("HelloWorld")  // Returns "hello_world"
//	ToSnakeCase("helloWorld")  // Returns "hello_world"
//	ToSnakeCase("Hello World") // Returns "hello_world"
//	ToSnakeCase("APIKey")      // Returns "api_key"
func ToSnakeCase(s string) string {
	if s == "" {
		return ""
	}

	var result strings.Builder
	result.Grow(len(s) + 10) // Pre-allocate for underscores

	prevLower := false
	prevUpper := false

	for i, r := range s {
		if r == ' ' || r == '-' || r == '_' {
			if result.Len() > 0 && !strings.HasSuffix(result.String(), "_") {
				_, _ = result.WriteRune('_')
			}
			prevLower = false
			prevUpper = false
			continue
		}

		if unicode.IsUpper(r) {
			// Add underscore before uppercase if:
			// - Previous was lowercase (helloWorld -> hello_world)
			// - Previous was uppercase AND next is lowercase (APIKey -> api_key)
			if i > 0 && result.Len() > 0 {
				if prevLower {
					_, _ = result.WriteRune('_')
				} else if prevUpper && i+1 < len(s) {
					nextRune := rune(s[i+1])
					if unicode.IsLower(nextRune) {
						_, _ = result.WriteRune('_')
					}
				}
			}
			_, _ = result.WriteRune(unicode.ToLower(r))
			prevUpper = true
			prevLower = false
		} else {
			_, _ = result.WriteRune(unicode.ToLower(r))
			prevLower = true
			prevUpper = false
		}
	}

	return result.String()
}

// ToCamelCase converts a string to camelCase.
// It handles snake_case, kebab-case, and space-separated words.
//
// Example:
//
//	ToCamelCase("hello_world")  // Returns "helloWorld"
//	ToCamelCase("hello-world")  // Returns "helloWorld"
//	ToCamelCase("Hello World")  // Returns "helloWorld"
//	ToCamelCase("HELLO_WORLD")  // Returns "helloWorld"
func ToCamelCase(s string) string {
	if s == "" {
		return ""
	}

	var result strings.Builder
	result.Grow(len(s))

	capitalizeNext := false
	firstWritten := false

	for _, r := range s {
		if r == '_' || r == '-' || r == ' ' {
			capitalizeNext = true
			continue
		}

		if !firstWritten {
			_, _ = result.WriteRune(unicode.ToLower(r))
			firstWritten = true
		} else if capitalizeNext {
			_, _ = result.WriteRune(unicode.ToUpper(r))
			capitalizeNext = false
		} else {
			_, _ = result.WriteRune(unicode.ToLower(r))
		}
	}

	return result.String()
}

// ToPascalCase converts a string to PascalCase.
// It handles snake_case, kebab-case, and space-separated words.
//
// Example:
//
//	ToPascalCase("hello_world")  // Returns "HelloWorld"
//	ToPascalCase("hello-world")  // Returns "HelloWorld"
//	ToPascalCase("Hello World")  // Returns "HelloWorld"
func ToPascalCase(s string) string {
	if s == "" {
		return ""
	}

	var result strings.Builder
	result.Grow(len(s))

	capitalizeNext := true

	for _, r := range s {
		if r == '_' || r == '-' || r == ' ' {
			capitalizeNext = true
			continue
		}

		if capitalizeNext {
			_, _ = result.WriteRune(unicode.ToUpper(r))
			capitalizeNext = false
		} else {
			_, _ = result.WriteRune(unicode.ToLower(r))
		}
	}

	return result.String()
}

// Contains checks if a string slice contains the given string.
//
// Example:
//
//	Contains([]string{"a", "b", "c"}, "b") // Returns true
//	Contains([]string{"a", "b", "c"}, "d") // Returns false
func Contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// ContainsIgnoreCase checks if a string slice contains the given string,
// ignoring case.
//
// Example:
//
//	ContainsIgnoreCase([]string{"Hello", "World"}, "hello") // Returns true
func ContainsIgnoreCase(slice []string, s string) bool {
	lower := strings.ToLower(s)
	for _, item := range slice {
		if strings.ToLower(item) == lower {
			return true
		}
	}
	return false
}

// IsEmpty checks if a string is empty or contains only whitespace.
//
// Example:
//
//	IsEmpty("")      // Returns true
//	IsEmpty("   ")   // Returns true
//	IsEmpty("hello") // Returns false
func IsEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}

// IsNotEmpty checks if a string is not empty and contains non-whitespace characters.
//
// Example:
//
//	IsNotEmpty("hello") // Returns true
//	IsNotEmpty("")      // Returns false
//	IsNotEmpty("   ")   // Returns false
func IsNotEmpty(s string) bool {
	return strings.TrimSpace(s) != ""
}

// DefaultIfEmpty returns the default value if the string is empty.
//
// Example:
//
//	DefaultIfEmpty("", "default")    // Returns "default"
//	DefaultIfEmpty("value", "default") // Returns "value"
func DefaultIfEmpty(s, defaultVal string) string {
	if IsEmpty(s) {
		return defaultVal
	}
	return s
}
