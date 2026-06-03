package services

import (
	"context"

	authmodels "github.com/kkz6/launch-go/internal/modules/auth/models"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
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

// ListUsers returns a cross-tenant page of users and the total count.
func (s *Service) ListUsers(ctx context.Context, limit, offset int) ([]authmodels.User, int64, error) {
	return s.repos.ListUsers(ctx, limit, offset)
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
