package dto

import (
	"fmt"
	"time"
)

// DisplayTimeFormat is the standard format for displaying dates to users (e.g., "Jan 2, 2006").
const DisplayTimeFormat = "Jan 2, 2006"

// DisplayDateTimeFormat is the standard format for displaying dates with time to users (e.g., "Jan 2, 2006 3:04 PM").
const DisplayDateTimeFormat = "Jan 2, 2006 3:04 PM"

// FormatTime converts a time pointer to an RFC3339 string pointer.
// Returns nil if the input is nil.
func FormatTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}

// FormatTimeValue converts a time value to an RFC3339 string.
func FormatTimeValue(t time.Time) string {
	return t.Format(time.RFC3339)
}

// FormatTimeOrEmpty returns an RFC3339 string or empty string if nil.
func FormatTimeOrEmpty(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

// ParseTime parses an RFC3339 string to a time value.
// Returns zero time and error if parsing fails.
func ParseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

// ParseTimePtr parses an RFC3339 string pointer to a time pointer.
// Returns nil if the input is nil or empty.
func ParseTimePtr(s *string) *time.Time {
	if s == nil || *s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil
	}
	return &t
}

// Now returns the current time formatted as RFC3339.
func Now() string {
	return time.Now().Format(time.RFC3339)
}

// NowPtr returns a pointer to the current time formatted as RFC3339.
func NowPtr() *string {
	s := time.Now().Format(time.RFC3339)
	return &s
}

// TimeAgo returns a human-readable string representing the time elapsed.
// For example: "2 hours ago", "3 days ago", "1 month ago".
func TimeAgo(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return formatDuration(mins, "minute")
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return formatDuration(hours, "hour")
	case diff < 30*24*time.Hour:
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return formatDuration(days, "day")
	case diff < 365*24*time.Hour:
		months := int(diff.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return formatDuration(months, "month")
	default:
		years := int(diff.Hours() / 24 / 365)
		if years == 1 {
			return "1 year ago"
		}
		return formatDuration(years, "year")
	}
}

// TimeAgoPtr returns a human-readable string for a time pointer.
// Returns empty string if nil.
func TimeAgoPtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return TimeAgo(*t)
}

// FormatDisplayTime converts a time pointer to a display-formatted string pointer (e.g., "Jan 2, 2006").
// Returns nil if the input is nil.
func FormatDisplayTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(DisplayTimeFormat)
	return &s
}

// FormatDisplayTimeValue converts a time value to a display-formatted string (e.g., "Jan 2, 2006").
func FormatDisplayTimeValue(t time.Time) string {
	return t.Format(DisplayTimeFormat)
}

// FormatDisplayTimeOrEmpty returns a display-formatted string or empty string if nil.
func FormatDisplayTimeOrEmpty(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(DisplayTimeFormat)
}

// FormatDisplayDateTime converts a time pointer to a display-formatted string with time (e.g., "Jan 2, 2006 3:04 PM").
// Returns nil if the input is nil.
func FormatDisplayDateTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(DisplayDateTimeFormat)
	return &s
}

func formatDuration(n int, unit string) string {
	return fmt.Sprintf("%d %ss ago", n, unit)
}
