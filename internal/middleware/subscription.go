package middleware

import (
	"context"

	"github.com/gofiber/fiber/v2"

	billingrepos "github.com/kkz6/launch-go/internal/modules/billing/repositories"
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
// Staff bypass subscription checks. Active subscriptions, trials, and cancelled
// subscriptions within their paid grace period allow the request to proceed.
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
		if isTeamSubscribed(c.UserContext(), teamID) {
			return c.Next()
		}

		return fiberctx.Error(c, fiber.StatusPaymentRequired, "Active subscription required")
	}
}

// isTeamSubscribed uses the billing repository's eligibility rules.
func isTeamSubscribed(ctx context.Context, teamID string) bool {
	if subscriptionMiddleware.db == nil {
		return true
	}

	subscribed, err := billingrepos.NewSubscriptionRepository(subscriptionMiddleware.db).IsTeamSubscribed(ctx, teamID)
	return err == nil && subscribed
}

// isUserAdmin reports whether a user has any staff role (support or
// super_admin) — staff bypass subscription checks. Replaces the legacy
// Spatie model_has_roles lookup.
func isUserAdmin(userID string) bool {
	if subscriptionMiddleware.db == nil {
		return false
	}
	var role *string
	subscriptionMiddleware.db.Table("users").
		Select("staff_role").
		Where("id = ?", userID).
		Scan(&role)
	return role != nil && *role != ""
}
