package middleware

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/cache"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// TeamContext middleware reads X-Team-ID header and validates team membership.
// This should be used after Auth middleware for routes that require team context.
//
// The middleware:
// 1. Reads X-Team-ID from request header
// 2. Validates the authenticated user is a member of the team (with caching)
// 3. Sets teamID and teamRole in request context (c.Locals)
//
// Usage:
//
//	router.Use(middleware.Auth(jwtSecret))
//	router.Use(middleware.TeamContext(membershipCache))
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

		// Validate membership (with caching)
		membership, err := membershipCache.GetMembership(c.Context(), userID, teamID)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to validate team membership")
		}

		if !membership.IsMember {
			return response.Forbidden(c, "You are not a member of this team")
		}

		// Set team context
		c.Locals("teamID", teamID)
		c.Locals("teamRole", membership.Role)

		return c.Next()
	}
}

// OptionalTeamContext is like TeamContext but doesn't require X-Team-ID header.
// If X-Team-ID is provided, it validates membership and sets context.
// If not provided, it continues without team context.
func OptionalTeamContext(membershipCache *cache.TeamMembershipCache) fiber.Handler {
	return func(c *fiber.Ctx) error {
		teamID := c.Get("X-Team-ID")
		if teamID == "" {
			return c.Next()
		}

		userID, ok := c.Locals("userID").(string)
		if !ok || userID == "" {
			return c.Next()
		}

		// Validate membership (with caching)
		membership, err := membershipCache.GetMembership(c.Context(), userID, teamID)
		if err != nil {
			return c.Next()
		}

		if membership.IsMember {
			c.Locals("teamID", teamID)
			c.Locals("teamRole", membership.Role)
		}

		return c.Next()
	}
}

// TeamScope middleware ensures team context is present.
// It reads X-Team-ID from header and validates team membership.
// This is a convenience middleware that combines TeamContext validation with scope checking.
//
// Note: This requires a membershipCache to be set via SetTeamScopeMembershipCache
// before the middleware is used. If not set, it will only check for existing context.
var teamScopeMembershipCache *cache.TeamMembershipCache

// SetTeamScopeMembershipCache sets the membership cache used by TeamScope middleware
func SetTeamScopeMembershipCache(c *cache.TeamMembershipCache) {
	teamScopeMembershipCache = c
}

// TeamScope middleware ensures team context is present.
// If team context is not already set, it reads X-Team-ID header and validates membership.
func TeamScope() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Check if team context is already set
		teamID := c.Locals("teamID")
		if teamID != nil && teamID != "" {
			return c.Next()
		}

		// Read from header
		headerTeamID := c.Get("X-Team-ID")
		if headerTeamID == "" {
			return response.Error(c, fiber.StatusBadRequest, "X-Team-ID header is required")
		}

		userID, ok := c.Locals("userID").(string)
		if !ok || userID == "" {
			return response.Unauthorized(c, "Authentication required")
		}

		// Validate membership if cache is available
		if teamScopeMembershipCache != nil {
			membership, err := teamScopeMembershipCache.GetMembership(c.Context(), userID, headerTeamID)
			if err != nil {
				return response.Error(c, fiber.StatusInternalServerError, "Failed to validate team membership")
			}

			if !membership.IsMember {
				return response.Forbidden(c, "You are not a member of this team")
			}

			c.Locals("teamRole", membership.Role)
		}

		// Set team context
		c.Locals("teamID", headerTeamID)

		return c.Next()
	}
}

// RequireTeamRole middleware ensures the user has at least the specified role.
// Roles are hierarchical: owner > admin > editor > member
func RequireTeamRole(minRole string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		teamRole, ok := c.Locals("teamRole").(string)
		if !ok || teamRole == "" {
			return response.Forbidden(c, "Team context required")
		}

		if !hasMinimumRole(teamRole, minRole) {
			return response.Forbidden(c, "Insufficient permissions")
		}

		return c.Next()
	}
}

// hasMinimumRole checks if the user's role meets the minimum required role
func hasMinimumRole(userRole, minRole string) bool {
	roleHierarchy := map[string]int{
		"owner":  4,
		"admin":  3,
		"editor": 2,
		"member": 1,
	}

	userLevel, ok := roleHierarchy[userRole]
	if !ok {
		return false
	}

	minLevel, ok := roleHierarchy[minRole]
	if !ok {
		return false
	}

	return userLevel >= minLevel
}
