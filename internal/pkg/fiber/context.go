package fiber

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/response"
)

// Context key constants for storing and retrieving values from fiber.Ctx.Locals
const (
	KeyUserID   = "userID"
	KeyTeamID   = "teamID"
	KeyTeamRole = "teamRole"
	KeyUser     = "user"
	KeyTraceID  = "traceID"
)

var (
	ErrTeamIDNotFound = errors.New("team ID not found in context")
	ErrUserIDNotFound = errors.New("user ID not found in context")
	ErrUserNotFound   = errors.New("user not found in context")
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
		return "", ErrTeamIDNotFound
	}
	return v, nil
}

// MustGetTeamID extracts team ID and sends error response if not found.
// Returns empty string and error response if team ID is not found.
// Use only in routes protected by auth middleware.
func MustGetTeamID(c *fiber.Ctx) (string, error) {
	v, err := GetTeamID(c)
	if err != nil {
		return "", response.Unauthorized(c, response.MsgUnauthorized)
	}
	return v, nil
}

// GetUserID safely extracts user ID from context.
// Returns an error if user ID is not found or empty.
func GetUserID(c *fiber.Ctx) (string, error) {
	v, ok := c.Locals(KeyUserID).(string)
	if !ok || v == "" {
		return "", ErrUserIDNotFound
	}
	return v, nil
}

// MustGetUserID extracts user ID and sends error response if not found.
// Returns empty string and error response if user ID is not found.
// Use only in routes protected by auth middleware.
func MustGetUserID(c *fiber.Ctx) (string, error) {
	v, err := GetUserID(c)
	if err != nil {
		return "", response.Unauthorized(c, response.MsgUnauthorized)
	}
	return v, nil
}

// GetUser safely extracts full user object from context.
// Returns nil and error if user is not found.
func GetUser[T any](c *fiber.Ctx) (*T, error) {
	v, ok := c.Locals(KeyUser).(*T)
	if !ok || v == nil {
		return nil, ErrUserNotFound
	}
	return v, nil
}

// MustGetUser extracts user object and sends error response if not found.
// Returns nil and error response if user is not found.
func MustGetUser[T any](c *fiber.Ctx) (*T, error) {
	v, err := GetUser[T](c)
	if err != nil {
		return nil, response.Unauthorized(c, response.MsgUnauthorized)
	}
	return v, nil
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

// MustGetTeamAndUserID extracts both team ID and user ID, sending error response if either is not found.
func MustGetTeamAndUserID(c *fiber.Ctx) (teamID, userID string, err error) {
	teamID, err = MustGetTeamID(c)
	if err != nil {
		return "", "", err
	}
	userID, err = MustGetUserID(c)
	if err != nil {
		return "", "", err
	}
	return teamID, userID, nil
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
