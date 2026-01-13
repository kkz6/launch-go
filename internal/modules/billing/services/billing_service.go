package services

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/kkz6/launch-go/internal/modules/billing/dto"
	"github.com/kkz6/launch-go/internal/modules/billing/enums"
	"github.com/kkz6/launch-go/internal/modules/billing/models"
	"github.com/kkz6/launch-go/internal/modules/billing/providers"
	"github.com/kkz6/launch-go/internal/modules/billing/repositories"
	apperrors "github.com/kkz6/launch-go/internal/pkg/errors"
)

// Service errors - re-exported from centralized error package
var (
	ErrSubscriptionNotFound     = apperrors.ErrSubscriptionNotFound
	ErrNoActiveSubscription     = apperrors.NotFound("No active subscription found")
	ErrAlreadySubscribed        = apperrors.Conflict("Team already has an active subscription")
	ErrSubscriptionNotCancelled = apperrors.BadRequest("Subscription is not cancelled")
	ErrCannotResume             = apperrors.BadRequest("Cannot resume subscription - grace period has ended")
	ErrPlanNotFound             = apperrors.ErrPlanNotFound
	ErrSubscriptionsNotEnabled  = apperrors.BadRequest("Subscriptions are not enabled")
	ErrLimitExceeded            = apperrors.BadRequest("Limit exceeded")
)

// Config holds billing configuration
type Config struct {
	SubscriptionsEnabled bool
	Plans                []models.Plan
}

// BillingService handles billing operations
type BillingService struct {
	repo         *repositories.BillingRepository
	lemonSqueezy *providers.LemonSqueezyClient
	config       *Config
	logger       *zerolog.Logger
}

// NewBillingService creates a new billing service
func NewBillingService(repo *repositories.BillingRepository, lemonSqueezy *providers.LemonSqueezyClient, config *Config, logger *zerolog.Logger) *BillingService {
	return &BillingService{
		repo:         repo,
		lemonSqueezy: lemonSqueezy,
		config:       config,
		logger:       logger,
	}
}

// GetPlans returns all available plans
func (s *BillingService) GetPlans() []models.Plan {
	return s.config.Plans
}

// GetPlanByID finds a plan by its ID
func (s *BillingService) GetPlanByID(planID string) *models.Plan {
	for i := range s.config.Plans {
		if s.config.Plans[i].ID == planID {
			return &s.config.Plans[i]
		}
	}

	return nil
}

// GetPlanByProductID finds a plan by its product ID
func (s *BillingService) GetPlanByProductID(productID string) *models.Plan {
	for i := range s.config.Plans {
		if s.config.Plans[i].MonthlyID == productID || s.config.Plans[i].YearlyID == productID {
			return &s.config.Plans[i]
		}
	}

	return nil
}

// GetPlanByVariantID finds a plan by its variant ID
func (s *BillingService) GetPlanByVariantID(variantID string) *models.Plan {
	for i := range s.config.Plans {
		if s.config.Plans[i].MonthlyID == variantID || s.config.Plans[i].YearlyID == variantID {
			return &s.config.Plans[i]
		}
	}

	return nil
}

// GenerateCheckoutURL generates a checkout URL for a team to subscribe
func (s *BillingService) GenerateCheckoutURL(ctx context.Context, teamID string, req *dto.GenerateCheckoutURLRequest, redirectURL string) (string, error) {
	plan := s.GetPlanByID(req.PlanID)
	if plan == nil {
		return "", ErrPlanNotFound
	}

	variantID := plan.MonthlyID
	if req.Annual {
		variantID = plan.YearlyID
	}

	url, err := s.lemonSqueezy.CreateCheckout(ctx, variantID, plan.Name, teamID, redirectURL)
	if err != nil {
		s.logger.Error().Err(err).Str("team_id", teamID).Str("plan_id", req.PlanID).Msg("Failed to create checkout URL")
		return "", err
	}

	return url, nil
}

// GetSubscriptions returns all subscriptions for a team
func (s *BillingService) GetSubscriptions(ctx context.Context, teamID string) ([]models.Subscription, error) {
	return s.repo.FindSubscriptionsByTeam(ctx, teamID)
}

// GetActiveSubscription returns the active subscription for a team
func (s *BillingService) GetActiveSubscription(ctx context.Context, teamID string) (*models.Subscription, error) {
	return s.repo.FindActiveSubscriptionByTeam(ctx, teamID)
}

// GetSubscriptionByID returns a subscription by ID
func (s *BillingService) GetSubscriptionByID(ctx context.Context, id string) (*models.Subscription, error) {
	return s.repo.FindSubscriptionByID(ctx, id)
}

// CancelSubscription cancels a subscription
func (s *BillingService) CancelSubscription(ctx context.Context, subscriptionID string) error {
	subscription, err := s.repo.FindSubscriptionByID(ctx, subscriptionID)
	if err != nil {
		return ErrSubscriptionNotFound
	}

	if subscription.Status == enums.SubscriptionStatusCancelled {
		return nil
	}

	err = s.lemonSqueezy.CancelSubscription(ctx, subscription.LemonSqueezyID)
	if err != nil {
		s.logger.Error().Err(err).Str("subscription_id", subscriptionID).Msg("Failed to cancel subscription")
		return err
	}

	subscription.Status = enums.SubscriptionStatusCancelled
	return s.repo.UpdateSubscription(ctx, subscription)
}

// ResumeSubscription resumes a cancelled subscription
func (s *BillingService) ResumeSubscription(ctx context.Context, subscriptionID string) error {
	subscription, err := s.repo.FindSubscriptionByID(ctx, subscriptionID)
	if err != nil {
		return ErrSubscriptionNotFound
	}

	if subscription.Status != enums.SubscriptionStatusCancelled {
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

	subscription.Status = enums.SubscriptionStatusActive
	subscription.EndsAt = nil
	return s.repo.UpdateSubscription(ctx, subscription)
}

// GetOrders returns all orders for a team
func (s *BillingService) GetOrders(ctx context.Context, teamID string) ([]models.Order, error) {
	return s.repo.FindOrdersByTeam(ctx, teamID)
}

// IsSubscribed checks if a team is subscribed
func (s *BillingService) IsSubscribed(ctx context.Context, teamID string) (bool, error) {
	return s.repo.IsTeamSubscribed(ctx, teamID)
}

// GetBillingData returns complete billing data for a team
func (s *BillingService) GetBillingData(ctx context.Context, teamID string, serverCount int) (*dto.BillingIndexResponse, error) {
	subscriptions, err := s.repo.FindSubscriptionsByTeam(ctx, teamID)
	if err != nil {
		return nil, err
	}

	orders, err := s.repo.FindOrdersByTeam(ctx, teamID)
	if err != nil {
		return nil, err
	}

	subscriptionResponses := make([]dto.SubscriptionResponse, len(subscriptions))
	for i, sub := range subscriptions {
		plan := s.GetPlanByProductID(sub.ProductID)
		updateURL := ""
		if s.lemonSqueezy != nil {
			updateURL, _ = s.lemonSqueezy.GetUpdatePaymentMethodURL(ctx, sub.LemonSqueezyID)
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

// GetRepository returns the billing repository
func (s *BillingService) GetRepository() *repositories.BillingRepository {
	return s.repo
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
