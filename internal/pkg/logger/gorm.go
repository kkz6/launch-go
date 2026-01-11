package logger

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Icons for SQL operations
const (
	iconSelect   = "🔎"
	iconInsert   = "➕"
	iconUpdate   = "📝"
	iconDelete   = "🗑️ "
	iconQuery    = "💾"
	iconSlow     = "🐢"
	iconSQLError = "💥"
)

// GormLogger is a custom GORM logger that uses zerolog
type GormLogger struct {
	logger        *zerolog.Logger
	logLevel      gormlogger.LogLevel
	slowThreshold time.Duration
}

// NewGormLogger creates a new GORM logger
func NewGormLogger(logger *zerolog.Logger) *GormLogger {
	return &GormLogger{
		logger:        logger,
		logLevel:      gormlogger.Info,
		slowThreshold: 200 * time.Millisecond, // Log slow queries > 200ms
	}
}

// NewGormLoggerWithConfig creates a GORM logger with custom configuration
func NewGormLoggerWithConfig(logger *zerolog.Logger, level gormlogger.LogLevel, slowThreshold time.Duration) *GormLogger {
	return &GormLogger{
		logger:        logger,
		logLevel:      level,
		slowThreshold: slowThreshold,
	}
}

// LogMode sets the log level
func (l *GormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	newLogger := *l
	newLogger.logLevel = level
	return &newLogger
}

// Info logs info level messages
func (l *GormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.logLevel >= gormlogger.Info {
		l.logger.Info().Msgf(msg, data...)
	}
}

// Warn logs warn level messages
func (l *GormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.logLevel >= gormlogger.Warn {
		l.logger.Warn().Msgf(msg, data...)
	}
}

// Error logs error level messages
func (l *GormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.logLevel >= gormlogger.Error {
		l.logger.Error().Msgf(msg, data...)
	}
}

// Trace logs SQL queries with beautiful formatting
func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if l.logLevel <= gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	// Get the caller information (skip GORM internals)
	caller := l.getCaller()

	// Determine the operation icon
	icon := l.getOperationIcon(sql)

	// Format the SQL for display (truncate if too long)
	displaySQL := l.formatSQL(sql)

	switch {
	case err != nil && !errors.Is(err, gorm.ErrRecordNotFound):
		// Error occurred
		l.logger.Error().
			Str("caller", caller).
			Str("duration", FormatDuration(elapsed)).
			Int64("rows", rows).
			Err(err).
			Msgf("%s %s", iconSQLError, displaySQL)

	case elapsed > l.slowThreshold && l.slowThreshold != 0:
		// Slow query
		l.logger.Warn().
			Str("caller", caller).
			Str("duration", FormatDuration(elapsed)).
			Int64("rows", rows).
			Str("threshold", FormatDuration(l.slowThreshold)).
			Msgf("%s SLOW %s %s", iconSlow, icon, displaySQL)

	case l.logLevel >= gormlogger.Info:
		// Normal query
		l.logger.Debug().
			Str("caller", caller).
			Str("duration", FormatDuration(elapsed)).
			Int64("rows", rows).
			Msgf("%s %s", icon, displaySQL)
	}
}

// ParamsFilter allows filtering of sensitive parameters
func (l *GormLogger) ParamsFilter(ctx context.Context, sql string, params ...interface{}) (string, []interface{}) {
	return sql, params
}

// getOperationIcon returns an icon based on the SQL operation
func (l *GormLogger) getOperationIcon(sql string) string {
	upperSQL := strings.ToUpper(strings.TrimSpace(sql))

	switch {
	case strings.HasPrefix(upperSQL, "SELECT"):
		return iconSelect
	case strings.HasPrefix(upperSQL, "INSERT"):
		return iconInsert
	case strings.HasPrefix(upperSQL, "UPDATE"):
		return iconUpdate
	case strings.HasPrefix(upperSQL, "DELETE"):
		return iconDelete
	default:
		return iconQuery
	}
}

// formatSQL formats SQL for display
func (l *GormLogger) formatSQL(sql string) string {
	// Clean up whitespace
	sql = strings.ReplaceAll(sql, "\n", " ")
	sql = strings.ReplaceAll(sql, "\t", " ")

	// Collapse multiple spaces
	for strings.Contains(sql, "  ") {
		sql = strings.ReplaceAll(sql, "  ", " ")
	}

	sql = strings.TrimSpace(sql)

	// Truncate if too long
	maxLen := 120
	if len(sql) > maxLen {
		sql = sql[:maxLen] + "..."
	}

	return sql
}

// getCaller returns a shortened caller path
func (l *GormLogger) getCaller() string {
	// Skip: runtime, this function, GORM internals
	for i := 4; i < 15; i++ {
		_, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}

		// Skip GORM internal files
		if strings.Contains(file, "gorm.io") {
			continue
		}

		// Return shortened path
		return fmt.Sprintf("%s:%d", filepath.Base(file), line)
	}

	return "unknown"
}
