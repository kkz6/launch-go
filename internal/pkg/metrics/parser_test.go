package metrics

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestNewParser(t *testing.T) {
	logger := zerolog.New(nil)
	parser := NewParser(logger, "server-123")

	assert.NotNil(t, parser)
	assert.Equal(t, "server-123", parser.serverID)
	assert.Empty(t, parser.errors)
}

func TestParseFloat(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		fieldName   string
		expected    float64
		shouldError bool
	}{
		{
			name:        "valid float",
			value:       "3.14",
			fieldName:   "cpu_usage",
			expected:    3.14,
			shouldError: false,
		},
		{
			name:        "valid integer as float",
			value:       "42",
			fieldName:   "memory",
			expected:    42.0,
			shouldError: false,
		},
		{
			name:        "empty string",
			value:       "",
			fieldName:   "empty_field",
			expected:    0,
			shouldError: false,
		},
		{
			name:        "whitespace only",
			value:       "   ",
			fieldName:   "whitespace_field",
			expected:    0,
			shouldError: false,
		},
		{
			name:        "value with whitespace",
			value:       "  75.5  ",
			fieldName:   "percent",
			expected:    75.5,
			shouldError: false,
		},
		{
			name:        "invalid value",
			value:       "not_a_number",
			fieldName:   "invalid_field",
			expected:    0,
			shouldError: true,
		},
		{
			name:        "negative float",
			value:       "-10.5",
			fieldName:   "negative",
			expected:    -10.5,
			shouldError: false,
		},
		{
			name:        "zero",
			value:       "0",
			fieldName:   "zero_field",
			expected:    0,
			shouldError: false,
		},
		{
			name:        "very large number",
			value:       "9999999999.99",
			fieldName:   "large",
			expected:    9999999999.99,
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := zerolog.New(&buf)
			parser := NewParser(logger, "test-server")

			result := parser.ParseFloat(tt.value, tt.fieldName)

			assert.Equal(t, tt.expected, result)

			if tt.shouldError {
				assert.True(t, parser.HasErrors())
				assert.Contains(t, parser.Errors(), tt.fieldName)
				assert.Contains(t, buf.String(), tt.fieldName)
				assert.Contains(t, buf.String(), "test-server")
			} else {
				assert.False(t, parser.HasErrors())
			}
		})
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		fieldName   string
		expected    int64
		shouldError bool
	}{
		{
			name:        "valid integer",
			value:       "42",
			fieldName:   "count",
			expected:    42,
			shouldError: false,
		},
		{
			name:        "empty string",
			value:       "",
			fieldName:   "empty_field",
			expected:    0,
			shouldError: false,
		},
		{
			name:        "whitespace only",
			value:       "   ",
			fieldName:   "whitespace_field",
			expected:    0,
			shouldError: false,
		},
		{
			name:        "value with whitespace",
			value:       "  100  ",
			fieldName:   "padded",
			expected:    100,
			shouldError: false,
		},
		{
			name:        "invalid value",
			value:       "abc",
			fieldName:   "invalid_field",
			expected:    0,
			shouldError: true,
		},
		{
			name:        "float value",
			value:       "3.14",
			fieldName:   "float_field",
			expected:    0,
			shouldError: true,
		},
		{
			name:        "negative integer",
			value:       "-500",
			fieldName:   "negative",
			expected:    -500,
			shouldError: false,
		},
		{
			name:        "zero",
			value:       "0",
			fieldName:   "zero_field",
			expected:    0,
			shouldError: false,
		},
		{
			name:        "very large number",
			value:       "9223372036854775807",
			fieldName:   "large",
			expected:    9223372036854775807,
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := zerolog.New(&buf)
			parser := NewParser(logger, "test-server")

			result := parser.ParseInt(tt.value, tt.fieldName)

			assert.Equal(t, tt.expected, result)

			if tt.shouldError {
				assert.True(t, parser.HasErrors())
				assert.Contains(t, parser.Errors(), tt.fieldName)
				assert.Contains(t, buf.String(), tt.fieldName)
				assert.Contains(t, buf.String(), "test-server")
			} else {
				assert.False(t, parser.HasErrors())
			}
		})
	}
}

func TestMultipleErrors(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	parser := NewParser(logger, "multi-error-server")

	parser.ParseFloat("invalid1", "field1")
	parser.ParseInt("invalid2", "field2")
	parser.ParseFloat("invalid3", "field3")

	assert.True(t, parser.HasErrors())
	assert.Len(t, parser.Errors(), 3)
	assert.Equal(t, []string{"field1", "field2", "field3"}, parser.Errors())
}

func TestNoErrors(t *testing.T) {
	logger := zerolog.New(nil)
	parser := NewParser(logger, "clean-server")

	parser.ParseFloat("3.14", "float_field")
	parser.ParseInt("42", "int_field")
	parser.ParseFloat("", "empty_float")
	parser.ParseInt("", "empty_int")

	assert.False(t, parser.HasErrors())
	assert.Empty(t, parser.Errors())
}
