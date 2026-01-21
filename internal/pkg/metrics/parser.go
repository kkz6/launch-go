// Package metrics provides utilities for parsing metric values from server responses.
package metrics

import (
	"strconv"
	"strings"

	"github.com/rs/zerolog"
)

// Parser handles parsing of metric values with error tracking.
type Parser struct {
	logger   zerolog.Logger
	serverID string
	errors   []string
}

// NewParser creates a new Parser instance for parsing metric values.
func NewParser(logger zerolog.Logger, serverID string) *Parser {
	return &Parser{
		logger:   logger,
		serverID: serverID,
		errors:   make([]string, 0),
	}
}

// ParseFloat parses a string value to float64.
// Returns 0 for empty strings without logging an error.
// Logs a warning and tracks the error if parsing fails.
func (p *Parser) ParseFloat(value, fieldName string) float64 {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0
	}

	result, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		p.logError(fieldName, value, err)
		return 0
	}

	return result
}

// ParseInt parses a string value to int64.
// Returns 0 for empty strings without logging an error.
// Logs a warning and tracks the error if parsing fails.
func (p *Parser) ParseInt(value, fieldName string) int64 {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0
	}

	result, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		p.logError(fieldName, value, err)
		return 0
	}

	return result
}

// HasErrors returns true if any parsing errors have occurred.
func (p *Parser) HasErrors() bool {
	return len(p.errors) > 0
}

// Errors returns a slice of all fields that had parsing errors.
func (p *Parser) Errors() []string {
	return p.errors
}

// logError logs a warning and tracks the field that had an error.
func (p *Parser) logError(fieldName, value string, err error) {
	p.errors = append(p.errors, fieldName)
	p.logger.Warn().
		Str("server_id", p.serverID).
		Str("field", fieldName).
		Str("value", value).
		Err(err).
		Msg("failed to parse metric value")
}
