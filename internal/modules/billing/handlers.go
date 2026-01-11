package billing

import (
	"context"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

// Handler handles HTTP requests for billing
type Handler struct {
	service        *Service
	serverCountFn  func(teamID string) (int, error)
}

// NewHandler creates a new billing handler
func NewHandler(service *Service, serverCountFn func(teamID string) (int, error)) *Handler {
	return &Handler{
		service:       service,
		serverCountFn: serverCountFn,
	}
}

// Index returns billing information for the current team
func (h *Handler) Index(c *fiber.Ctx) error {
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
		return response.InternalError(c, "Failed to fetch billing data")
	}

	return response.OK(c, "Billing data retrieved", data)
}

// GetPlans returns all available subscription plans
func (h *Handler) GetPlans(c *fiber.Ctx) error {
	plans := h.service.GetPlans()

	planResponses := make([]PlanResponse, len(plans))
	for i, plan := range plans {
		planResponses[i] = ToPlanResponse(&plan)
	}

	return response.OK(c, "Plans retrieved", planResponses)
}

// GenerateCheckoutURL generates a checkout URL for subscribing
func (h *Handler) GenerateCheckoutURL(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)

	var req GenerateCheckoutURLRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	redirectURL := c.BaseURL() + "/settings/billing"

	url, err := h.service.GenerateCheckoutURL(c.Context(), teamID, &req, redirectURL)
	if err != nil {
		if err == ErrPlanNotFound {
			return response.Error(c, fiber.StatusBadRequest, "Plan not found")
		}
		return response.InternalError(c, "Failed to generate checkout URL")
	}

	return response.OK(c, "Checkout URL generated", GenerateCheckoutURLResponse{URL: url})
}

// CancelSubscription cancels a subscription
func (h *Handler) CancelSubscription(c *fiber.Ctx) error {
	var req CancelSubscriptionRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	err := h.service.CancelSubscription(c.Context(), req.SubscriptionID)
	if err != nil {
		if err == ErrSubscriptionNotFound {
			return response.NotFound(c, "Subscription not found")
		}
		return response.InternalError(c, "Failed to cancel subscription")
	}

	return response.OK(c, "Subscription cancelled successfully", nil)
}

// ResumeSubscription resumes a cancelled subscription
func (h *Handler) ResumeSubscription(c *fiber.Ctx) error {
	var req ResumeSubscriptionRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	err := h.service.ResumeSubscription(c.Context(), req.SubscriptionID)
	if err != nil {
		switch err {
		case ErrSubscriptionNotFound:
			return response.NotFound(c, "Subscription not found")
		case ErrSubscriptionNotCancelled:
			return response.Error(c, fiber.StatusBadRequest, "Subscription is not cancelled")
		case ErrCannotResume:
			return response.Error(c, fiber.StatusBadRequest, "Cannot resume subscription - grace period has ended")
		default:
			return response.InternalError(c, err.Error())
		}
	}

	return response.OK(c, "Subscription resumed successfully", nil)
}

// GetSubscriptions returns all subscriptions for the current team
func (h *Handler) GetSubscriptions(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)

	subscriptions, err := h.service.GetSubscriptions(c.Context(), teamID)
	if err != nil {
		return response.InternalError(c, "Failed to fetch subscriptions")
	}

	subscriptionResponses := make([]SubscriptionResponse, len(subscriptions))
	for i, sub := range subscriptions {
		plan := h.service.GetPlanByProductID(sub.ProductID)
		subscriptionResponses[i] = ToSubscriptionResponse(&sub, plan, "")
	}

	return response.OK(c, "Subscriptions retrieved", subscriptionResponses)
}

// GetSubscription returns a specific subscription
func (h *Handler) GetSubscription(c *fiber.Ctx) error {
	id := c.Params("id")

	subscription, err := h.service.GetSubscriptionByID(c.Context(), id)
	if err != nil {
		return response.NotFound(c, "Subscription not found")
	}

	plan := h.service.GetPlanByProductID(subscription.ProductID)
	return response.OK(c, "Subscription retrieved", ToSubscriptionResponse(subscription, plan, ""))
}

// GetOrders returns all orders for the current team
func (h *Handler) GetOrders(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)

	orders, err := h.service.GetOrders(c.Context(), teamID)
	if err != nil {
		return response.InternalError(c, "Failed to fetch orders")
	}

	orderResponses := make([]OrderResponse, len(orders))
	for i, order := range orders {
		orderResponses[i] = ToOrderResponse(&order)
	}

	return response.OK(c, "Orders retrieved", orderResponses)
}

// GetSubscriptionOptions returns the subscription limits/options for the current team
func (h *Handler) GetSubscriptionOptions(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)
	userRole := UserRoleCustomer

	if role, ok := c.Locals("userRole").(string); ok {
		userRole = UserRole(role)
	}

	options := NewTeamSubscriptionOptions(
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
		return response.InternalError(c, "Failed to fetch subscription options")
	}

	return response.OK(c, "Subscription options retrieved", resp)
}

// RegisterSubscription shows the subscription selection page
func (h *Handler) RegisterSubscription(c *fiber.Ctx) error {
	teamID := c.Locals("teamID").(string)

	subscribed, _ := h.service.IsSubscribed(c.Context(), teamID)
	if subscribed {
		return response.Error(c, fiber.StatusBadRequest, "Team already has an active subscription")
	}

	plans := h.service.GetPlans()
	planResponses := make([]PlanResponse, len(plans))
	for i, plan := range plans {
		planResponses[i] = ToPlanResponse(&plan)
	}

	return response.OK(c, "Registration subscription plans", planResponses)
}
