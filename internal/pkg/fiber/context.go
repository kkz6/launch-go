package fiber

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/response"
)

var (
	ErrTeamIDNotFound = errors.New("team ID not found in context")
	ErrUserIDNotFound = errors.New("user ID not found in context")
	ErrUserNotFound   = errors.New("user not found in context")
)

// GetTeamID safely extracts team ID from context.
// Returns an error if team ID is not found or empty.
func GetTeamID(c *fiber.Ctx) (string, error) {
	v, ok := c.Locals("teamID").(string)
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
	v, ok := c.Locals("userID").(string)
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
	v, ok := c.Locals("user").(*T)
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
