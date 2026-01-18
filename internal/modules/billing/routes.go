package billing

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/billing/handlers"
)

// RegisterRoutes registers the module routes (implements app.RouteRegistrar)
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	deps := m.Deps()
	handler := handlers.NewBillingHandler(m.service, m.serverCountFn, deps.Config.App.URL)

	m.registerBillingRoutes(router, authMiddleware, handler)
	m.registerSubscriptionRoutes(router, authMiddleware, handler)
}

// registerBillingRoutes registers billing-related routes
func (m *Module) registerBillingRoutes(router fiber.Router, authMiddleware fiber.Handler, handler *handlers.BillingHandler) {
	billing := router.Group("/billing", authMiddleware, middleware.TeamScope())
	{
		billing.Get("/", handler.Index)
		billing.Get("/plans", handler.GetPlans)
		billing.Post("/generate-checkout-url", handler.GenerateCheckoutURL)
		billing.Post("/cancel-subscription", handler.CancelSubscription)
		billing.Post("/resume-subscription", handler.ResumeSubscription)

		billing.Get("/subscriptions", handler.GetSubscriptions)
		billing.Get("/subscriptions/:id", handler.GetSubscription)
		billing.Get("/orders", handler.GetOrders)
		billing.Get("/options", handler.GetSubscriptionOptions)
	}
}

// registerSubscriptionRoutes registers subscription registration route
func (m *Module) registerSubscriptionRoutes(router fiber.Router, authMiddleware fiber.Handler, handler *handlers.BillingHandler) {
	router.Get("/register/subscription", authMiddleware, middleware.TeamScope(), handler.RegisterSubscription)
}

// RegisterWebhookRoutes registers webhook routes (implements app.WebhookRegistrar)
func (m *Module) RegisterWebhookRoutes(router fiber.Router) {
	deps := m.Deps()
	webhookHandler := handlers.NewWebhookHandler(m.repos, m.service, deps.Config.Billing.WebhookSecret, deps.Logger)

	m.registerWebhookRoutes(router, webhookHandler)
}

// registerWebhookRoutes registers Lemon Squeezy webhook routes
func (m *Module) registerWebhookRoutes(router fiber.Router, handler *handlers.WebhookHandler) {
	router.Post("/webhooks/lemon-squeezy", handler.HandleWebhook)
}
