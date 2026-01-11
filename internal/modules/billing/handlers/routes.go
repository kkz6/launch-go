package handlers

import (
	"github.com/gofiber/fiber/v2"
)

// RegisterRoutes registers billing routes
func RegisterRoutes(router fiber.Router, handler *BillingHandler, webhookHandler *WebhookHandler, authMiddleware fiber.Handler) {
	billing := router.Group("/billing", authMiddleware)

	billing.Get("/", handler.Index)
	billing.Get("/plans", handler.GetPlans)
	billing.Post("/checkout-url", handler.GenerateCheckoutURL)
	billing.Post("/cancel-subscription", handler.CancelSubscription)
	billing.Post("/resume-subscription", handler.ResumeSubscription)

	billing.Get("/subscriptions", handler.GetSubscriptions)
	billing.Get("/subscriptions/:id", handler.GetSubscription)
	billing.Get("/orders", handler.GetOrders)
	billing.Get("/options", handler.GetSubscriptionOptions)

	router.Get("/register/subscription", authMiddleware, handler.RegisterSubscription)

	router.Post("/webhooks/lemon-squeezy", webhookHandler.HandleWebhook)
}

// RegisterSettingsRoutes registers billing routes under settings
func RegisterSettingsRoutes(settings fiber.Router, handler *BillingHandler) {
	settings.Get("/billing", handler.Index)
}
