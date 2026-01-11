package logger

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Color codes for terminal output
const (
	colorReset   = "\033[0m"
	colorRed     = "\033[31m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorMagenta = "\033[35m"
	colorCyan    = "\033[36m"
	colorWhite   = "\033[37m"
	colorGray    = "\033[90m"
	colorBold    = "\033[1m"
	colorDim     = "\033[2m"
)

// Icons for different log levels
const (
	iconDebug = "🔍"
	iconInfo  = "✨"
	iconWarn  = "⚠️ "
	iconError = "❌"
	iconFatal = "💀"
	iconPanic = "🔥"
	iconTrace = "📍"
)

// New creates a new logger instance with beautiful console output
func New(env string) *zerolog.Logger {
	var logger zerolog.Logger

	if env == "development" {
		output := zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: "15:04:05",
			NoColor:    false,
			FormatLevel: func(i interface{}) string {
				level := strings.ToUpper(fmt.Sprintf("%s", i))
				switch level {
				case "DEBUG":
					return fmt.Sprintf("%s%s DBG%s", colorBlue, iconDebug, colorReset)
				case "INFO":
					return fmt.Sprintf("%s%s INF%s", colorGreen, iconInfo, colorReset)
				case "WARN":
					return fmt.Sprintf("%s%s WRN%s", colorYellow, iconWarn, colorReset)
				case "ERROR":
					return fmt.Sprintf("%s%s ERR%s", colorRed, iconError, colorReset)
				case "FATAL":
					return fmt.Sprintf("%s%s%s FTL%s", colorBold, colorRed, iconFatal, colorReset)
				case "PANIC":
					return fmt.Sprintf("%s%s%s PNC%s", colorBold, colorRed, iconPanic, colorReset)
				case "TRACE":
					return fmt.Sprintf("%s%s TRC%s", colorGray, iconTrace, colorReset)
				default:
					return fmt.Sprintf("%s%s%s", colorGray, level, colorReset)
				}
			},
			FormatMessage: func(i interface{}) string {
				return fmt.Sprintf("%s%s%s", colorWhite, i, colorReset)
			},
			FormatFieldName: func(i interface{}) string {
				return fmt.Sprintf("%s%s%s=", colorCyan, i, colorReset)
			},
			FormatFieldValue: func(i interface{}) string {
				return fmt.Sprintf("%s%s%s", colorMagenta, i, colorReset)
			},
			FormatTimestamp: func(i interface{}) string {
				return fmt.Sprintf("%s%s%s", colorGray, i, colorReset)
			},
			FormatCaller: func(i interface{}) string {
				caller := fmt.Sprintf("%s", i)
				// Shorten the path for readability
				parts := strings.Split(caller, "/")
				if len(parts) > 2 {
					caller = strings.Join(parts[len(parts)-2:], "/")
				}
				return fmt.Sprintf("%s%s%s", colorDim, caller, colorReset)
			},
			PartsOrder: []string{
				zerolog.TimestampFieldName,
				zerolog.LevelFieldName,
				zerolog.CallerFieldName,
				zerolog.MessageFieldName,
			},
		}
		logger = zerolog.New(output).With().Timestamp().Caller().Logger()
	} else {
		// Production: JSON logging for structured log aggregation
		logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	}

	return &logger
}

// NewWithConfig creates a logger with additional configuration options
func NewWithConfig(env string, debug bool) *zerolog.Logger {
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else if env == "production" {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	return New(env)
}

// FormatDuration formats a duration for display
func FormatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%.2fµs", float64(d.Microseconds()))
	}
	if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d.Microseconds())/1000)
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

// FormatBytes formats bytes for display
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
