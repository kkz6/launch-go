package billing

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Common errors
var (
	ErrSubscriptionNotFound     = errors.New("subscription not found")
	ErrNoActiveSubscription     = errors.New("no active subscription found")
	ErrAlreadySubscribed        = errors.New("team already has an active subscription")
	ErrSubscriptionNotCancelled = errors.New("subscription is not cancelled")
	ErrCannotResume             = errors.New("cannot resume subscription")
	ErrPlanNotFound             = errors.New("plan not found")
	ErrSubscriptionsNotEnabled  = errors.New("subscriptions are not enabled")
	ErrLimitExceeded            = errors.New("limit exceeded")
)

// Config holds billing configuration
type Config struct {
	SubscriptionsEnabled bool
	Plans                []Plan
}

// Service handles billing operations
type Service struct {
	repo           *Repository
	lemonSqueezy   *LemonSqueezyClient
	config         *Config
	logger         *zerolog.Logger
}

// NewService creates a new billing service
func NewService(repo *Repository, lemonSqueezy *LemonSqueezyClient, config *Config, logger *zerolog.Logger) *Service {
	return &Service{
		repo:         repo,
		lemonSqueezy: lemonSqueezy,
		config:       config,
		logger:       logger,
	}
}

// GetPlans returns all available plans
func (s *Service) GetPlans() []Plan {
	return s.config.Plans
}

// GetPlanByID finds a plan by its ID
func (s *Service) GetPlanByID(planID string) *Plan {
	for i := range s.config.Plans {
		if s.config.Plans[i].ID == planID {
			return &s.config.Plans[i]
		}
	}

	return nil
}

// GetPlanByProductID finds a plan by its product ID
func (s *Service) GetPlanByProductID(productID string) *Plan {
	for i := range s.config.Plans {
		if s.config.Plans[i].MonthlyID == productID || s.config.Plans[i].YearlyID == productID {
			return &s.config.Plans[i]
		}
	}

	return nil
}

// GetPlanByVariantID finds a plan by its variant ID
func (s *Service) GetPlanByVariantID(variantID string) *Plan {
	for i := range s.config.Plans {
		if s.config.Plans[i].MonthlyID == variantID || s.config.Plans[i].YearlyID == variantID {
			return &s.config.Plans[i]
		}
	}

	return nil
}

// GenerateCheckoutURL generates a checkout URL for a team to subscribe
func (s *Service) GenerateCheckoutURL(ctx context.Context, teamID string, req *GenerateCheckoutURLRequest, redirectURL string) (string, error) {
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
func (s *Service) GetSubscriptions(ctx context.Context, teamID string) ([]Subscription, error) {
	return s.repo.FindSubscriptionsByTeam(ctx, teamID)
}

// GetActiveSubscription returns the active subscription for a team
func (s *Service) GetActiveSubscription(ctx context.Context, teamID string) (*Subscription, error) {
	return s.repo.FindActiveSubscriptionByTeam(ctx, teamID)
}

// GetSubscriptionByID returns a subscription by ID
func (s *Service) GetSubscriptionByID(ctx context.Context, id string) (*Subscription, error) {
	return s.repo.FindSubscriptionByID(ctx, id)
}

// CancelSubscription cancels a subscription
func (s *Service) CancelSubscription(ctx context.Context, subscriptionID string) error {
	subscription, err := s.repo.FindSubscriptionByID(ctx, subscriptionID)
	if err != nil {
		return ErrSubscriptionNotFound
	}

	if subscription.Status == SubscriptionStatusCancelled {
		return nil
	}

	err = s.lemonSqueezy.CancelSubscription(ctx, subscription.LemonSqueezyID)
	if err != nil {
		s.logger.Error().Err(err).Str("subscription_id", subscriptionID).Msg("Failed to cancel subscription")
		return err
	}

	subscription.Status = SubscriptionStatusCancelled
	return s.repo.UpdateSubscription(ctx, subscription)
}

// ResumeSubscription resumes a cancelled subscription
func (s *Service) ResumeSubscription(ctx context.Context, subscriptionID string) error {
	subscription, err := s.repo.FindSubscriptionByID(ctx, subscriptionID)
	if err != nil {
		return ErrSubscriptionNotFound
	}

	if subscription.Status != SubscriptionStatusCancelled {
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

	subscription.Status = SubscriptionStatusActive
	subscription.EndsAt = nil
	return s.repo.UpdateSubscription(ctx, subscription)
}

// GetOrders returns all orders for a team
func (s *Service) GetOrders(ctx context.Context, teamID string) ([]Order, error) {
	return s.repo.FindOrdersByTeam(ctx, teamID)
}

// IsSubscribed checks if a team is subscribed
func (s *Service) IsSubscribed(ctx context.Context, teamID string) (bool, error) {
	return s.repo.IsTeamSubscribed(ctx, teamID)
}

// GetBillingData returns complete billing data for a team
func (s *Service) GetBillingData(ctx context.Context, teamID string, serverCount int) (*BillingIndexResponse, error) {
	subscriptions, err := s.repo.FindSubscriptionsByTeam(ctx, teamID)
	if err != nil {
		return nil, err
	}

	orders, err := s.repo.FindOrdersByTeam(ctx, teamID)
	if err != nil {
		return nil, err
	}

	subscriptionResponses := make([]SubscriptionResponse, len(subscriptions))
	for i, sub := range subscriptions {
		plan := s.GetPlanByProductID(sub.ProductID)
		updateURL := ""
		if s.lemonSqueezy != nil {
			updateURL, _ = s.lemonSqueezy.GetUpdatePaymentMethodURL(ctx, sub.LemonSqueezyID)
		}
		subscriptionResponses[i] = ToSubscriptionResponse(&sub, plan, updateURL)
	}

	orderResponses := make([]OrderResponse, len(orders))
	for i, order := range orders {
		orderResponses[i] = ToOrderResponse(&order)
	}

	return &BillingIndexResponse{
		ServerCount:       serverCount,
		Subscriptions:     subscriptionResponses,
		SubscriptionPlans: s.config.Plans,
		Receipts:          orderResponses,
	}, nil
}

// TeamSubscriptionOptions handles subscription-based limits for a team
type TeamSubscriptionOptions struct {
	teamID            string
	service           *Service
	serverCountFn     func(ctx context.Context, teamID string) (int, error)
	teamMemberCountFn func(ctx context.Context, teamID string) (int, error)
	siteCountFn       func(ctx context.Context, serverID string) (int, error)
	userRole          UserRole

	mu               sync.RWMutex
	cachedOptions    *PlanOptions
	cachedIsAdmin    *bool
}

// AdminLimits are the limits for admin users
var AdminLimits = PlanOptions{
	MaxServers:             999999,
	MaxSitesPerServer:      999999,
	MaxDeploymentsPerSite:  999999,
	MaxTeamMembers:         999999,
	HasBackups:             true,
	HasMonitoring:          true,
}

// FreeLimits are the limits for users without a subscription
var FreeLimits = PlanOptions{
	MaxServers:             0,
	MaxSitesPerServer:      0,
	MaxDeploymentsPerSite:  5,
	MaxTeamMembers:         1,
	HasBackups:             false,
	HasMonitoring:          false,
}

// NewTeamSubscriptionOptions creates a new TeamSubscriptionOptions
func NewTeamSubscriptionOptions(
	teamID string,
	service *Service,
	userRole UserRole,
	serverCountFn func(ctx context.Context, teamID string) (int, error),
	teamMemberCountFn func(ctx context.Context, teamID string) (int, error),
	siteCountFn func(ctx context.Context, serverID string) (int, error),
) *TeamSubscriptionOptions {
	return &TeamSubscriptionOptions{
		teamID:            teamID,
		service:           service,
		userRole:          userRole,
		serverCountFn:     serverCountFn,
		teamMemberCountFn: teamMemberCountFn,
		siteCountFn:       siteCountFn,
	}
}

// MustVerifySubscription returns true if subscriptions are enabled
func (t *TeamSubscriptionOptions) MustVerifySubscription() bool {
	return t.service.config.SubscriptionsEnabled
}

// IsSubscribed checks if the team is subscribed
func (t *TeamSubscriptionOptions) IsSubscribed(ctx context.Context) bool {
	subscribed, _ := t.service.IsSubscribed(ctx, t.teamID)
	return subscribed
}

// OnTrialOrIsSubscribed checks if the team is on trial or subscribed
func (t *TeamSubscriptionOptions) OnTrialOrIsSubscribed(ctx context.Context) bool {
	if !t.MustVerifySubscription() {
		return true
	}

	if t.IsAdmin() {
		return true
	}

	subscription, err := t.service.GetActiveSubscription(ctx, t.teamID)
	if err != nil {
		return false
	}

	if subscription == nil {
		return false
	}

	return subscription.IsActive() || subscription.OnTrial()
}

// CanCreateServer checks if the team can create a server
func (t *TeamSubscriptionOptions) CanCreateServer(ctx context.Context) bool {
	if !t.MustVerifySubscription() {
		return true
	}

	if t.IsAdmin() {
		return true
	}

	serverCount, err := t.CountServers(ctx)
	if err != nil {
		return false
	}

	return serverCount < t.MaxServers(ctx)
}

// CanCreateSiteOnServer checks if the team can create a site on a server
func (t *TeamSubscriptionOptions) CanCreateSiteOnServer(ctx context.Context, serverID string) bool {
	if !t.MustVerifySubscription() {
		return true
	}

	if t.IsAdmin() {
		return true
	}

	siteCount, err := t.CountSitesOnServer(ctx, serverID)
	if err != nil {
		return false
	}

	return siteCount < t.MaxSitesPerServer(ctx)
}

// CanAddTeamMember checks if the team can add a team member
func (t *TeamSubscriptionOptions) CanAddTeamMember(ctx context.Context) bool {
	if !t.MustVerifySubscription() {
		return true
	}

	if t.IsAdmin() {
		return true
	}

	memberCount, err := t.CountTeamMembers(ctx)
	if err != nil {
		return false
	}

	options := t.PlanOptions(ctx)
	return memberCount < options.MaxTeamMembers
}

// MaxServers returns the maximum number of servers allowed
func (t *TeamSubscriptionOptions) MaxServers(ctx context.Context) int {
	return t.PlanOptions(ctx).MaxServers
}

// MaxSitesPerServer returns the maximum number of sites per server
func (t *TeamSubscriptionOptions) MaxSitesPerServer(ctx context.Context) int {
	return t.PlanOptions(ctx).MaxSitesPerServer
}

// MaxDeploymentsPerSite returns the maximum number of deployments per site
func (t *TeamSubscriptionOptions) MaxDeploymentsPerSite(ctx context.Context) int {
	if !t.MustVerifySubscription() {
		return 0
	}

	return t.PlanOptions(ctx).MaxDeploymentsPerSite
}

// HasBackups returns true if the team has access to backups
func (t *TeamSubscriptionOptions) HasBackups(ctx context.Context) bool {
	if !t.MustVerifySubscription() {
		return true
	}

	return t.PlanOptions(ctx).HasBackups
}

// HasMonitoring returns true if the team has access to monitoring
func (t *TeamSubscriptionOptions) HasMonitoring(ctx context.Context) bool {
	if !t.MustVerifySubscription() {
		return true
	}

	return t.PlanOptions(ctx).HasMonitoring
}

// CountServers returns the number of servers for the team
func (t *TeamSubscriptionOptions) CountServers(ctx context.Context) (int, error) {
	if t.serverCountFn == nil {
		return 0, nil
	}

	return t.serverCountFn(ctx, t.teamID)
}

// CountSitesOnServer returns the number of sites on a server
func (t *TeamSubscriptionOptions) CountSitesOnServer(ctx context.Context, serverID string) (int, error) {
	if t.siteCountFn == nil {
		return 0, nil
	}

	return t.siteCountFn(ctx, serverID)
}

// CountTeamMembers returns the number of team members
func (t *TeamSubscriptionOptions) CountTeamMembers(ctx context.Context) (int, error) {
	if t.teamMemberCountFn == nil {
		return 1, nil
	}

	count, err := t.teamMemberCountFn(ctx, t.teamID)
	if err != nil {
		return 0, err
	}

	return count + 1, nil
}

// PlanOptions returns the plan options for the team
func (t *TeamSubscriptionOptions) PlanOptions(ctx context.Context) PlanOptions {
	t.mu.RLock()
	if t.cachedOptions != nil {
		defer t.mu.RUnlock()
		return *t.cachedOptions
	}
	t.mu.RUnlock()

	t.mu.Lock()
	defer t.mu.Unlock()

	// Double-check after acquiring write lock
	if t.cachedOptions != nil {
		return *t.cachedOptions
	}

	options := t.resolvePlanOptions(ctx)
	t.cachedOptions = &options
	return options
}

// IsAdmin checks if the current user is an admin
func (t *TeamSubscriptionOptions) IsAdmin() bool {
	t.mu.RLock()
	if t.cachedIsAdmin != nil {
		defer t.mu.RUnlock()
		return *t.cachedIsAdmin
	}
	t.mu.RUnlock()

	t.mu.Lock()
	defer t.mu.Unlock()

	// Double-check after acquiring write lock
	if t.cachedIsAdmin != nil {
		return *t.cachedIsAdmin
	}

	isAdmin := t.userRole.IsAdmin()
	t.cachedIsAdmin = &isAdmin
	return isAdmin
}

// isAdminUnsafe checks if the current user is an admin without locking (assumes caller holds lock)
func (t *TeamSubscriptionOptions) isAdminUnsafe() bool {
	if t.cachedIsAdmin != nil {
		return *t.cachedIsAdmin
	}
	isAdmin := t.userRole.IsAdmin()
	t.cachedIsAdmin = &isAdmin
	return isAdmin
}

// resolvePlanOptions resolves the plan options based on subscription status
// Note: This method assumes the caller holds the write lock
func (t *TeamSubscriptionOptions) resolvePlanOptions(ctx context.Context) PlanOptions {
	if t.isAdminUnsafe() {
		return AdminLimits
	}

	subscription, err := t.service.GetActiveSubscription(ctx, t.teamID)
	if err != nil || subscription == nil {
		return FreeLimits
	}

	if subscription.OnTrial() {
		return t.getFirstPlanOptions()
	}

	if !subscription.IsActive() {
		return FreeLimits
	}

	return t.getSubscribedPlanOptions(subscription)
}

// getFirstPlanOptions returns the options for the first plan (used for trials)
func (t *TeamSubscriptionOptions) getFirstPlanOptions() PlanOptions {
	plans := t.service.GetPlans()
	if len(plans) == 0 {
		return FreeLimits
	}

	return plans[0].Options
}

// getSubscribedPlanOptions returns the options for the subscribed plan
func (t *TeamSubscriptionOptions) getSubscribedPlanOptions(subscription *Subscription) PlanOptions {
	plan := t.service.GetPlanByProductID(subscription.ProductID)
	if plan == nil {
		return FreeLimits
	}

	return plan.Options
}

// GetSubscriptionOptionsResponse returns the subscription options as a response
func (t *TeamSubscriptionOptions) GetSubscriptionOptionsResponse(ctx context.Context) (*SubscriptionOptionsResponse, error) {
	options := t.PlanOptions(ctx)

	serverCount, _ := t.CountServers(ctx)

	subscribed := t.IsSubscribed(ctx)

	subscription, _ := t.service.GetActiveSubscription(ctx, t.teamID)
	onTrial := subscription != nil && subscription.OnTrial()

	return &SubscriptionOptionsResponse{
		IsSubscribed:          subscribed,
		OnTrial:               onTrial,
		MaxServers:            options.MaxServers,
		MaxSitesPerServer:     options.MaxSitesPerServer,
		MaxDeploymentsPerSite: options.MaxDeploymentsPerSite,
		MaxTeamMembers:        options.MaxTeamMembers,
		HasBackups:            options.HasBackups,
		HasMonitoring:         options.HasMonitoring,
		CanCreateServer:       t.CanCreateServer(ctx),
		ServerCount:           serverCount,
	}, nil
}
