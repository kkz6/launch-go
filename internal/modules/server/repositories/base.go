package repositories

import (
	"gorm.io/gorm"

	apperrors "github.com/kkz6/launch-go/internal/pkg/errors"
)

// Repository errors - re-exported from centralized error package
var (
	ErrServerNotFound         = apperrors.ErrServerNotFound
	ErrServiceNotFound        = apperrors.ErrServiceNotFound
	ErrFirewallRuleNotFound   = apperrors.ErrFirewallRuleNotFound
	ErrCronNotFound           = apperrors.ErrCronNotFound
	ErrDaemonNotFound         = apperrors.ErrDaemonNotFound
	ErrSshKeyNotFound         = apperrors.ErrSshKeyNotFound
	ErrTaskNotFound           = apperrors.ErrTaskNotFound
	ErrMetricNotFound         = apperrors.ErrMetricNotFound
	ErrServerProviderNotFound = apperrors.ErrServerProviderNotFound
	ErrDatabaseNotFound       = apperrors.ErrDatabaseNotFound
	ErrDatabaseUserNotFound   = apperrors.ErrDatabaseUserNotFound
)

// BaseRepository provides common functionality for all repositories
type BaseRepository struct {
	db *gorm.DB
}

// NewBaseRepository creates a new BaseRepository instance
func NewBaseRepository(db *gorm.DB) BaseRepository {
	return BaseRepository{db: db}
}

// DB returns the underlying database connection
func (r *BaseRepository) DB() *gorm.DB {
	return r.db
}
