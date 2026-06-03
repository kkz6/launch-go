package services

import (
	"context"

	authmodels "github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/staff/repositories"
	stafftypes "github.com/kkz6/launch-go/internal/modules/staff/types"
)

// Service exposes the staff module's business operations.
type Service struct {
	repos *repositories.Registry
}

// NewService creates a new staff service.
func NewService(repos *repositories.Registry) *Service {
	return &Service{repos: repos}
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
