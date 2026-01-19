package middleware

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/response"
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
// Must be used after TeamScope middleware.
//
// If subscriptions are not enabled (config), this middleware does nothing.
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

		teamID, ok := c.Locals("teamID").(string)
		if !ok || teamID == "" {
			return response.Error(c, fiber.StatusBadRequest, "Team context required")
		}

		// Check if team has active subscription
		if isTeamSubscribed(teamID) {
			return c.Next()
		}

		return response.Error(c, fiber.StatusPaymentRequired, "Active subscription required")
	}
}

// isTeamSubscribed checks if a team has an active subscription or is on trial
func isTeamSubscribed(teamID string) bool {
	if subscriptionMiddleware.db == nil {
		// No database configured - allow request (shouldn't happen in production)
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
