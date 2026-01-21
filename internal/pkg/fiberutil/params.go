package fiberutil

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// IntParamConfig configures how to parse an integer query parameter.
type IntParamConfig struct {
	Name    string
	Default int
	Min     int
	Max     int
}

// ParseIntParam parses an integer query parameter with bounds checking.
// Returns the default value if the parameter is missing or invalid.
// Clamps the value between Min and Max if they are set (non-zero).
func ParseIntParam(c *fiber.Ctx, cfg IntParamConfig) int {
	str := c.Query(cfg.Name)
	if str == "" {
		return cfg.Default
	}

	val, err := strconv.Atoi(str)
	if err != nil {
		return cfg.Default
	}

	if cfg.Min != 0 && val < cfg.Min {
		return cfg.Min
	}

	if cfg.Max != 0 && val > cfg.Max {
		return cfg.Max
	}

	return val
}

// ParseLimit parses a "limit" query parameter with a default and maximum value.
// If the parameter is missing or invalid, returns defaultVal.
// If the value exceeds maxVal, returns maxVal.
func ParseLimit(c *fiber.Ctx, defaultVal, maxVal int) int {
	return ParseIntParam(c, IntParamConfig{
		Name:    "limit",
		Default: defaultVal,
		Min:     1,
		Max:     maxVal,
	})
}

// ParseTail parses a "tail" query parameter for log-style endpoints.
// Default is 100, maximum is 10000.
func ParseTail(c *fiber.Ctx) int {
	return ParseIntParam(c, IntParamConfig{
		Name:    "tail",
		Default: 100,
		Min:     1,
		Max:     10000,
	})
}

// ParseInterval parses an "interval" query parameter for time-based data.
// Maximum is 60 (typically representing minutes or seconds).
func ParseInterval(c *fiber.Ctx, defaultVal int) int {
	return ParseIntParam(c, IntParamConfig{
		Name:    "interval",
		Default: defaultVal,
		Min:     1,
		Max:     60,
	})
}

// ParsePage parses a "page" query parameter for pagination.
// Default is 1, minimum is 1.
func ParsePage(c *fiber.Ctx) int {
	return ParseIntParam(c, IntParamConfig{
		Name:    "page",
		Default: 1,
		Min:     1,
		Max:     0, // no max
	})
}

// ParsePerPage parses a "per_page" query parameter for pagination.
// Default is 15, maximum is 100.
func ParsePerPage(c *fiber.Ctx) int {
	return ParseIntParam(c, IntParamConfig{
		Name:    "per_page",
		Default: 15,
		Min:     1,
		Max:     100,
	})
}

// ParseOffset parses an "offset" query parameter.
// Default is 0, minimum is 0.
func ParseOffset(c *fiber.Ctx) int {
	return ParseIntParam(c, IntParamConfig{
		Name:    "offset",
		Default: 0,
		Min:     0,
		Max:     0, // no max
	})
}

// PaginationParams holds parsed pagination parameters.
type PaginationParams struct {
	Page    int
	PerPage int
	Offset  int
}

// ParsePagination parses standard pagination parameters from the request.
func ParsePagination(c *fiber.Ctx) PaginationParams {
	page := ParsePage(c)
	perPage := ParsePerPage(c)

	return PaginationParams{
		Page:    page,
		PerPage: perPage,
		Offset:  (page - 1) * perPage,
	}
}

// ParseBool parses a boolean query parameter.
// Returns the default value if the parameter is missing.
// Accepts "true", "1", "yes" as true values.
func ParseBool(c *fiber.Ctx, name string, defaultVal bool) bool {
	str := c.Query(name)
	if str == "" {
		return defaultVal
	}

	switch str {
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	default:
		return defaultVal
	}
}

// ParseString parses a string query parameter.
// Returns the default value if the parameter is missing.
func ParseString(c *fiber.Ctx, name, defaultVal string) string {
	str := c.Query(name)
	if str == "" {
		return defaultVal
	}
	return str
}

// ParseIntValue parses an integer from a raw string value with bounds checking.
// This is useful for non-Fiber contexts (e.g., WebSocket handlers).
// Returns the default value if the string is empty or invalid.
// Clamps the value between minVal and maxVal if they are set (non-zero).
func ParseIntValue(str string, defaultVal, minVal, maxVal int) int {
	if str == "" {
		return defaultVal
	}

	val, err := strconv.Atoi(str)
	if err != nil {
		return defaultVal
	}

	if minVal != 0 && val < minVal {
		return minVal
	}

	if maxVal != 0 && val > maxVal {
		return maxVal
	}

	return val
}

// ParseTailValue parses a "tail" string value for log-style endpoints.
// Default is 100, minimum is 1, maximum is 10000.
func ParseTailValue(str string) int {
	return ParseIntValue(str, 100, 1, 10000)
}

// ParseIntervalValue parses an "interval" string value.
// Clamps between min (default 1) and max (default 60).
func ParseIntervalValue(str string, defaultVal int) int {
	return ParseIntValue(str, defaultVal, 1, 60)
}
