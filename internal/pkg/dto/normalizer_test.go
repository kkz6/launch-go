package dto

import (
	"testing"
)

func TestNormalizeEmptyStringPtr(t *testing.T) {
	tests := []struct {
		name     string
		input    *string
		expected *string
	}{
		{
			name:     "nil pointer",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty string",
			input:    strPtr(""),
			expected: nil,
		},
		{
			name:     "whitespace only",
			input:    strPtr("   "),
			expected: nil,
		},
		{
			name:     "non-empty string",
			input:    strPtr("hello"),
			expected: strPtr("hello"),
		},
		{
			name:     "string with whitespace",
			input:    strPtr("  hello  "),
			expected: strPtr("  hello  "), // Not trimmed, just checking if empty
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.input
			NormalizeEmptyStringPtr(&input)

			if tt.expected == nil {
				if input != nil {
					t.Errorf("expected nil, got %q", *input)
				}
			} else {
				if input == nil {
					t.Errorf("expected %q, got nil", *tt.expected)
				} else if *input != *tt.expected {
					t.Errorf("expected %q, got %q", *tt.expected, *input)
				}
			}
		})
	}
}

func TestNormalizeTrimPtr(t *testing.T) {
	tests := []struct {
		name     string
		input    *string
		expected *string
	}{
		{
			name:     "nil pointer",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty string",
			input:    strPtr(""),
			expected: strPtr(""),
		},
		{
			name:     "string with leading/trailing whitespace",
			input:    strPtr("  hello  "),
			expected: strPtr("hello"),
		},
		{
			name:     "no whitespace",
			input:    strPtr("hello"),
			expected: strPtr("hello"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.input
			NormalizeTrimPtr(&input)

			if tt.expected == nil {
				if input != nil {
					t.Errorf("expected nil, got %q", *input)
				}
			} else {
				if input == nil {
					t.Errorf("expected %q, got nil", *tt.expected)
				} else if *input != *tt.expected {
					t.Errorf("expected %q, got %q", *tt.expected, *input)
				}
			}
		})
	}
}

func TestNormalizeEmailPtr(t *testing.T) {
	tests := []struct {
		name     string
		input    *string
		expected *string
	}{
		{
			name:     "nil pointer",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty string",
			input:    strPtr(""),
			expected: nil,
		},
		{
			name:     "uppercase email",
			input:    strPtr("  TEST@EXAMPLE.COM  "),
			expected: strPtr("test@example.com"),
		},
		{
			name:     "normal email",
			input:    strPtr("user@example.com"),
			expected: strPtr("user@example.com"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.input
			NormalizeEmailPtr(&input)

			if tt.expected == nil {
				if input != nil {
					t.Errorf("expected nil, got %q", *input)
				}
			} else {
				if input == nil {
					t.Errorf("expected %q, got nil", *tt.expected)
				} else if *input != *tt.expected {
					t.Errorf("expected %q, got %q", *tt.expected, *input)
				}
			}
		})
	}
}

func TestNormalizeTrim(t *testing.T) {
	s := "  hello  "
	NormalizeTrim(&s)
	if s != "hello" {
		t.Errorf("expected 'hello', got %q", s)
	}
}

func TestNormalizeLowercase(t *testing.T) {
	s := "HELLO"
	NormalizeLowercase(&s)
	if s != "hello" {
		t.Errorf("expected 'hello', got %q", s)
	}
}

func TestNormalizeEmail(t *testing.T) {
	s := "  TEST@EXAMPLE.COM  "
	NormalizeEmail(&s)
	if s != "test@example.com" {
		t.Errorf("expected 'test@example.com', got %q", s)
	}
}

// Helper function to create string pointers
func strPtr(s string) *string {
	return &s
}
