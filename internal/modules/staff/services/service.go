package services

import (
	"context"

	authmodels "github.com/kkz6/launch-go/internal/modules/auth/models"
	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	staffdto "github.com/kkz6/launch-go/internal/modules/staff/dto"
	"github.com/kkz6/launch-go/internal/modules/staff/repositories"
	stafftypes "github.com/kkz6/launch-go/internal/modules/staff/types"
)

// ServerLogReader is the minimal cross-module dependency the staff module needs
// to surface a server's recent operational activity. It is satisfied by the
// server module's TaskRepository (FindByServer) and injected from main.go via
// SetServerLogReader, so the staff module never imports the server module's
// repositories/services directly.
//
// Security note: the returned Task carries a `Server *Server` relation tagged
// json:"server". This endpoint relies on FindByServer NOT preloading that
// relation — secret server fields are json:"-", but a preloaded raw Server
// would still bypass the ServerSummary allow-list used by ListServers. If
// FindByServer ever starts preloading Server, map tasks to a summary DTO here.
type ServerLogReader interface {
	FindByServer(ctx context.Context, serverID string, limit int) ([]servermodels.Task, error)
}

// Service exposes the staff module's business operations.
type Service struct {
	repos     *repositories.Registry
	logReader ServerLogReader

	// jwtSecret signs the scoped impersonation token. Wired from main.go/module
	// via SetJWTSecret out of config; never read from the environment directly.
	jwtSecret string

	// monthlyEquivByProduct maps a billing product id to its monthly-equivalent
	// price in cents (yearly products divided by 12). Used to compute MRR. When
	// nil it is lazily built from the default plan config; tests inject a fixed
	// map via SetMonthlyEquivByProduct for determinism.
	monthlyEquivByProduct map[string]int64
}

// NewService creates a new staff service.
func NewService(repos *repositories.Registry) *Service {
	return &Service{repos: repos}
}

// SetJWTSecret wires the HS256 signing secret used to mint scoped impersonation
// tokens. Injected from the module constructor out of config (config.JWT.Secret),
// mirroring how SetServerLogReader wires its cross-module dependency.
func (s *Service) SetJWTSecret(secret string) {
	s.jwtSecret = secret
}

// SetServerLogReader wires the cross-module reader used to fetch a server's
// recent task/activity records. Injected from main.go.
func (s *Service) SetServerLogReader(reader ServerLogReader) {
	s.logReader = reader
}

// StaffRoleForUser resolves a user's staff role (nil = not staff). This is the
// lookup wired into the RequireStaff middleware.
func (s *Service) StaffRoleForUser(ctx context.Context, userID string) *stafftypes.StaffRole {
	return s.repos.StaffRole(ctx, userID)
}

// UserStatusForUser resolves a user's account status (active by safe default).
// This is the lookup wired into the suspended-account enforcement at the Auth
// chokepoint via middleware.InitUserStatus.
func (s *Service) UserStatusForUser(ctx context.Context, userID string) authtypes.UserStatus {
	return s.repos.UserStatus(ctx, userID)
}

// ListUsersWithBilling returns a cross-tenant page of users as admin rows, each
// carrying the teams the user owns and every team's current subscription
// status. The repo fetches users + teams + subscriptions in three queries (no
// N+1); this method assembles them into the back-office DTO, exposing only the
// allow-listed fields (no passwords, 2FA secrets, customer ids or card data).
func (s *Service) ListUsersWithBilling(ctx context.Context, limit, offset int) ([]staffdto.AdminUserRow, int64, error) {
	users, teamsByOwner, subscriptionsByTeam, total, err := s.repos.ListUsersWithBilling(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	rows := make([]staffdto.AdminUserRow, 0, len(users))
	for i := range users {
		user := users[i]

		var staffRole *string
		if user.StaffRole != nil {
			role := user.StaffRole.String()
			staffRole = &role
		}

		ownedTeams := teamsByOwner[user.ID]
		teams := make([]staffdto.AdminTeam, 0, len(ownedTeams))
		for _, team := range ownedTeams {
			adminTeam := staffdto.AdminTeam{
				ID:           team.ID,
				Name:         team.Name,
				PersonalTeam: team.PersonalTeam,
			}

			if sub := subscriptionsByTeam[team.ID]; sub != nil {
				adminTeam.Subscription = &staffdto.AdminTeamSubscription{
					Status:      sub.Status.String(),
					TrialEndsAt: sub.TrialEndsAt,
				}
			}

			teams = append(teams, adminTeam)
		}

		rows = append(rows, staffdto.AdminUserRow{
			ID:        user.ID,
			Name:      user.Name,
			Email:     user.Email,
			StaffRole: staffRole,
			Status:    string(user.Status),
			CreatedAt: user.CreatedAt,
			Teams:     teams,
		})
	}

	return rows, total, nil
}

// ListTeams returns a cross-tenant page of teams and the total count.
func (s *Service) ListTeams(ctx context.Context, limit, offset int) ([]authmodels.Team, int64, error) {
	return s.repos.ListTeams(ctx, limit, offset)
}

// ListServers returns a cross-tenant page of servers and the total count.
func (s *Service) ListServers(ctx context.Context, limit, offset int) ([]servermodels.Server, int64, error) {
	return s.repos.ListServers(ctx, limit, offset)
}

// ServerLogs returns a server's recent task/activity records. Returns
// ErrServerLogReaderUnavailable if the cross-module reader was not wired.
func (s *Service) ServerLogs(ctx context.Context, serverID string, limit int) ([]servermodels.Task, error) {
	if s.logReader == nil {
		return nil, ErrServerLogReaderUnavailable
	}

	return s.logReader.FindByServer(ctx, serverID, limit)
}
