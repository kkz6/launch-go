package repositories

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// FirewallRuleRepository handles firewall rule database operations
type FirewallRuleRepository struct {
	BaseRepository
}

// NewFirewallRuleRepository creates a new FirewallRuleRepository instance
func NewFirewallRuleRepository(db *gorm.DB) *FirewallRuleRepository {
	return &FirewallRuleRepository{
		BaseRepository: NewBaseRepository(db),
	}
}

// Create creates a new firewall rule
func (r *FirewallRuleRepository) Create(ctx context.Context, rule *models.FirewallRule) error {
	return r.DB().WithContext(ctx).Create(rule).Error
}

// FindByID finds a firewall rule by ID
func (r *FirewallRuleRepository) FindByID(ctx context.Context, id string) (*models.FirewallRule, error) {
	var rule models.FirewallRule
	err := r.DB().WithContext(ctx).First(&rule, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFirewallRuleNotFound
		}
		return nil, err
	}
	return &rule, nil
}

// FindByIDWithServer finds a firewall rule by ID with the Server relation preloaded
func (r *FirewallRuleRepository) FindByIDWithServer(ctx context.Context, id string) (*models.FirewallRule, error) {
	var rule models.FirewallRule
	err := r.DB().WithContext(ctx).Preload("Server").First(&rule, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFirewallRuleNotFound
		}
		return nil, err
	}
	return &rule, nil
}

// FindByIDAndServer finds a firewall rule by ID and server ID
func (r *FirewallRuleRepository) FindByIDAndServer(ctx context.Context, id, serverID string) (*models.FirewallRule, error) {
	var rule models.FirewallRule
	err := r.DB().WithContext(ctx).First(&rule, "id = ? AND server_id = ?", id, serverID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFirewallRuleNotFound
		}
		return nil, err
	}
	return &rule, nil
}

// FindByServer finds all firewall rules for a server
func (r *FirewallRuleRepository) FindByServer(ctx context.Context, serverID string) ([]models.FirewallRule, error) {
	var rules []models.FirewallRule
	err := r.DB().WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&rules).Error
	return rules, err
}

// Update updates a firewall rule
func (r *FirewallRuleRepository) Update(ctx context.Context, rule *models.FirewallRule) error {
	return r.DB().WithContext(ctx).Save(rule).Error
}

// MarkInstalled marks a firewall rule as installed
func (r *FirewallRuleRepository) MarkInstalled(ctx context.Context, id string) error {
	now := time.Now()
	return r.DB().WithContext(ctx).
		Model(&models.FirewallRule{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"installed_at":           now,
			"installation_failed_at": nil,
		}).Error
}

// MarkFailed marks a firewall rule installation as failed
func (r *FirewallRuleRepository) MarkFailed(ctx context.Context, id string) error {
	now := time.Now()
	return r.DB().WithContext(ctx).
		Model(&models.FirewallRule{}).
		Where("id = ?", id).
		Update("installation_failed_at", now).Error
}

// Delete deletes a firewall rule
func (r *FirewallRuleRepository) Delete(ctx context.Context, id string) error {
	return r.DB().WithContext(ctx).Delete(&models.FirewallRule{}, "id = ?", id).Error
}
