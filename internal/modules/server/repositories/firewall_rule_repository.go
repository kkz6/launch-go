package repositories

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// CreateFirewallRule creates a new firewall rule
func (r *Repository) CreateFirewallRule(ctx context.Context, rule *models.FirewallRule) error {
	return create(r, ctx, rule)
}

// FindFirewallRuleByID finds a firewall rule by ID
func (r *Repository) FindFirewallRuleByID(ctx context.Context, id string) (*models.FirewallRule, error) {
	return findByID[models.FirewallRule](r, ctx, id, ErrFirewallRuleNotFound)
}

// FindFirewallRuleByIDWithServer finds a firewall rule by ID with the Server relation preloaded
func (r *Repository) FindFirewallRuleByIDWithServer(ctx context.Context, id string) (*models.FirewallRule, error) {
	return findByIDWithPreload[models.FirewallRule](r, ctx, id, "Server", ErrFirewallRuleNotFound)
}

// FindFirewallRuleByIDAndServer finds a firewall rule by ID and server ID
func (r *Repository) FindFirewallRuleByIDAndServer(ctx context.Context, id, serverID string) (*models.FirewallRule, error) {
	return findByIDAndServer[models.FirewallRule](r, ctx, id, serverID, ErrFirewallRuleNotFound)
}

// FindFirewallRulesByServer finds all firewall rules for a server
func (r *Repository) FindFirewallRulesByServer(ctx context.Context, serverID string) ([]models.FirewallRule, error) {
	return findByServer[models.FirewallRule](r, ctx, serverID)
}

// UpdateFirewallRule updates a firewall rule
func (r *Repository) UpdateFirewallRule(ctx context.Context, rule *models.FirewallRule) error {
	return update(r, ctx, rule)
}

// MarkFirewallRuleInstalled marks a firewall rule as installed
func (r *Repository) MarkFirewallRuleInstalled(ctx context.Context, id string) error {
	return markAsInstalled[models.FirewallRule](r, ctx, id)
}

// MarkFirewallRuleFailed marks a firewall rule installation as failed
func (r *Repository) MarkFirewallRuleFailed(ctx context.Context, id string) error {
	return markAsFailed[models.FirewallRule](r, ctx, id)
}

// DeleteFirewallRule deletes a firewall rule
func (r *Repository) DeleteFirewallRule(ctx context.Context, id string) error {
	return deleteByID[models.FirewallRule](r, ctx, id)
}
