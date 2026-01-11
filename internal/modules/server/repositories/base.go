package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// Repository errors with HTTP status codes
var (
	ErrServerNotFound       = response.ErrNotFound("Server not found")
	ErrServiceNotFound      = response.ErrNotFound("Service not found")
	ErrFirewallRuleNotFound = response.ErrNotFound("Firewall rule not found")
	ErrCronNotFound         = response.ErrNotFound("Cron job not found")
	ErrDaemonNotFound       = response.ErrNotFound("Daemon not found")
	ErrSshKeyNotFound       = response.ErrNotFound("SSH key not found")
	ErrTaskNotFound         = response.ErrNotFound("Task not found")
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
