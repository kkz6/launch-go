package xutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		max      int
		expected string
	}{
		{
			name:     "shorter than max",
			input:    "Hello",
			max:      10,
			expected: "Hello",
		},
		{
			name:     "exact max length",
			input:    "Hello",
			max:      5,
			expected: "Hello",
		},
		{
			name:     "longer than max",
			input:    "Hello World",
			max:      8,
			expected: "Hello...",
		},
		{
			name:     "very short max",
			input:    "Hello",
			max:      3,
			expected: "Hel",
		},
		{
			name:     "zero max",
			input:    "Hello",
			max:      0,
			expected: "",
		},
		{
			name:     "negative max",
			input:    "Hello",
			max:      -1,
			expected: "",
		},
		{
			name:     "empty string",
			input:    "",
			max:      10,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Truncate(tt.input, tt.max)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "camelCase",
			input:    "helloWorld",
			expected: "hello_world",
		},
		{
			name:     "PascalCase",
			input:    "HelloWorld",
			expected: "hello_world",
		},
		{
			name:     "with spaces",
			input:    "Hello World",
			expected: "hello_world",
		},
		{
			name:     "with hyphens",
			input:    "hello-world",
			expected: "hello_world",
		},
		{
			name:     "already snake_case",
			input:    "hello_world",
			expected: "hello_world",
		},
		{
			name:     "acronym at start",
			input:    "APIKey",
			expected: "api_key",
		},
		{
			name:     "acronym in middle",
			input:    "getAPIKey",
			expected: "get_api_key",
		},
		{
			name:     "single word",
			input:    "Hello",
			expected: "hello",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "multiple underscores",
			input:    "hello__world",
			expected: "hello_world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToSnakeCase(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToCamelCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "snake_case",
			input:    "hello_world",
			expected: "helloWorld",
		},
		{
			name:     "kebab-case",
			input:    "hello-world",
			expected: "helloWorld",
		},
		{
			name:     "with spaces",
			input:    "Hello World",
			expected: "helloWorld",
		},
		{
			name:     "already camelCase",
			input:    "helloWorld",
			expected: "helloworld",
		},
		{
			name:     "UPPERCASE",
			input:    "HELLO_WORLD",
			expected: "helloWorld",
		},
		{
			name:     "single word",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToCamelCase(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "snake_case",
			input:    "hello_world",
			expected: "HelloWorld",
		},
		{
			name:     "kebab-case",
			input:    "hello-world",
			expected: "HelloWorld",
		},
		{
			name:     "with spaces",
			input:    "hello world",
			expected: "HelloWorld",
		},
		{
			name:     "single word",
			input:    "hello",
			expected: "Hello",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToPascalCase(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContains(t *testing.T) {
	t.Run("element exists", func(t *testing.T) {
		assert.True(t, Contains([]string{"a", "b", "c"}, "b"))
	})

	t.Run("element does not exist", func(t *testing.T) {
		assert.False(t, Contains([]string{"a", "b", "c"}, "d"))
	})

	t.Run("empty slice", func(t *testing.T) {
		assert.False(t, Contains([]string{}, "a"))
	})

	t.Run("nil slice", func(t *testing.T) {
		assert.False(t, Contains(nil, "a"))
	})
}

func TestContainsIgnoreCase(t *testing.T) {
	t.Run("exact match", func(t *testing.T) {
		assert.True(t, ContainsIgnoreCase([]string{"Hello", "World"}, "Hello"))
	})

	t.Run("case insensitive match", func(t *testing.T) {
		assert.True(t, ContainsIgnoreCase([]string{"Hello", "World"}, "hello"))
	})

	t.Run("no match", func(t *testing.T) {
		assert.False(t, ContainsIgnoreCase([]string{"Hello", "World"}, "Foo"))
	})
}

func TestIsEmpty(t *testing.T) {
	t.Run("empty string", func(t *testing.T) {
		assert.True(t, IsEmpty(""))
	})

	t.Run("whitespace only", func(t *testing.T) {
		assert.True(t, IsEmpty("   "))
	})

	t.Run("non-empty string", func(t *testing.T) {
		assert.False(t, IsEmpty("hello"))
	})

	t.Run("string with spaces around", func(t *testing.T) {
		assert.False(t, IsEmpty("  hello  "))
	})
}

func TestIsNotEmpty(t *testing.T) {
	t.Run("non-empty string", func(t *testing.T) {
		assert.True(t, IsNotEmpty("hello"))
	})

	t.Run("empty string", func(t *testing.T) {
		assert.False(t, IsNotEmpty(""))
	})

	t.Run("whitespace only", func(t *testing.T) {
		assert.False(t, IsNotEmpty("   "))
	})
}

func TestDefaultIfEmpty(t *testing.T) {
	t.Run("non-empty string", func(t *testing.T) {
		assert.Equal(t, "value", DefaultIfEmpty("value", "default"))
	})

	t.Run("empty string", func(t *testing.T) {
		assert.Equal(t, "default", DefaultIfEmpty("", "default"))
	})

	t.Run("whitespace only", func(t *testing.T) {
		assert.Equal(t, "default", DefaultIfEmpty("   ", "default"))
	})
}
