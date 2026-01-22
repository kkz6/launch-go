package fiber

import (
	"github.com/gofiber/fiber/v2"
)

// Context key constants for storing and retrieving values from fiber.Ctx.Locals
const (
	KeyUserID   = "userID"
	KeyTeamID   = "teamID"
	KeyTeamRole = "teamRole"
	KeyUser     = "user"
	KeyTraceID  = "traceID"
)

// SetUserContext sets the user ID and optional user object in the request context.
func SetUserContext(c *fiber.Ctx, userID string, user interface{}) {
	c.Locals(KeyUserID, userID)
	if user != nil {
		c.Locals(KeyUser, user)
	}
}

// SetTeamContext sets the team ID and team role in the request context.
func SetTeamContext(c *fiber.Ctx, teamID, teamRole string) {
	c.Locals(KeyTeamID, teamID)
	c.Locals(KeyTeamRole, teamRole)
}

// GetTeamID safely extracts team ID from context.
// Returns an error if team ID is not found or empty.
func GetTeamID(c *fiber.Ctx) (string, error) {
	v, ok := c.Locals(KeyTeamID).(string)
	if !ok || v == "" {
		return "", Unauthorized("Team ID not found")
	}
	return v, nil
}

// MustGetTeamID extracts team ID or returns unauthorized error.
// Use only in routes protected by auth middleware.
func MustGetTeamID(c *fiber.Ctx) (string, error) {
	return GetTeamID(c)
}

// GetUserID safely extracts user ID from context.
// Returns an error if user ID is not found or empty.
func GetUserID(c *fiber.Ctx) (string, error) {
	v, ok := c.Locals(KeyUserID).(string)
	if !ok || v == "" {
		return "", Unauthorized("User ID not found")
	}
	return v, nil
}

// MustGetUserID extracts user ID or returns unauthorized error.
// Use only in routes protected by auth middleware.
func MustGetUserID(c *fiber.Ctx) (string, error) {
	return GetUserID(c)
}

// GetUser safely extracts full user object from context.
// Returns nil and error if user is not found.
func GetUser[T any](c *fiber.Ctx) (*T, error) {
	v, ok := c.Locals(KeyUser).(*T)
	if !ok || v == nil {
		return nil, Unauthorized("User not found")
	}
	return v, nil
}

// MustGetUser extracts user object or returns unauthorized error.
func MustGetUser[T any](c *fiber.Ctx) (*T, error) {
	return GetUser[T](c)
}

// GetTeamAndUserID safely extracts both team ID and user ID from context.
// This is a convenience function for handlers that need both values.
func GetTeamAndUserID(c *fiber.Ctx) (teamID, userID string, err error) {
	teamID, err = GetTeamID(c)
	if err != nil {
		return "", "", err
	}
	userID, err = GetUserID(c)
	if err != nil {
		return "", "", err
	}
	return teamID, userID, nil
}

// MustGetTeamAndUserID extracts both team ID and user ID, returning error if either is not found.
func MustGetTeamAndUserID(c *fiber.Ctx) (teamID, userID string, err error) {
	return GetTeamAndUserID(c)
}

// GetUserRole safely extracts team role from context.
// Returns empty string if team role is not found.
func GetUserRole(c *fiber.Ctx) string {
	v, ok := c.Locals(KeyTeamRole).(string)
	if !ok {
		return ""
	}
	return v
}

// MustGetTeamRole extracts team role from context.
// Returns "member" as default if team role is not found.
func MustGetTeamRole(c *fiber.Ctx) string {
	role := GetUserRole(c)
	if role == "" {
		return "member"
	}
	return role
}
