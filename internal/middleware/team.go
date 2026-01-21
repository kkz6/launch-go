package middleware

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/cache"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// Role hierarchy - higher number = more permissions
var roleHierarchy = map[string]int{
	"owner":  4,
	"admin":  3,
	"editor": 2,
	"member": 1,
}

// teamMiddleware holds the shared state for team middleware
var teamMiddleware struct {
	cache *cache.TeamMembershipCache
}

// InitTeamMiddleware initializes the team middleware with required dependencies.
// Call this during application bootstrap before routes are registered.
func InitTeamMiddleware(membershipCache *cache.TeamMembershipCache) {
	teamMiddleware.cache = membershipCache
}

// teamScopeResult holds the result of team scope validation.
type teamScopeResult struct {
	teamID string
	role   string
	err    error
}

// validateTeamScope validates team membership for the current request.
// Returns the validated team context or an error if validation fails.
func validateTeamScope(c *fiber.Ctx) teamScopeResult {
	teamID := c.Get("X-Team-ID")
	if teamID == "" {
		return teamScopeResult{err: fiber.NewError(fiber.StatusBadRequest, "X-Team-ID header is required")}
	}

	userID, ok := c.Locals(fiberctx.KeyUserID).(string)
	if !ok || userID == "" {
		return teamScopeResult{err: fiber.NewError(fiber.StatusUnauthorized, "Authentication required")}
	}

	if teamMiddleware.cache == nil {
		return teamScopeResult{teamID: teamID, role: ""}
	}

	membership, err := teamMiddleware.cache.GetMembership(c.Context(), userID, teamID)
	if err != nil {
		return teamScopeResult{err: fiber.NewError(fiber.StatusInternalServerError, "Failed to validate team membership")}
	}

	if !membership.IsMember {
		return teamScopeResult{err: fiber.NewError(fiber.StatusForbidden, "You are not a member of this team")}
	}

	return teamScopeResult{teamID: teamID, role: membership.Role}
}

// TeamScope middleware reads X-Team-ID header and validates team membership.
// It sets teamID and teamRole in request context (c.Locals).
//
// This should be used after Auth middleware for routes that require team context.
//
// Usage:
//
//	router.Group("/servers", middleware.Auth(secret), middleware.TeamScope())
func TeamScope() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if existingID, ok := c.Locals(fiberctx.KeyTeamID).(string); ok && existingID != "" {
			return c.Next()
		}

		result := validateTeamScope(c)
		if result.err != nil {
			fiberErr, ok := result.err.(*fiber.Error)
			if !ok {
				return response.Error(c, fiber.StatusInternalServerError, result.err.Error())
			}

			switch fiberErr.Code {
			case fiber.StatusBadRequest:
				return response.Error(c, fiber.StatusBadRequest, fiberErr.Message)
			case fiber.StatusUnauthorized:
				return response.Unauthorized(c, fiberErr.Message)
			case fiber.StatusForbidden:
				return response.Forbidden(c, fiberErr.Message)
			default:
				return response.Error(c, fiberErr.Code, fiberErr.Message)
			}
		}

		fiberctx.SetTeamContext(c, result.teamID, result.role)

		return c.Next()
	}
}

// OptionalTeamScope is like TeamScope but doesn't require X-Team-ID header.
// If header is provided and valid, it sets the team context.
// If header is missing or invalid, it continues without team context.
//
// Useful for endpoints that work with or without team context.
func OptionalTeamScope() fiber.Handler {
	return func(c *fiber.Ctx) error {
		teamID := c.Get("X-Team-ID")
		if teamID == "" {
			return c.Next()
		}

		result := validateTeamScope(c)
		if result.err != nil {
			return c.Next()
		}

		fiberctx.SetTeamContext(c, result.teamID, result.role)

		return c.Next()
	}
}

// RequireRole middleware ensures the user has at least the specified role.
// Must be used after TeamScope middleware.
//
// Roles are hierarchical: owner > admin > editor > member
//
// Usage:
//
//	router.Delete("/:id", middleware.RequireRole("admin"), handler.Delete)
func RequireRole(minRole string) fiber.Handler {
	minLevel, validRole := roleHierarchy[minRole]
	if !validRole {
		// Invalid role specified - fail closed
		return func(c *fiber.Ctx) error {
			return response.Forbidden(c, "Invalid role configuration")
		}
	}

	return func(c *fiber.Ctx) error {
		teamRole, ok := c.Locals(fiberctx.KeyTeamRole).(string)
		if !ok || teamRole == "" {
			return response.Forbidden(c, "Team context required")
		}

		userLevel, ok := roleHierarchy[teamRole]
		if !ok || userLevel < minLevel {
			return response.Forbidden(c, "Insufficient permissions")
		}

		return c.Next()
	}
}

// Deprecated: Use InitTeamMiddleware instead
func SetTeamScopeMembershipCache(c *cache.TeamMembershipCache) {
	InitTeamMiddleware(c)
}

// Deprecated: Use TeamScope instead
func TeamContext(membershipCache *cache.TeamMembershipCache) fiber.Handler {
	return func(c *fiber.Ctx) error {
		teamID := c.Get("X-Team-ID")
		if teamID == "" {
			return response.Error(c, fiber.StatusBadRequest, "X-Team-ID header is required")
		}

		userID, ok := c.Locals(fiberctx.KeyUserID).(string)
		if !ok || userID == "" {
			return response.Unauthorized(c, "Authentication required")
		}

		membership, err := membershipCache.GetMembership(c.Context(), userID, teamID)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to validate team membership")
		}

		if !membership.IsMember {
			return response.Forbidden(c, "You are not a member of this team")
		}

		fiberctx.SetTeamContext(c, teamID, membership.Role)

		return c.Next()
	}
}

// Deprecated: Use RequireRole instead
func RequireTeamRole(minRole string) fiber.Handler {
	return RequireRole(minRole)
}
