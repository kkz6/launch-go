package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// FirewallRuleRepository handles firewall rule database operations
type FirewallRuleRepository struct {
	repository.Installable[models.FirewallRule]
}

// NewFirewallRuleRepository creates a new FirewallRuleRepository instance
func NewFirewallRuleRepository(db *gorm.DB) *FirewallRuleRepository {
	return &FirewallRuleRepository{
		Installable: repository.NewInstallable[models.FirewallRule](db),
	}
}

// FindByID finds a firewall rule by ID (override to return specific error)
func (r *FirewallRuleRepository) FindByID(ctx context.Context, id string) (*models.FirewallRule, error) {
	rule, err := r.Installable.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrFirewallRuleNotFound
		}
		return nil, err
	}
	return rule, nil
}

// FindByIDWithServer finds a firewall rule by ID with the Server relation preloaded (override to return specific error)
func (r *FirewallRuleRepository) FindByIDWithServer(ctx context.Context, id string) (*models.FirewallRule, error) {
	rule, err := r.Installable.FindByIDWithServer(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrFirewallRuleNotFound
		}
		return nil, err
	}
	return rule, nil
}

// FindByIDAndServer finds a firewall rule by ID and server ID (override to return specific error)
func (r *FirewallRuleRepository) FindByIDAndServer(ctx context.Context, id, serverID string) (*models.FirewallRule, error) {
	rule, err := r.Installable.FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrFirewallRuleNotFound
		}
		return nil, err
	}
	return rule, nil
}

// MarkInstalled marks a firewall rule as installed (alias for MarkAsInstalled)
func (r *FirewallRuleRepository) MarkInstalled(ctx context.Context, id string) error {
	return r.MarkAsInstalled(ctx, id)
}

// MarkFailed marks a firewall rule installation as failed (alias for MarkAsFailed)
func (r *FirewallRuleRepository) MarkFailed(ctx context.Context, id string) error {
	return r.MarkAsFailed(ctx, id)
}
