package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/contracts"
)

var (
	ErrServerNotFound       = errors.New("server not found")
	ErrServiceNotFound      = errors.New("service not found")
	ErrFirewallRuleNotFound = errors.New("firewall rule not found")
	ErrCronNotFound         = errors.New("cron job not found")
	ErrDaemonNotFound       = errors.New("daemon not found")
	ErrSshKeyNotFound       = errors.New("ssh key not found")
	ErrTaskNotFound         = errors.New("task not found")
	ErrQueueNotConfigured   = errors.New("queue not configured")
)

// Repository provides database operations for server module
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new Repository instance
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Transaction executes a function within a database transaction
func (r *Repository) Transaction(ctx context.Context, fn func(tx contracts.Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&Repository{db: tx})
	})
}
