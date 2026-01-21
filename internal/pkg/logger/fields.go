package logger

import "github.com/rs/zerolog"

// ApplyFields adds key-value pairs to a zerolog event.
// Fields should be provided as alternating key, value pairs.
// If a key is not a string, the pair is skipped.
//
// Example:
//
//	event := logger.Info()
//	ApplyFields(event, "user_id", "123", "action", "login").Msg("User logged in")
func ApplyFields(event *zerolog.Event, fields ...any) *zerolog.Event {
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			event = event.Interface(key, fields[i+1])
		}
	}
	return event
}

// LogWithFields logs a message with variadic fields to the given event.
// This is a convenience function that combines ApplyFields and Msg.
//
// Example:
//
//	LogWithFields(logger.Info(), "Processing request", "user_id", "123", "path", "/api/v1/users")
func LogWithFields(event *zerolog.Event, msg string, fields ...any) {
	ApplyFields(event, fields...).Msg(msg)
}

// Info logs an info message with variadic fields.
//
// Example:
//
//	logger.Info(log, "User created", "user_id", "123", "email", "user@example.com")
func Info(log *zerolog.Logger, msg string, fields ...any) {
	if log == nil {
		return
	}
	LogWithFields(log.Info(), msg, fields...)
}

// Warn logs a warning message with variadic fields.
//
// Example:
//
//	logger.Warn(log, "Rate limit approaching", "user_id", "123", "requests", 95)
func Warn(log *zerolog.Logger, msg string, fields ...any) {
	if log == nil {
		return
	}
	LogWithFields(log.Warn(), msg, fields...)
}

// Error logs an error message with variadic fields.
// The error is automatically added to the log event.
//
// Example:
//
//	logger.Error(log, err, "Failed to create user", "email", "user@example.com")
func Error(log *zerolog.Logger, err error, msg string, fields ...any) {
	if log == nil {
		return
	}
	event := log.Error()
	if err != nil {
		event = event.Err(err)
	}
	LogWithFields(event, msg, fields...)
}

// Debug logs a debug message with variadic fields.
//
// Example:
//
//	logger.Debug(log, "Processing item", "item_id", "456", "queue", "default")
func Debug(log *zerolog.Logger, msg string, fields ...any) {
	if log == nil {
		return
	}
	LogWithFields(log.Debug(), msg, fields...)
}
