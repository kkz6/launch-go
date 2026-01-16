package enums

import (
	"testing"
)

func TestPhpBinaryFromVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"php83", "php8.3"},
		{"php82", "php8.2"},
		{"php81", "php8.1"},
		{"php80", "php8.0"},
		{"php74", "php7.4"},
		{"8.3", "php8.3"},
		{"8.2", "php8.2"},
		{"", "php"},
	}

	for _, tt := range tests {
		result := PhpBinaryFromVersion(tt.input)
		if result != tt.expected {
			t.Errorf("PhpBinaryFromVersion(%q) = %q, want %q", tt.input, result, tt.expected)
		} else {
			t.Logf("PhpBinaryFromVersion(%q) = %q ✓", tt.input, result)
		}
	}
}

func TestPhpFpmServiceFromVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"php83", "php8.3-fpm"},
		{"php82", "php8.2-fpm"},
		{"8.3", "php8.3-fpm"},
		{"", ""},
	}

	for _, tt := range tests {
		result := PhpFpmServiceFromVersion(tt.input)
		if result != tt.expected {
			t.Errorf("PhpFpmServiceFromVersion(%q) = %q, want %q", tt.input, result, tt.expected)
		} else {
			t.Logf("PhpFpmServiceFromVersion(%q) = %q ✓", tt.input, result)
		}
	}
}
