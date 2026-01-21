package dto

import (
	"testing"
	"time"
)

func TestFormatTime(t *testing.T) {
	tests := []struct {
		name     string
		input    *time.Time
		wantNil  bool
		validate func(string) bool
	}{
		{
			name:    "nil input",
			input:   nil,
			wantNil: true,
		},
		{
			name:    "valid time",
			input:   timePtr(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)),
			wantNil: false,
			validate: func(s string) bool {
				return s == "2024-01-15T10:30:00Z"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTime(tt.input)
			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil, got %v", *result)
				}
			} else {
				if result == nil {
					t.Error("expected non-nil result")
				} else if !tt.validate(*result) {
					t.Errorf("unexpected result: %s", *result)
				}
			}
		})
	}
}

func TestFormatTimeValue(t *testing.T) {
	input := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	result := FormatTimeValue(input)
	expected := "2024-01-15T10:30:00Z"
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestFormatTimeOrEmpty(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		result := FormatTimeOrEmpty(nil)
		if result != "" {
			t.Errorf("expected empty string, got %s", result)
		}
	})

	t.Run("valid time", func(t *testing.T) {
		input := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		result := FormatTimeOrEmpty(&input)
		expected := "2024-01-15T10:30:00Z"
		if result != expected {
			t.Errorf("expected %s, got %s", expected, result)
		}
	})
}

func TestParseTime(t *testing.T) {
	t.Run("valid RFC3339", func(t *testing.T) {
		result, err := ParseTime("2024-01-15T10:30:00Z")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Year() != 2024 || result.Month() != 1 || result.Day() != 15 {
			t.Errorf("unexpected date: %v", result)
		}
	})

	t.Run("invalid format", func(t *testing.T) {
		_, err := ParseTime("not-a-date")
		if err == nil {
			t.Error("expected error for invalid date")
		}
	})
}

func TestParseTimePtr(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		result := ParseTimePtr(nil)
		if result != nil {
			t.Error("expected nil result")
		}
	})

	t.Run("empty string", func(t *testing.T) {
		s := ""
		result := ParseTimePtr(&s)
		if result != nil {
			t.Error("expected nil result for empty string")
		}
	})

	t.Run("valid RFC3339", func(t *testing.T) {
		s := "2024-01-15T10:30:00Z"
		result := ParseTimePtr(&s)
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.Year() != 2024 {
			t.Errorf("unexpected year: %d", result.Year())
		}
	})

	t.Run("invalid format", func(t *testing.T) {
		s := "not-a-date"
		result := ParseTimePtr(&s)
		if result != nil {
			t.Error("expected nil result for invalid date")
		}
	})
}

func TestNow(t *testing.T) {
	result := Now()
	if result == "" {
		t.Error("expected non-empty result")
	}
	// Verify it parses correctly
	_, err := time.Parse(time.RFC3339, result)
	if err != nil {
		t.Errorf("result is not valid RFC3339: %v", err)
	}
}

func TestNowPtr(t *testing.T) {
	result := NowPtr()
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// Verify it parses correctly
	_, err := time.Parse(time.RFC3339, *result)
	if err != nil {
		t.Errorf("result is not valid RFC3339: %v", err)
	}
}

func TestTimeAgo(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"just now", 30 * time.Second, "just now"},
		{"1 minute", 1 * time.Minute, "1 minute ago"},
		{"5 minutes", 5 * time.Minute, "5 minutes ago"},
		{"1 hour", 1 * time.Hour, "1 hour ago"},
		{"3 hours", 3 * time.Hour, "3 hours ago"},
		{"1 day", 24 * time.Hour, "1 day ago"},
		{"5 days", 5 * 24 * time.Hour, "5 days ago"},
		{"1 month", 35 * 24 * time.Hour, "1 month ago"},
		{"3 months", 100 * 24 * time.Hour, "3 months ago"},
		{"1 year", 400 * 24 * time.Hour, "1 year ago"},
		{"2 years", 800 * 24 * time.Hour, "2 years ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := time.Now().Add(-tt.duration)
			result := TimeAgo(input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestTimeAgoPtr(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		result := TimeAgoPtr(nil)
		if result != "" {
			t.Errorf("expected empty string, got %s", result)
		}
	})

	t.Run("valid time", func(t *testing.T) {
		input := time.Now().Add(-5 * time.Minute)
		result := TimeAgoPtr(&input)
		if result != "5 minutes ago" {
			t.Errorf("expected '5 minutes ago', got %s", result)
		}
	})
}

func timePtr(t time.Time) *time.Time {
	return &t
}
