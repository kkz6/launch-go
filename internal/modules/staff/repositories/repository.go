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
	sitemodels "github.com/kkz6/launch-go/internal/modules/site/models"
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

// DB exposes the underlying gorm handle for read-only aggregate queries that
// don't warrant a dedicated repository method (e.g. the overview dashboard's
// sums and counts).
func (r *Registry) DB() *gorm.DB {
	return r.db
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

// UserExists reports whether a user row with the given id exists. Used by the
// suspend/unsuspend flow to distinguish a missing target (404) from a no-op
// status update (e.g. setting an already-suspended user to suspended).
func (r *Registry) UserExists(ctx context.Context, userID string) (bool, error) {
	if userID == "" {
		return false, nil
	}

	var count int64
	err := r.db.WithContext(ctx).
		Model(&authmodels.User{}).
		Where("id = ?", userID).
		Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// SetUserStatus updates a user's status column. Returns gorm.ErrRecordNotFound
// when no row matched the id so callers can surface a 404.
func (r *Registry) SetUserStatus(ctx context.Context, userID string, status authtypes.UserStatus) error {
	res := r.db.WithContext(ctx).
		Model(&authmodels.User{}).
		Where("id = ?", userID).
		Update("status", status)
	if res.Error != nil {
		return res.Error
	}

	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

// ListUsersWithBilling returns a cross-tenant page of users ordered by
// created_at desc, the teams each user owns, and the current subscription for
// each of those teams. The work is done in three cheap queries (users page,
// owned teams for that page, current subscriptions for those teams) so there is
// no N+1 fan-out per user/team. The returned maps are keyed by user id (owned
// teams) and team id (current subscription, nil when a team has none).
// ListUsersOptions carries the page + toolbar state for the users listing.
// Search matches name/email (case-insensitive), Status filters by exact status
// ("" = all), and SortColumn/SortDir order the result (whitelisted by the repo,
// defaulting to created_at desc).
type ListUsersOptions struct {
	Limit      int
	Offset     int
	Search     string
	Status     string
	SortColumn string
	SortDir    string
}

func (r *Registry) ListUsersWithBilling(ctx context.Context, opts ListUsersOptions) ([]authmodels.User, map[string][]authmodels.Team, map[string]*billingmodels.Subscription, int64, error) {
	base := r.db.WithContext(ctx).Model(&authmodels.User{})

	if search := strings.TrimSpace(opts.Search); search != "" {
		like := "%" + strings.ToLower(search) + "%"
		base = base.Where("LOWER(name) LIKE ? OR LOWER(email) LIKE ?", like, like)
	}
	if opts.Status != "" {
		base = base.Where("status = ?", opts.Status)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, nil, nil, 0, err
	}

	// Whitelist the sort column to avoid SQL injection via the sort param.
	sortColumn := "created_at"
	switch opts.SortColumn {
	case "name", "email", "status", "created_at":
		sortColumn = opts.SortColumn
	}
	sortDir := "DESC"
	if strings.EqualFold(opts.SortDir, "asc") {
		sortDir = "ASC"
	}

	var users []authmodels.User
	err := base.
		Order(sortColumn + " " + sortDir).
		Limit(opts.Limit).
		Offset(opts.Offset).
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

// ServersByTeamIDs loads all servers owned by the given team ids in a single
// query, ordered by created_at desc. Returns an empty slice (no query) when no
// team ids are supplied.
func (r *Registry) ServersByTeamIDs(ctx context.Context, teamIDs []string) ([]servermodels.Server, error) {
	if len(teamIDs) == 0 {
		return []servermodels.Server{}, nil
	}

	var servers []servermodels.Server
	err := r.db.WithContext(ctx).
		Model(&servermodels.Server{}).
		Where("team_id IN ?", teamIDs).
		Order("created_at DESC").
		Find(&servers).Error
	if err != nil {
		return nil, err
	}

	return servers, nil
}

// SitesByServerIDs loads all sites hosted on the given server ids in a single
// query, ordered by created_at desc. Returns an empty slice (no query) when no
// server ids are supplied.
func (r *Registry) SitesByServerIDs(ctx context.Context, serverIDs []string) ([]sitemodels.Site, error) {
	if len(serverIDs) == 0 {
		return []sitemodels.Site{}, nil
	}

	var sites []sitemodels.Site
	err := r.db.WithContext(ctx).
		Model(&sitemodels.Site{}).
		Where("server_id IN ?", serverIDs).
		Order("created_at DESC").
		Find(&sites).Error
	if err != nil {
		return nil, err
	}

	return sites, nil
}

// AllSubscriptionsByTeams loads every subscription for the given team ids in a
// single query, ordered by id desc. Unlike CurrentSubscriptionsByTeams (one
// current subscription per team), this returns the full history. Returns an
// empty slice (no query) when no team ids are supplied.
func (r *Registry) AllSubscriptionsByTeams(ctx context.Context, teamIDs []string) ([]billingmodels.Subscription, error) {
	if len(teamIDs) == 0 {
		return []billingmodels.Subscription{}, nil
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

	return subscriptions, nil
}

// ServerByID loads a single server by id across tenants, or nil when no server
// matches. Returns nil (not an error) when absent.
func (r *Registry) ServerByID(ctx context.Context, id string) (*servermodels.Server, error) {
	if id == "" {
		return nil, nil
	}

	var server servermodels.Server
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&server).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &server, nil
}

// TeamByID loads a single team by id across tenants, or nil when no team
// matches. Returns nil (not an error) when absent.
func (r *Registry) TeamByID(ctx context.Context, id string) (*authmodels.Team, error) {
	if id == "" {
		return nil, nil
	}

	var team authmodels.Team
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&team).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &team, nil
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
