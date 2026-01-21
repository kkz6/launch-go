package services

import (
	"context"
	"sync"

	"github.com/kkz6/launch-go/internal/modules/billing/dto"
	"github.com/kkz6/launch-go/internal/modules/billing/enums"
	"github.com/kkz6/launch-go/internal/modules/billing/models"
)

// AdminLimits are the limits for admin users
var AdminLimits = models.PlanOptions{
	MaxServers:            999999,
	MaxSitesPerServer:     999999,
	MaxDeploymentsPerSite: 999999,
	MaxTeamMembers:        999999,
	HasBackups:            true,
	HasMonitoring:         true,
}

// FreeLimits are the limits for users without a subscription
var FreeLimits = models.PlanOptions{
	MaxServers:            0,
	MaxSitesPerServer:     0,
	MaxDeploymentsPerSite: 5,
	MaxTeamMembers:        1,
	HasBackups:            false,
	HasMonitoring:         false,
}

// TeamSubscriptionOptions handles subscription-based limits for a team
type TeamSubscriptionOptions struct {
	teamID            string
	service           *BillingService
	serverCountFn     func(ctx context.Context, teamID string) (int, error)
	teamMemberCountFn func(ctx context.Context, teamID string) (int, error)
	siteCountFn       func(ctx context.Context, serverID string) (int, error)
	userRole          enums.UserRole

	mu            sync.RWMutex
	cachedOptions *models.PlanOptions
	cachedIsAdmin *bool
}

// NewTeamSubscriptionOptions creates a new TeamSubscriptionOptions
func NewTeamSubscriptionOptions(
	teamID string,
	service *BillingService,
	userRole enums.UserRole,
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
	subscribed, err := t.service.IsSubscribed(ctx, t.teamID)
	if err != nil {
		t.service.logger.Warn().Err(err).Str("team_id", t.teamID).Msg("Failed to check subscription status")
		return false
	}
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
func (t *TeamSubscriptionOptions) PlanOptions(ctx context.Context) models.PlanOptions {
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
func (t *TeamSubscriptionOptions) resolvePlanOptions(ctx context.Context) models.PlanOptions {
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
func (t *TeamSubscriptionOptions) getFirstPlanOptions() models.PlanOptions {
	plans := t.service.GetPlans()
	if len(plans) == 0 {
		return FreeLimits
	}

	return plans[0].Options
}

// getSubscribedPlanOptions returns the options for the subscribed plan
func (t *TeamSubscriptionOptions) getSubscribedPlanOptions(subscription *models.Subscription) models.PlanOptions {
	plan := t.service.GetPlanByProductID(subscription.ProductID)
	if plan == nil {
		return FreeLimits
	}

	return plan.Options
}

// GetSubscriptionOptionsResponse returns the subscription options as a response
func (t *TeamSubscriptionOptions) GetSubscriptionOptionsResponse(ctx context.Context) (*dto.SubscriptionOptionsResponse, error) {
	options := t.PlanOptions(ctx)

	serverCount, err := t.CountServers(ctx)
	if err != nil {
		t.service.logger.Warn().Err(err).Str("team_id", t.teamID).Msg("Failed to count servers for subscription options")
	}

	subscribed := t.IsSubscribed(ctx)

	subscription, err := t.service.GetActiveSubscription(ctx, t.teamID)
	if err != nil {
		t.service.logger.Warn().Err(err).Str("team_id", t.teamID).Msg("Failed to get active subscription")
	}
	onTrial := subscription != nil && subscription.OnTrial()

	return &dto.SubscriptionOptionsResponse{
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
