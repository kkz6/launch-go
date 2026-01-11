package repositories

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// CreateFirewallRule creates a new firewall rule
func (r *Repository) CreateFirewallRule(ctx context.Context, rule *models.FirewallRule) error {
	return r.db.WithContext(ctx).Create(rule).Error
}

// FindFirewallRuleByID finds a firewall rule by ID
func (r *Repository) FindFirewallRuleByID(ctx context.Context, id string) (*models.FirewallRule, error) {
	var rule models.FirewallRule
	err := r.db.WithContext(ctx).First(&rule, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFirewallRuleNotFound
		}

		return nil, err
	}

	return &rule, nil
}

// FindFirewallRuleByIDAndServer finds a firewall rule by ID and server ID
func (r *Repository) FindFirewallRuleByIDAndServer(ctx context.Context, id, serverID string) (*models.FirewallRule, error) {
	var rule models.FirewallRule
	err := r.db.WithContext(ctx).
		First(&rule, "id = ? AND server_id = ?", id, serverID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFirewallRuleNotFound
		}

		return nil, err
	}

	return &rule, nil
}

// FindFirewallRulesByServer finds all firewall rules for a server
func (r *Repository) FindFirewallRulesByServer(ctx context.Context, serverID string) ([]models.FirewallRule, error) {
	var rules []models.FirewallRule
	err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&rules).Error

	return rules, err
}

// UpdateFirewallRule updates a firewall rule
func (r *Repository) UpdateFirewallRule(ctx context.Context, rule *models.FirewallRule) error {
	return r.db.WithContext(ctx).Save(rule).Error
}

// MarkFirewallRuleInstalled marks a firewall rule as installed
func (r *Repository) MarkFirewallRuleInstalled(ctx context.Context, id string) error {
	now := time.Now()

	return r.db.WithContext(ctx).
		Model(&models.FirewallRule{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"installed_at":           now,
			"installation_failed_at": nil,
		}).Error
}

// MarkFirewallRuleFailed marks a firewall rule installation as failed
func (r *Repository) MarkFirewallRuleFailed(ctx context.Context, id string) error {
	now := time.Now()

	return r.db.WithContext(ctx).
		Model(&models.FirewallRule{}).
		Where("id = ?", id).
		Update("installation_failed_at", now).Error
}

// DeleteFirewallRule deletes a firewall rule
func (r *Repository) DeleteFirewallRule(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.FirewallRule{}, "id = ?", id).Error
}
