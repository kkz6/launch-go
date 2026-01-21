package handlers

import (
	"context"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/billing/dto"
	"github.com/kkz6/launch-go/internal/modules/billing/enums"
	"github.com/kkz6/launch-go/internal/modules/billing/services"
	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// BillingHandler handles HTTP requests for billing
type BillingHandler struct {
	service       *services.BillingService
	serverCountFn func(teamID string) (int, error)
	appURL        string
}

// NewBillingHandler creates a new billing handler
func NewBillingHandler(service *services.BillingService, serverCountFn func(teamID string) (int, error), appURL string) *BillingHandler {
	return &BillingHandler{
		service:       service,
		serverCountFn: serverCountFn,
		appURL:        appURL,
	}
}

// Index returns billing information for the current team
func (h *BillingHandler) Index(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)

	serverCount := 0
	if h.serverCountFn != nil {
		count, err := h.serverCountFn(teamID)
		if err == nil {
			serverCount = count
		}
	}

	data, err := h.service.GetBillingData(c.Context(), teamID, serverCount)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Billing data retrieved", data)
}

// GetPlans returns all available subscription plans
func (h *BillingHandler) GetPlans(c *fiber.Ctx) error {
	plans := h.service.GetPlans()

	planResponses := make([]dto.PlanResponse, len(plans))
	for i, plan := range plans {
		planResponses[i] = dto.ToPlanResponse(&plan)
	}

	return response.OK(c, "Plans retrieved", planResponses)
}

// GenerateCheckoutURL generates a checkout URL for subscribing
func (h *BillingHandler) GenerateCheckoutURL(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)

	var req dto.GenerateCheckoutURLRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	redirectURL := h.appURL + "/settings/billing"

	url, err := h.service.GenerateCheckoutURL(c.Context(), teamID, &req, redirectURL)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Checkout URL generated", dto.GenerateCheckoutURLResponse{URL: url})
}

// CancelSubscription cancels a subscription
func (h *BillingHandler) CancelSubscription(c *fiber.Ctx) error {
	var req dto.CancelSubscriptionRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	if err := h.service.CancelSubscription(c.Context(), req.SubscriptionID); err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Subscription cancelled successfully", nil)
}

// ResumeSubscription resumes a cancelled subscription
func (h *BillingHandler) ResumeSubscription(c *fiber.Ctx) error {
	var req dto.ResumeSubscriptionRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	if err := h.service.ResumeSubscription(c.Context(), req.SubscriptionID); err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Subscription resumed successfully", nil)
}

// GetSubscriptions returns all subscriptions for the current team
func (h *BillingHandler) GetSubscriptions(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)

	subscriptions, err := h.service.GetSubscriptions(c.Context(), teamID)
	if err != nil {
		return response.HandleError(c, err)
	}

	subscriptionResponses := make([]dto.SubscriptionResponse, len(subscriptions))
	for i, sub := range subscriptions {
		plan := h.service.GetPlanByProductID(sub.ProductID)
		subscriptionResponses[i] = dto.ToSubscriptionResponse(&sub, plan, "")
	}

	return response.OK(c, "Subscriptions retrieved", subscriptionResponses)
}

// GetSubscription returns a specific subscription
func (h *BillingHandler) GetSubscription(c *fiber.Ctx) error {
	id := c.Params("id")

	subscription, err := h.service.GetSubscriptionByID(c.Context(), id)
	if err != nil {
		return response.HandleError(c, err)
	}

	plan := h.service.GetPlanByProductID(subscription.ProductID)
	return response.OK(c, "Subscription retrieved", dto.ToSubscriptionResponse(subscription, plan, ""))
}

// GetOrders returns all orders for the current team
func (h *BillingHandler) GetOrders(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)

	orders, err := h.service.GetOrders(c.Context(), teamID)
	if err != nil {
		return response.HandleError(c, err)
	}

	orderResponses := make([]dto.OrderResponse, len(orders))
	for i, order := range orders {
		orderResponses[i] = dto.ToOrderResponse(&order)
	}

	return response.OK(c, "Orders retrieved", orderResponses)
}

// GetSubscriptionOptions returns the subscription limits/options for the current team
func (h *BillingHandler) GetSubscriptionOptions(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	userRole := enums.UserRoleCustomer

	if role, ok := c.Locals("userRole").(string); ok {
		userRole = enums.UserRole(role)
	}

	options := services.NewTeamSubscriptionOptions(
		teamID,
		h.service,
		userRole,
		func(ctx context.Context, tID string) (int, error) {
			if h.serverCountFn != nil {
				return h.serverCountFn(tID)
			}
			return 0, nil
		},
		nil,
		nil,
	)

	resp, err := options.GetSubscriptionOptionsResponse(c.Context())
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Subscription options retrieved", resp)
}

// RegisterSubscription shows the subscription selection page
func (h *BillingHandler) RegisterSubscription(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)

	subscribed, err := h.service.IsSubscribed(c.Context(), teamID)
	if err != nil {
		return response.HandleError(c, err)
	}
	if subscribed {
		return response.HandleError(c, services.ErrAlreadySubscribed)
	}

	plans := h.service.GetPlans()
	planResponses := make([]dto.PlanResponse, len(plans))
	for i, plan := range plans {
		planResponses[i] = dto.ToPlanResponse(&plan)
	}

	return response.OK(c, "Registration subscription plans", planResponses)
}
