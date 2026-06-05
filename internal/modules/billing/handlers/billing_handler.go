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
func (h *BillingHandler) Index(r *fiberctx.Request) error {
	serverCount := 0
	if h.serverCountFn != nil {
		count, err := h.serverCountFn(r.TeamID)
		if err == nil {
			serverCount = count
		}
	}

	data, err := h.service.GetBillingData(r.Context(), r.TeamID, serverCount)
	if err != nil {
		return fiberctx.HandleError(r.Ctx, err)
	}

	return fiberctx.OK(r.Ctx, "Billing data retrieved", data)
}

// GetPlans returns all available subscription plans
func (h *BillingHandler) GetPlans(c *fiber.Ctx) error {
	plans := h.service.GetPlans()

	planResponses := pkgdto.TransformSlice(plans, dto.ToPlanResponse)

	return fiberctx.OK(c, "Plans retrieved", planResponses)
}

// GenerateCheckoutURL generates a checkout URL for subscribing
func (h *BillingHandler) GenerateCheckoutURL(r *fiberctx.Request, req *dto.GenerateCheckoutURLRequest) error {
	redirectURL := h.appURL + "/settings/billing"

	// Extract customer info from auth context to prefill checkout
	customerEmail, _ := r.Locals("email").(string)
	customerName, _ := r.Locals("name").(string)

	url, err := h.service.GenerateCheckoutURL(r.Context(), r.TeamID, req, redirectURL, customerEmail, customerName)
	if err != nil {
		return fiberctx.HandleError(r.Ctx, err)
	}

	return fiberctx.OK(r.Ctx, "Checkout URL generated", dto.GenerateCheckoutURLResponse{URL: url})
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
func (h *BillingHandler) GetSubscriptions(r *fiberctx.Request) error {
	subscriptions, err := h.service.GetSubscriptions(r.Context(), r.TeamID)
	if err != nil {
		return fiberctx.HandleError(r.Ctx, err)
	}

	subscriptionResponses := make([]dto.SubscriptionResponse, len(subscriptions))
	for i, sub := range subscriptions {
		plan := h.service.GetPlanByProductID(sub.ProductID)
		subscriptionResponses[i] = dto.ToSubscriptionResponse(&sub, plan, "")
	}

	return fiberctx.OK(r.Ctx, "Subscriptions retrieved", subscriptionResponses)
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
func (h *BillingHandler) GetOrders(r *fiberctx.Request) error {
	orders, err := h.service.GetOrders(r.Context(), r.TeamID)
	if err != nil {
		return fiberctx.HandleError(r.Ctx, err)
	}

	orderResponses := pkgdto.TransformSlice(orders, dto.ToOrderResponse)

	return fiberctx.OK(r.Ctx, "Orders retrieved", orderResponses)
}

// GetSubscriptionOptions returns the subscription limits/options for the current team
func (h *BillingHandler) GetSubscriptionOptions(r *fiberctx.Request) error {
	userRole := billingtypes.UserRoleCustomer

	if role := fiberctx.GetUserRole(r.Ctx); role != "" {
		userRole = billingtypes.UserRole(role)
	}

	options := services.NewTeamSubscriptionOptions(
		r.TeamID,
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

	resp, err := options.GetSubscriptionOptionsResponse(r.Context())
	if err != nil {
		return fiberctx.HandleError(r.Ctx, err)
	}

	return fiberctx.OK(r.Ctx, "Subscription options retrieved", resp)
}

// RegisterSubscription shows the subscription selection page
func (h *BillingHandler) RegisterSubscription(r *fiberctx.Request) error {
	subscribed, err := h.service.IsSubscribed(r.Context(), r.TeamID)
	if err != nil {
		return fiberctx.HandleError(r.Ctx, err)
	}
	if subscribed {
		return fiberctx.HandleError(r.Ctx, services.ErrAlreadySubscribed)
	}

	plans := h.service.GetPlans()
	planResponses := pkgdto.TransformSlice(plans, dto.ToPlanResponse)

	return fiberctx.OK(r.Ctx, "Registration subscription plans", planResponses)
}
