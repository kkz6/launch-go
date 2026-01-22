package middleware

import (
	"github.com/gofiber/fiber/v2"

	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"gorm.io/gorm"
)

// subscriptionMiddleware holds the shared state for subscription verification
var subscriptionMiddleware struct {
	db                   *gorm.DB
	subscriptionsEnabled bool
}

// InitSubscriptionMiddleware initializes the subscription middleware with required dependencies.
// Call this during application bootstrap before routes are registered.
func InitSubscriptionMiddleware(db *gorm.DB, subscriptionsEnabled bool) {
	subscriptionMiddleware.db = db
	subscriptionMiddleware.subscriptionsEnabled = subscriptionsEnabled
}

// VerifySubscription middleware ensures the team has an active subscription.
// Must be used after Auth and TeamScope middleware.
//
// If subscriptions are not enabled (config), this middleware does nothing.
// If the user is an admin/manager, they bypass subscription checks.
// If the team is on trial or has an active subscription, request proceeds.
// Otherwise, returns a 402 Payment Required error.
//
// Usage:
//
//	router.Group("/servers", middleware.Auth(secret), middleware.TeamScope(), middleware.VerifySubscription())
func VerifySubscription() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Skip if subscriptions are not enabled
		if !subscriptionMiddleware.subscriptionsEnabled {
			return c.Next()
		}

		userID, ok := c.Locals("userID").(string)
		if !ok || userID == "" {
			return fiberctx.RespondUnauthorized(c, "Authentication required")
		}

		// Admins bypass subscription checks
		if isUserAdmin(userID) {
			return c.Next()
		}

		teamID, ok := c.Locals("teamID").(string)
		if !ok || teamID == "" {
			return fiberctx.Error(c, fiber.StatusBadRequest, "Team context required")
		}

		// Check if team has active subscription
		if isTeamSubscribed(teamID) {
			return c.Next()
		}

		return fiberctx.Error(c, fiber.StatusPaymentRequired, "Active subscription required")
	}
}

// isTeamSubscribed checks if a team has an active subscription or is on trial
func isTeamSubscribed(teamID string) bool {
	if subscriptionMiddleware.db == nil {
		return true
	}

	var count int64
	subscriptionMiddleware.db.Table("lemon_squeezy_subscriptions").
		Where("billable_id = ?", teamID).
		Where("billable_type IN ?", []string{"Modules\\Auth\\Models\\Team", "App\\Models\\Team"}).
		Where("status IN ?", []string{"active", "on_trial"}).
		Count(&count)

	return count > 0
}

// isUserAdmin checks if a user has admin or manager role
func isUserAdmin(userID string) bool {
	if subscriptionMiddleware.db == nil {
		return false
	}

	var count int64
	subscriptionMiddleware.db.Table("model_has_roles").
		Joins("JOIN roles ON roles.id = model_has_roles.role_id").
		Where("model_has_roles.model_id = ?", userID).
		Where("model_has_roles.model_type IN ?", []string{"Modules\\Auth\\Models\\User", "App\\Models\\User"}).
		Where("roles.name IN ?", []string{"admin", "manager"}).
		Count(&count)

	return count > 0
}
