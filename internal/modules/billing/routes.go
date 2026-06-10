package billing

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/billing/handlers"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// RegisterRoutes registers the module routes (implements app.RouteRegistrar)
func (m *Module) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	deps := m.Deps()
	// Billing redirects (Polar checkout success/cancel) land the user back
	// in the frontend, not the API.
	handler := handlers.NewBillingHandler(m.service, m.serverCountFn, deps.Config.App.Frontend())

	m.registerBillingRoutes(router, authMiddleware, handler)
	m.registerSubscriptionRoutes(router, authMiddleware, handler)
}

// registerBillingRoutes registers billing-related routes
func (m *Module) registerBillingRoutes(router fiber.Router, authMiddleware fiber.Handler, handler *handlers.BillingHandler) {
	billing := router.Group("/billing", authMiddleware, middleware.TeamScope())
	{
		billing.Get("/", fiberutil.Handler(handler.Index))
		billing.Get("/plans", handler.GetPlans)
		billing.Post("/generate-checkout-url", middleware.Can("billing.update"), fiberutil.Bind(handler.GenerateCheckoutURL))
		billing.Post("/cancel-subscription", middleware.Can("billing.update"), fiberutil.Validate(handler.CancelSubscription))
		billing.Post("/resume-subscription", middleware.Can("billing.update"), fiberutil.Validate(handler.ResumeSubscription))

		billing.Get("/subscriptions", fiberutil.Handler(handler.GetSubscriptions))
		billing.Get("/subscriptions/:id", handler.GetSubscription)
		billing.Get("/orders", fiberutil.Handler(handler.GetOrders))
		billing.Get("/options", fiberutil.Handler(handler.GetSubscriptionOptions))
	}
}

// registerSubscriptionRoutes registers subscription registration route
func (m *Module) registerSubscriptionRoutes(router fiber.Router, authMiddleware fiber.Handler, handler *handlers.BillingHandler) {
	router.Get("/register/subscription", authMiddleware, middleware.TeamScope(), fiberutil.Handler(handler.RegisterSubscription))
}

// RegisterWebhookRoutes registers webhook routes (implements app.WebhookRegistrar)
func (m *Module) RegisterWebhookRoutes(router fiber.Router) {
	deps := m.Deps()
	webhookHandler := handlers.NewWebhookHandler(m.service, m.webhookService, m.polarClient, deps.Logger)

	m.setupWebhookRoutes(router, webhookHandler)
}

// setupWebhookRoutes registers Polar webhook routes
func (m *Module) setupWebhookRoutes(router fiber.Router, handler *handlers.WebhookHandler) {
	router.Post("/webhooks/polar", handler.HandleWebhook)
}
