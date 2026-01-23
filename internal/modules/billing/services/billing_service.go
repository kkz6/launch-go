package services

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/billing/dto"
	"github.com/kkz6/launch-go/internal/modules/billing/models"
	"github.com/kkz6/launch-go/internal/modules/billing/providers"
	"github.com/kkz6/launch-go/internal/modules/billing/repositories"
	billingtypes "github.com/kkz6/launch-go/internal/modules/billing/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// Service errors - using fiber error utilities
var (
	ErrSubscriptionNotFound     = fiberutil.NotFound("Subscription not found")
	ErrNoActiveSubscription     = fiberutil.NotFound("No active subscription found")
	ErrAlreadySubscribed        = fiberutil.Conflict("Team already has an active subscription")
	ErrSubscriptionNotCancelled = fiberutil.BadRequest("Subscription is not cancelled")
	ErrCannotResume             = fiberutil.BadRequest("Cannot resume subscription - grace period has ended")
	ErrPlanNotFound             = fiberutil.NotFound("Plan not found")
	ErrSubscriptionsNotEnabled  = fiberutil.BadRequest("Subscriptions are not enabled")
	ErrLimitExceeded            = fiberutil.BadRequest("Limit exceeded")
)

// Config holds billing configuration
type Config struct {
	SubscriptionsEnabled bool
	Plans                []models.Plan
}

// BillingService handles billing operations
type BillingService struct {
	repos            *repositories.Registry
	lemonSqueezy     *providers.LemonSqueezyClient
	config           *Config
	logger           *zerolog.Logger
	plansByID        map[string]*models.Plan
	plansByProductID map[string]*models.Plan
	plansByVariantID map[string]*models.Plan
}

// NewBillingService creates a new billing service
func NewBillingService(repos *repositories.Registry, lemonSqueezy *providers.LemonSqueezyClient, config *Config, logger *zerolog.Logger) *BillingService {
	svc := &BillingService{
		repos:            repos,
		lemonSqueezy:     lemonSqueezy,
		config:           config,
		logger:           logger,
		plansByID:        make(map[string]*models.Plan, len(config.Plans)),
		plansByProductID: make(map[string]*models.Plan, len(config.Plans)),
		plansByVariantID: make(map[string]*models.Plan, len(config.Plans)),
	}

	for i := range config.Plans {
		p := &config.Plans[i]
		svc.plansByID[p.ID] = p
		svc.plansByProductID[p.ID] = p
		if p.MonthlyID != "" {
			svc.plansByVariantID[p.MonthlyID] = p
		}
		if p.YearlyID != "" {
			svc.plansByVariantID[p.YearlyID] = p
		}
	}

	return svc
}

// GetPlans returns all available plans
func (s *BillingService) GetPlans() []models.Plan {
	return s.config.Plans
}

// GetPlanByID finds a plan by its ID
func (s *BillingService) GetPlanByID(planID string) *models.Plan {
	return s.plansByID[planID]
}

// GetPlanByProductID finds a plan by its product ID
func (s *BillingService) GetPlanByProductID(productID string) *models.Plan {
	return s.plansByProductID[productID]
}

// GetPlanByVariantID finds a plan by its variant ID
func (s *BillingService) GetPlanByVariantID(variantID string) *models.Plan {
	return s.plansByVariantID[variantID]
}

// GenerateCheckoutURL generates a checkout URL for a team to subscribe
func (s *BillingService) GenerateCheckoutURL(ctx context.Context, teamID string, req *dto.GenerateCheckoutURLRequest, redirectURL string) (string, error) {
	if s.lemonSqueezy == nil {
		return "", ErrSubscriptionsNotEnabled
	}

	plan := s.GetPlanByID(req.Plan)
	if plan == nil {
		return "", ErrPlanNotFound
	}

	variantID := plan.MonthlyID
	if req.Annual {
		variantID = plan.YearlyID
	}

	url, err := s.lemonSqueezy.CreateCheckout(ctx, variantID, plan.Name, teamID, redirectURL)
	if err != nil {
		s.logger.Error().Err(err).Str("team_id", teamID).Str("plan_id", req.Plan).Msg("Failed to create checkout URL")
		return "", err
	}

	return url, nil
}

// GetSubscriptions returns all subscriptions for a team
func (s *BillingService) GetSubscriptions(ctx context.Context, teamID string) ([]models.Subscription, error) {
	return s.repos.Subscription().FindByTeam(ctx, teamID)
}

// GetActiveSubscription returns the active subscription for a team
func (s *BillingService) GetActiveSubscription(ctx context.Context, teamID string) (*models.Subscription, error) {
	return s.repos.Subscription().FindActiveByTeam(ctx, teamID)
}

// GetSubscriptionByID returns a subscription by ID
func (s *BillingService) GetSubscriptionByID(ctx context.Context, id string) (*models.Subscription, error) {
	return s.repos.Subscription().FindByID(ctx, id)
}

// CancelSubscription cancels a subscription
func (s *BillingService) CancelSubscription(ctx context.Context, subscriptionID string) error {
	subscription, err := s.repos.Subscription().FindByID(ctx, subscriptionID)
	if err != nil {
		return ErrSubscriptionNotFound
	}

	if subscription.Status == billingtypes.SubscriptionStatusCancelled {
		return nil
	}

	err = s.lemonSqueezy.CancelSubscription(ctx, subscription.LemonSqueezyID)
	if err != nil {
		s.logger.Error().Err(err).Str("subscription_id", subscriptionID).Msg("Failed to cancel subscription")
		return err
	}

	subscription.Status = billingtypes.SubscriptionStatusCancelled
	return s.repos.Subscription().Update(ctx, subscription)
}

// ResumeSubscription resumes a cancelled subscription
func (s *BillingService) ResumeSubscription(ctx context.Context, subscriptionID string) error {
	subscription, err := s.repos.Subscription().FindByID(ctx, subscriptionID)
	if err != nil {
		return ErrSubscriptionNotFound
	}

	if subscription.Status != billingtypes.SubscriptionStatusCancelled {
		return ErrSubscriptionNotCancelled
	}

	if subscription.EndsAt != nil && subscription.EndsAt.Before(time.Now()) {
		return ErrCannotResume
	}

	err = s.lemonSqueezy.ResumeSubscription(ctx, subscription.LemonSqueezyID)
	if err != nil {
		s.logger.Error().Err(err).Str("subscription_id", subscriptionID).Msg("Failed to resume subscription")
		return err
	}

	subscription.Status = billingtypes.SubscriptionStatusActive
	subscription.EndsAt = nil
	return s.repos.Subscription().Update(ctx, subscription)
}

// GetOrders returns all orders for a team
func (s *BillingService) GetOrders(ctx context.Context, teamID string) ([]models.Order, error) {
	return s.repos.Order().FindByTeam(ctx, teamID)
}

// IsSubscribed checks if a team is subscribed
func (s *BillingService) IsSubscribed(ctx context.Context, teamID string) (bool, error) {
	return s.repos.Subscription().IsTeamSubscribed(ctx, teamID)
}

// GetBillingData returns complete billing data for a team
func (s *BillingService) GetBillingData(ctx context.Context, teamID string, serverCount int) (*dto.BillingIndexResponse, error) {
	subscriptions, err := s.repos.Subscription().FindByTeam(ctx, teamID)
	if err != nil {
		return nil, err
	}

	orders, err := s.repos.Order().FindByTeam(ctx, teamID)
	if err != nil {
		return nil, err
	}

	// NOTE: Payment URL fetches are N+1 because the LemonSqueezy API does not support
	// batch retrieval of update payment method URLs. Each subscription requires a
	// separate API call. Consider caching these URLs if performance becomes an issue,
	// as they typically don't change frequently.
	subscriptionResponses := make([]dto.SubscriptionResponse, len(subscriptions))
	for i, sub := range subscriptions {
		plan := s.GetPlanByProductID(sub.ProductID)
		updateURL := ""
		if s.lemonSqueezy != nil {
			var err error
			updateURL, err = s.lemonSqueezy.GetUpdatePaymentMethodURL(ctx, sub.LemonSqueezyID)
			if err != nil {
				s.logger.Warn().Err(err).Uint("subscription_id", sub.ID).Msg("Failed to get update payment method URL")
			}
		}
		subscriptionResponses[i] = dto.ToSubscriptionResponse(&sub, plan, updateURL)
	}

	orderResponses := make([]dto.OrderResponse, len(orders))
	for i, order := range orders {
		orderResponses[i] = dto.ToOrderResponse(&order)
	}

	return &dto.BillingIndexResponse{
		ServerCount:       serverCount,
		Subscriptions:     subscriptionResponses,
		SubscriptionPlans: s.config.Plans,
		Receipts:          orderResponses,
	}, nil
}

// Repos returns the billing repository registry
func (s *BillingService) Repos() *repositories.Registry {
	return s.repos
}

// GetConfig returns the billing configuration
func (s *BillingService) GetConfig() *Config {
	return s.config
}

// GetLemonSqueezyClient returns the LemonSqueezy client
func (s *BillingService) GetLemonSqueezyClient() *providers.LemonSqueezyClient {
	return s.lemonSqueezy
}

// SetSubscriptionsEnabled sets whether subscriptions are enabled
func (s *BillingService) SetSubscriptionsEnabled(enabled bool) {
	s.config.SubscriptionsEnabled = enabled
}
