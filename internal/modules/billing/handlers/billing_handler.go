package handlers

import (
	"context"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/billing/dto"
	"github.com/kkz6/launch-go/internal/modules/billing/services"
	billingtypes "github.com/kkz6/launch-go/internal/modules/billing/types"
	pkgdto "github.com/kkz6/launch-go/internal/pkg/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
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
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	serverCount := 0
	if h.serverCountFn != nil {
		count, err := h.serverCountFn(teamID)
		if err == nil {
			serverCount = count
		}
	}

	data, err := h.service.GetBillingData(c.Context(), teamID, serverCount)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Billing data retrieved", data)
}

// GetPlans returns all available subscription plans
func (h *BillingHandler) GetPlans(c *fiber.Ctx) error {
	plans := h.service.GetPlans()

	planResponses := pkgdto.TransformSlice(plans, dto.ToPlanResponse)

	return fiberctx.OK(c, "Plans retrieved", planResponses)
}

// GenerateCheckoutURL generates a checkout URL for subscribing
func (h *BillingHandler) GenerateCheckoutURL(c *fiber.Ctx, req *dto.GenerateCheckoutURLRequest) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	redirectURL := h.appURL + "/settings/billing"

	// Extract customer info from auth context to prefill checkout
	customerEmail, _ := c.Locals("email").(string)
	customerName, _ := c.Locals("name").(string)

	url, err := h.service.GenerateCheckoutURL(c.Context(), teamID, req, redirectURL, customerEmail, customerName)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Checkout URL generated", dto.GenerateCheckoutURLResponse{URL: url})
}

// CancelSubscription cancels a subscription
func (h *BillingHandler) CancelSubscription(c *fiber.Ctx, req *dto.CancelSubscriptionRequest) error {
	if err := h.service.CancelSubscription(c.Context(), req.SubscriptionID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Subscription cancelled successfully", nil)
}

// ResumeSubscription resumes a cancelled subscription
func (h *BillingHandler) ResumeSubscription(c *fiber.Ctx, req *dto.ResumeSubscriptionRequest) error {
	if err := h.service.ResumeSubscription(c.Context(), req.SubscriptionID); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Subscription resumed successfully", nil)
}

// GetSubscriptions returns all subscriptions for the current team
func (h *BillingHandler) GetSubscriptions(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	subscriptions, err := h.service.GetSubscriptions(c.Context(), teamID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	subscriptionResponses := make([]dto.SubscriptionResponse, len(subscriptions))
	for i, sub := range subscriptions {
		plan := h.service.GetPlanByProductID(sub.ProductID)
		subscriptionResponses[i] = dto.ToSubscriptionResponse(&sub, plan, "")
	}

	return fiberctx.OK(c, "Subscriptions retrieved", subscriptionResponses)
}

// GetSubscription returns a specific subscription
func (h *BillingHandler) GetSubscription(c *fiber.Ctx) error {
	id := c.Params("id")

	subscription, err := h.service.GetSubscriptionByID(c.Context(), id)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	plan := h.service.GetPlanByProductID(subscription.ProductID)
	return fiberctx.OK(c, "Subscription retrieved", dto.ToSubscriptionResponse(subscription, plan, ""))
}

// GetOrders returns all orders for the current team
func (h *BillingHandler) GetOrders(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	orders, err := h.service.GetOrders(c.Context(), teamID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	orderResponses := pkgdto.TransformSlice(orders, dto.ToOrderResponse)

	return fiberctx.OK(c, "Orders retrieved", orderResponses)
}

// GetSubscriptionOptions returns the subscription limits/options for the current team
func (h *BillingHandler) GetSubscriptionOptions(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}
	userRole := billingtypes.UserRoleCustomer

	if role := fiberctx.GetUserRole(c); role != "" {
		userRole = billingtypes.UserRole(role)
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
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Subscription options retrieved", resp)
}

// RegisterSubscription shows the subscription selection page
func (h *BillingHandler) RegisterSubscription(c *fiber.Ctx) error {
	teamID, err := fiberctx.MustGetTeamID(c)
	if err != nil {
		return err
	}

	subscribed, err := h.service.IsSubscribed(c.Context(), teamID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}
	if subscribed {
		return fiberctx.HandleError(c, services.ErrAlreadySubscribed)
	}

	plans := h.service.GetPlans()
	planResponses := pkgdto.TransformSlice(plans, dto.ToPlanResponse)

	return fiberctx.OK(c, "Registration subscription plans", planResponses)
}
