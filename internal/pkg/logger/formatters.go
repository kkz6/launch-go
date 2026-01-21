package logger

import (
	"fmt"
	"time"
)

// ANSI color codes
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorWhite  = "\033[37m"
	ColorGray   = "\033[90m"
)

// HTTP method icons
var MethodIcons = map[string]string{
	"GET":     "📥",
	"POST":    "📤",
	"PUT":     "📝",
	"PATCH":   "🔧",
	"DELETE":  "🗑️ ",
	"OPTIONS": "⚙️ ",
	"HEAD":    "📋",
}

// GetMethodIcon returns an icon for the given HTTP method
func GetMethodIcon(method string) string {
	if icon, ok := MethodIcons[method]; ok {
		return icon
	}
	return "📡"
}

// GetStatusColor returns the ANSI color code for a status code
func GetStatusColor(status int) string {
	switch {
	case status >= 500:
		return ColorRed
	case status >= 400:
		return ColorYellow
	case status >= 300:
		return ColorCyan
	case status >= 200:
		return ColorGreen
	default:
		return ColorWhite
	}
}

// GetStatusIcon returns an icon for a status code
func GetStatusIcon(status int) string {
	switch {
	case status >= 500:
		return "💥"
	case status >= 400:
		return "⚠️ "
	case status >= 300:
		return "↪️ "
	case status >= 200:
		return "✅"
	default:
		return "❓"
	}
}

// FormatDurationCompact formats duration in a compact form
func FormatDurationCompact(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%.1fm", d.Minutes())
}

// Colorize wraps text in ANSI color codes
func Colorize(text, color string) string {
	return color + text + ColorReset
}

// ColorizeStatus returns a colorized status code string
func ColorizeStatus(status int) string {
	return Colorize(fmt.Sprintf("%d", status), GetStatusColor(status))
}

// LogLevel represents a logging level
type LogLevel string

const (
	LogLevelDebug LogLevel = "DEBUG"
	LogLevelInfo  LogLevel = "INFO"
	LogLevelWarn  LogLevel = "WARN"
	LogLevelError LogLevel = "ERROR"
	LogLevelFatal LogLevel = "FATAL"
)

// GetLevelColor returns the color for a log level
func GetLevelColor(level LogLevel) string {
	switch level {
	case LogLevelDebug:
		return ColorGray
	case LogLevelInfo:
		return ColorGreen
	case LogLevelWarn:
		return ColorYellow
	case LogLevelError:
		return ColorRed
	case LogLevelFatal:
		return ColorPurple
	default:
		return ColorWhite
	}
}

// ColorizeLeve returns a colorized log level string
func ColorizeLevel(level LogLevel) string {
	return Colorize(string(level), GetLevelColor(level))
}

// TruncateString truncates a string to max length, adding ellipsis if truncated
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
