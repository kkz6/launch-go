package repositories

import (
	"context"
	"database/sql"
	"strings"

	"gorm.io/gorm"

	authmodels "github.com/kkz6/launch-go/internal/modules/auth/models"
	stafftypes "github.com/kkz6/launch-go/internal/modules/staff/types"
)

// Registry holds the staff module's database access.
type Registry struct {
	db *gorm.DB
}

// NewRegistry creates a new staff repository registry.
func NewRegistry(db *gorm.DB) *Registry {
	return &Registry{db: db}
}

// StaffRole reads the user's staff_role column and parses it. Returns nil for
// a NULL/empty value, a missing user, or an unparseable value (defensive).
func (r *Registry) StaffRole(ctx context.Context, userID string) *stafftypes.StaffRole {
	if userID == "" {
		return nil
	}

	var raw sql.NullString
	err := r.db.WithContext(ctx).
		Model(&authmodels.User{}).
		Where("id = ?", userID).
		Limit(1).
		Pluck("staff_role", &raw).Error
	if err != nil {
		return nil
	}

	if !raw.Valid || strings.TrimSpace(raw.String) == "" {
		return nil
	}

	role, err := stafftypes.ParseStaffRole(raw.String)
	if err != nil {
		return nil
	}

	return &role
}

// ListUsers returns a cross-tenant page of users ordered by created_at desc
// together with the total count of users.
func (r *Registry) ListUsers(ctx context.Context, limit, offset int) ([]authmodels.User, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&authmodels.User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var users []authmodels.User
	err := r.db.WithContext(ctx).
		Model(&authmodels.User{}).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&users).Error
	if err != nil {
		return nil, 0, err
	}

	return users, total, nil
}
