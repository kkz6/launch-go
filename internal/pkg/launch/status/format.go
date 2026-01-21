// Package status provides utilities for formatting server status information.
package status

import "fmt"

// FormatBytes formats a byte count into a human-readable string.
// Examples: "1.5 KB", "2.3 MB", "1.0 GB"
func FormatBytes(bytes int64) string {
	if bytes < 0 {
		return "0 B"
	}

	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
		tb = gb * 1024
	)

	switch {
	case bytes >= tb:
		return fmt.Sprintf("%.1f TB", float64(bytes)/float64(tb))
	case bytes >= gb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// FormatUptime formats a duration in seconds into a human-readable string.
// Examples: "5d 12h 30m", "12h 30m", "30m", "45s"
func FormatUptime(seconds int64) string {
	if seconds < 0 {
		return "0s"
	}

	const (
		secondsPerMinute = 60
		secondsPerHour   = 60 * secondsPerMinute
		secondsPerDay    = 24 * secondsPerHour
	)

	days := seconds / secondsPerDay
	seconds %= secondsPerDay

	hours := seconds / secondsPerHour
	seconds %= secondsPerHour

	minutes := seconds / secondsPerMinute
	secs := seconds % secondsPerMinute

	result := ""

	if days > 0 {
		result += fmt.Sprintf("%dd ", days)
	}

	if hours > 0 || days > 0 {
		result += fmt.Sprintf("%dh ", hours)
	}

	if minutes > 0 || hours > 0 || days > 0 {
		result += fmt.Sprintf("%dm", minutes)
	} else if secs > 0 {
		result += fmt.Sprintf("%ds", secs)
	} else {
		result = "0s"
	}

	return result
}

// FormatPercentage calculates and formats a percentage from used and total values.
// Returns "0.0%" if total is zero or negative.
// Examples: "75.5%", "100.0%", "0.0%"
func FormatPercentage(used, total float64) string {
	if total <= 0 {
		return "0.0%"
	}

	percentage := (used / total) * 100

	if percentage < 0 {
		percentage = 0
	}

	if percentage > 100 {
		percentage = 100
	}

	return fmt.Sprintf("%.1f%%", percentage)
}

// ParseBytesString parses a string containing a byte count and formats it.
// Useful for parsing systemctl output like "MemoryCurrent=123456".
// Returns the original string if parsing fails.
func ParseBytesString(byteStr string) string {
	var bytes int64
	_, err := fmt.Sscanf(byteStr, "%d", &bytes)
	if err != nil {
		return byteStr
	}
	return FormatBytes(bytes)
}
