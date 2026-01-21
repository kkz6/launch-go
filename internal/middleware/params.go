package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/oklog/ulid/v2"
)

// ValidateULIDParams creates middleware that validates specified path parameters are valid ULIDs.
// This prevents empty strings or malformed IDs from reaching database queries.
//
// Note: For most cases, prefer using the handler-level helpers in internal/pkg/fiber/params.go
// (e.g., fiberctx.GetID, fiberctx.GetULIDParam) which provide the same validation
// but keep the logic closer to where the params are used.
//
// Usage:
//
//	router.Get("/servers/:id", middleware.ValidateULIDParams("id"), handler.Show)
func ValidateULIDParams(params ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		for _, param := range params {
			value := c.Params(param)
			if value == "" {
				return response.BadRequest(c, "Missing required parameter: "+param)
			}
			if _, err := ulid.Parse(value); err != nil {
				return response.BadRequest(c, "Invalid parameter format: "+param)
			}
		}
		return c.Next()
	}
}

// RequireParams creates middleware that ensures specified path parameters are non-empty.
// Use this for parameters that don't need to be ULIDs (e.g., slugs, names).
func RequireParams(params ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		for _, param := range params {
			if c.Params(param) == "" {
				return response.BadRequest(c, "Missing required parameter: "+param)
			}
		}
		return c.Next()
	}
}
