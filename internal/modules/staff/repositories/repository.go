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
	billingmodels "github.com/kkz6/launch-go/internal/modules/billing/models"
	billingtypes "github.com/kkz6/launch-go/internal/modules/billing/types"
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

// ListUsersWithBilling returns a cross-tenant page of users ordered by
// created_at desc, the teams each user owns, and the current subscription for
// each of those teams. The work is done in three cheap queries (users page,
// owned teams for that page, current subscriptions for those teams) so there is
// no N+1 fan-out per user/team. The returned maps are keyed by user id (owned
// teams) and team id (current subscription, nil when a team has none).
func (r *Registry) ListUsersWithBilling(ctx context.Context, limit, offset int) ([]authmodels.User, map[string][]authmodels.Team, map[string]*billingmodels.Subscription, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&authmodels.User{}).Count(&total).Error; err != nil {
		return nil, nil, nil, 0, err
	}

	var users []authmodels.User
	err := r.db.WithContext(ctx).
		Model(&authmodels.User{}).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&users).Error
	if err != nil {
		return nil, nil, nil, 0, err
	}

	userIDs := make([]string, 0, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.ID)
	}

	teamsByOwner, err := r.TeamsByOwners(ctx, userIDs)
	if err != nil {
		return nil, nil, nil, 0, err
	}

	teamIDs := make([]string, 0)
	for _, teams := range teamsByOwner {
		for _, team := range teams {
			teamIDs = append(teamIDs, team.ID)
		}
	}

	subscriptionsByTeam, err := r.CurrentSubscriptionsByTeams(ctx, teamIDs)
	if err != nil {
		return nil, nil, nil, 0, err
	}

	return users, teamsByOwner, subscriptionsByTeam, total, nil
}

// TeamsByOwners loads all teams owned by the given user ids in a single query
// and groups them by owner id. Returns an empty map when no ids are supplied.
func (r *Registry) TeamsByOwners(ctx context.Context, userIDs []string) (map[string][]authmodels.Team, error) {
	grouped := make(map[string][]authmodels.Team, len(userIDs))
	if len(userIDs) == 0 {
		return grouped, nil
	}

	var teams []authmodels.Team
	err := r.db.WithContext(ctx).
		Model(&authmodels.Team{}).
		Where("user_id IN ?", userIDs).
		Order("created_at ASC").
		Find(&teams).Error
	if err != nil {
		return nil, err
	}

	for _, team := range teams {
		grouped[team.UserID] = append(grouped[team.UserID], team)
	}

	return grouped, nil
}

// CurrentSubscriptionsByTeams loads subscriptions for the given team ids in a
// single query and selects the most relevant one per team: an active/on_trial
// subscription is preferred, otherwise the newest by id. Teams without any
// subscription are simply absent from the returned map.
func (r *Registry) CurrentSubscriptionsByTeams(ctx context.Context, teamIDs []string) (map[string]*billingmodels.Subscription, error) {
	current := make(map[string]*billingmodels.Subscription, len(teamIDs))
	if len(teamIDs) == 0 {
		return current, nil
	}

	var subscriptions []billingmodels.Subscription
	err := r.db.WithContext(ctx).
		Model(&billingmodels.Subscription{}).
		Where("billable_type IN ? AND billable_id IN ?", billingmodels.TeamBillableTypes(), teamIDs).
		Order("id DESC").
		Find(&subscriptions).Error
	if err != nil {
		return nil, err
	}

	for i := range subscriptions {
		sub := &subscriptions[i]
		existing, ok := current[sub.BillableID]
		if !ok {
			current[sub.BillableID] = sub
			continue
		}

		if isPreferredSubscription(sub.Status) && !isPreferredSubscription(existing.Status) {
			current[sub.BillableID] = sub
		}
	}

	return current, nil
}

// isPreferredSubscription reports whether a status should win when picking a
// team's current subscription: active and on-trial subscriptions are preferred
// over any other status.
func isPreferredSubscription(status billingtypes.SubscriptionStatus) bool {
	return status == billingtypes.SubscriptionStatusActive ||
		status == billingtypes.SubscriptionStatusOnTrial
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
