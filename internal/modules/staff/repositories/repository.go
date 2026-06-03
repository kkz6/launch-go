package repositories

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	authmodels "github.com/kkz6/launch-go/internal/modules/auth/models"
	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	staffmodels "github.com/kkz6/launch-go/internal/modules/staff/models"
	stafftypes "github.com/kkz6/launch-go/internal/modules/staff/types"
	"github.com/kkz6/launch-go/internal/pkg/util"
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

// UserStatus reads the user's status column and parses it. Returns
// UserStatusActive as the safe default for a NULL/empty value, a missing user,
// a query error, or an unparseable value. Mirrors StaffRole's sql.NullString
// approach (Pluck into a pointer was a prior bug; sql.NullString is correct).
func (r *Registry) UserStatus(ctx context.Context, userID string) authtypes.UserStatus {
	if userID == "" {
		return authtypes.UserStatusActive
	}

	var raw sql.NullString
	err := r.db.WithContext(ctx).
		Model(&authmodels.User{}).
		Where("id = ?", userID).
		Limit(1).
		Pluck("status", &raw).Error
	if err != nil {
		return authtypes.UserStatusActive
	}

	if !raw.Valid || strings.TrimSpace(raw.String) == "" {
		return authtypes.UserStatusActive
	}

	status, err := authtypes.ParseUserStatus(raw.String)
	if err != nil {
		return authtypes.UserStatusActive
	}

	return status
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

// ListTeams returns a cross-tenant page of teams ordered by created_at desc
// together with the total count of teams.
func (r *Registry) ListTeams(ctx context.Context, limit, offset int) ([]authmodels.Team, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&authmodels.Team{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var teams []authmodels.Team
	err := r.db.WithContext(ctx).
		Model(&authmodels.Team{}).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&teams).Error
	if err != nil {
		return nil, 0, err
	}

	return teams, total, nil
}

// ListServers returns a cross-tenant page of servers ordered by created_at desc
// together with the total count of servers.
func (r *Registry) ListServers(ctx context.Context, limit, offset int) ([]servermodels.Server, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&servermodels.Server{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var servers []servermodels.Server
	err := r.db.WithContext(ctx).
		Model(&servermodels.Server{}).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&servers).Error
	if err != nil {
		return nil, 0, err
	}

	return servers, total, nil
}

// UserByID loads a single user by id, or nil if no such user exists. Used by
// the impersonation flow to read the target user's email/name/current team for
// the minted token. Returns nil (not an error) when the user is absent.
func (r *Registry) UserByID(ctx context.Context, id string) (*authmodels.User, error) {
	if id == "" {
		return nil, nil
	}

	var user authmodels.User
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return &user, nil
}

// CreateImpersonationSession inserts a new impersonation session (start of a
// spectate session). It assigns a ULID id and a started_at timestamp when those
// are not already set on the record.
func (r *Registry) CreateImpersonationSession(ctx context.Context, session *staffmodels.ImpersonationSession) error {
	if session.ID == "" {
		session.ID = util.NewULID()
	}

	if session.StartedAt.IsZero() {
		session.StartedAt = time.Now()
	}

	return r.db.WithContext(ctx).Create(session).Error
}

// EndImpersonationSession stamps ended_at on an active session by id. Only
// sessions that are still active (ended_at IS NULL) are affected.
func (r *Registry) EndImpersonationSession(ctx context.Context, sessionID string, endedAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&staffmodels.ImpersonationSession{}).
		Where("id = ? AND ended_at IS NULL", sessionID).
		Update("ended_at", endedAt).Error
}

// EndActiveImpersonationsForStaff stamps ended_at on ALL of a staff member's
// currently-active sessions. Returns the number of sessions closed.
func (r *Registry) EndActiveImpersonationsForStaff(ctx context.Context, staffID string, endedAt time.Time) (int64, error) {
	res := r.db.WithContext(ctx).
		Model(&staffmodels.ImpersonationSession{}).
		Where("staff_id = ? AND ended_at IS NULL", staffID).
		Update("ended_at", endedAt)
	return res.RowsAffected, res.Error
}

// ActiveImpersonationForStaff returns the staff member's most recent active
// (ended_at IS NULL) session, or nil if none exists.
func (r *Registry) ActiveImpersonationForStaff(ctx context.Context, staffID string) (*staffmodels.ImpersonationSession, error) {
	var session staffmodels.ImpersonationSession
	err := r.db.WithContext(ctx).
		Where("staff_id = ? AND ended_at IS NULL", staffID).
		Order("started_at DESC").
		First(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return &session, nil
}
