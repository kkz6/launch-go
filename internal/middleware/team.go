package middleware

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/cache"
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
		// Skip if team context already set (allows chaining with other team middleware)
		if existingID, ok := c.Locals("teamID").(string); ok && existingID != "" {
			return c.Next()
		}

		teamID := c.Get("X-Team-ID")
		if teamID == "" {
			return response.Error(c, fiber.StatusBadRequest, "X-Team-ID header is required")
		}

		userID, ok := c.Locals("userID").(string)
		if !ok || userID == "" {
			return response.Unauthorized(c, "Authentication required")
		}

		// Validate membership
		if teamMiddleware.cache == nil {
			// No cache configured - just set the team ID without validation
			// This should only happen in tests or misconfigured environments
			c.Locals("teamID", teamID)
			return c.Next()
		}

		membership, err := teamMiddleware.cache.GetMembership(c.Context(), userID, teamID)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to validate team membership")
		}

		if !membership.IsMember {
			return response.Forbidden(c, "You are not a member of this team")
		}

		c.Locals("teamID", teamID)
		c.Locals("teamRole", membership.Role)

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

		userID, ok := c.Locals("userID").(string)
		if !ok || userID == "" {
			return c.Next()
		}

		if teamMiddleware.cache == nil {
			c.Locals("teamID", teamID)
			return c.Next()
		}

		membership, err := teamMiddleware.cache.GetMembership(c.Context(), userID, teamID)
		if err != nil || !membership.IsMember {
			return c.Next()
		}

		c.Locals("teamID", teamID)
		c.Locals("teamRole", membership.Role)

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
		teamRole, ok := c.Locals("teamRole").(string)
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

		userID, ok := c.Locals("userID").(string)
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

		c.Locals("teamID", teamID)
		c.Locals("teamRole", membership.Role)

		return c.Next()
	}
}

// Deprecated: Use RequireRole instead
func RequireTeamRole(minRole string) fiber.Handler {
	return RequireRole(minRole)
}
