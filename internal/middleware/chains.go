package middleware

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/signedurl"
)

// Chain combines multiple middleware handlers into a single slice.
// This is useful for applying multiple middleware to a route group.
//
// Usage:
//
//	router.Group("/api", middleware.Chain(mw1, mw2, mw3)...)
func Chain(handlers ...fiber.Handler) []fiber.Handler {
	return handlers
}

// AuthenticatedChain returns the standard middleware chain for authenticated API routes.
// This includes JWT authentication, team scope, and subscription verification.
//
// Usage:
//
//	router.Group("/servers", middleware.AuthenticatedChain(authMiddleware)...)
func AuthenticatedChain(authMiddleware fiber.Handler) []fiber.Handler {
	return []fiber.Handler{
		authMiddleware,
		TeamScope(),
		VerifySubscription(),
	}
}

// AuthenticatedChainWithRole returns the authenticated chain plus a role requirement.
// This is useful for routes that require a specific team role.
//
// Usage:
//
//	router.Group("/admin", middleware.AuthenticatedChainWithRole(authMiddleware, "admin")...)
func AuthenticatedChainWithRole(authMiddleware fiber.Handler, minRole string) []fiber.Handler {
	return []fiber.Handler{
		authMiddleware,
		TeamScope(),
		VerifySubscription(),
		RequireRole(minRole),
	}
}

// TeamScopeChain returns middleware for team-scoped routes without subscription check.
// This is useful for routes like billing that need team context but not subscription.
//
// Usage:
//
//	router.Group("/billing", middleware.TeamScopeChain(authMiddleware)...)
func TeamScopeChain(authMiddleware fiber.Handler) []fiber.Handler {
	return []fiber.Handler{
		authMiddleware,
		TeamScope(),
	}
}

// OptionalAuthChain returns middleware for routes with optional authentication.
// If authenticated, team scope is also optional.
//
// Usage:
//
//	router.Group("/public", middleware.OptionalAuthChain(jwtSecret, db)...)
func OptionalAuthChain(jwtSecret string, db *gorm.DB) []fiber.Handler {
	return []fiber.Handler{
		OptionalAuth(jwtSecret, db),
		OptionalTeamScope(),
	}
}

// WebhookChain returns middleware for webhook routes that require signed URL verification.
// This provides signature verification without authentication.
//
// Usage:
//
//	router.Group("/webhooks", middleware.WebhookChain()...)
func WebhookChain() []fiber.Handler {
	return []fiber.Handler{
		signedurl.RequireSignedURL(nil),
	}
}

// WebhookChainWithSigner returns middleware for webhook routes with a custom signer.
//
// Usage:
//
//	router.Group("/webhooks", middleware.WebhookChainWithSigner(customSigner)...)
func WebhookChainWithSigner(signer *signedurl.Signer) []fiber.Handler {
	return []fiber.Handler{
		signedurl.RequireSignedURL(signer),
	}
}

// APIChain returns middleware suitable for API endpoints.
// This includes rate limiting for the specified requests per window.
//
// Usage:
//
//	router.Group("/api/v1", middleware.APIChain(100, 60)...)
func APIChain(maxRequests int, windowSeconds int) []fiber.Handler {
	return []fiber.Handler{
		RateLimit(maxRequests, windowSeconds),
	}
}

// AuthenticatedAPIChain combines authentication and API-specific middleware.
// This includes JWT auth, team scope, subscription check, and rate limiting.
//
// Usage:
//
//	router.Group("/api/v1", middleware.AuthenticatedAPIChain(authMiddleware, 100, 60)...)
func AuthenticatedAPIChain(authMiddleware fiber.Handler, maxRequests int, windowSeconds int) []fiber.Handler {
	return []fiber.Handler{
		authMiddleware,
		TeamScope(),
		VerifySubscription(),
		RateLimit(maxRequests, windowSeconds),
	}
}

// TeamRoleChain returns middleware for routes requiring team membership verification.
// Uses the TeamService interface for runtime membership checks.
//
// Usage:
//
//	router.Get("/:teamId", middleware.TeamRoleChain(authMiddleware, service, "member")...)
func TeamRoleChain(authMiddleware fiber.Handler, service TeamService, role string) []fiber.Handler {
	switch role {
	case "owner":
		return []fiber.Handler{authMiddleware, TeamOwner(service)}
	case "admin":
		return []fiber.Handler{authMiddleware, TeamAdmin(service)}
	default:
		return []fiber.Handler{authMiddleware, TeamMember(service)}
	}
}

// Append adds additional middleware to an existing chain.
// This is useful for extending a base chain with route-specific middleware.
//
// Usage:
//
//	chain := middleware.AuthenticatedChain(authMiddleware)
//	router.Group("/admin", middleware.Append(chain, customMiddleware)...)
func Append(chain []fiber.Handler, handlers ...fiber.Handler) []fiber.Handler {
	result := make([]fiber.Handler, len(chain)+len(handlers))
	copy(result, chain)
	copy(result[len(chain):], handlers)
	return result
}

// Prepend adds middleware to the beginning of an existing chain.
// This is useful for adding middleware that should run before the chain.
//
// Usage:
//
//	chain := middleware.AuthenticatedChain(authMiddleware)
//	router.Group("/api", middleware.Prepend(chain, loggingMiddleware)...)
func Prepend(chain []fiber.Handler, handlers ...fiber.Handler) []fiber.Handler {
	result := make([]fiber.Handler, len(handlers)+len(chain))
	copy(result, handlers)
	copy(result[len(handlers):], chain)
	return result
}
