package metrics

import (
	"strconv"

	"github.com/rs/zerolog"
)

// Parser provides helper methods for parsing metric string values with consistent error logging.
type Parser struct {
	logger   zerolog.Logger
	serverID string
	errors   []string
}

// NewParser creates a new metrics parser with logging context.
func NewParser(logger zerolog.Logger, serverID string) *Parser {
	return &Parser{
		logger:   logger,
		serverID: serverID,
	}
}

// ParseFloat parses a string value to float64.
// Returns 0 if the value is empty or parsing fails.
// Logs a warning on parse failure.
func (p *Parser) ParseFloat(value, fieldName string) float64 {
	if value == "" {
		return 0
	}

	v, err := strconv.ParseFloat(value, 64)
	if err != nil {
		p.logger.Warn().
			Err(err).
			Str("server_id", p.serverID).
			Str("field", fieldName).
			Str("value", value).
			Msg("Failed to parse metric")
		p.errors = append(p.errors, fieldName)
		return 0
	}

	return v
}

// ParseInt parses a string value to int64.
// Returns 0 if the value is empty or parsing fails.
// Logs a warning on parse failure.
func (p *Parser) ParseInt(value, fieldName string) int64 {
	if value == "" {
		return 0
	}

	v, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		p.logger.Warn().
			Err(err).
			Str("server_id", p.serverID).
			Str("field", fieldName).
			Str("value", value).
			Msg("Failed to parse metric")
		p.errors = append(p.errors, fieldName)
		return 0
	}

	return v
}

// HasErrors returns true if any parsing errors occurred.
func (p *Parser) HasErrors() bool {
	return len(p.errors) > 0
}

// Errors returns the list of field names that failed to parse.
func (p *Parser) Errors() []string {
	return p.errors
}

// ErrorCount returns the number of parsing errors.
func (p *Parser) ErrorCount() int {
	return len(p.errors)
}
