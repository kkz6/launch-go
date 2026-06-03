package services

import (
	"context"

	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
)

// SetUserStatus suspends or unsuspends a customer account. The actor is the
// authenticated staff member performing the action (used for the self-guard).
//
// Guards, in order:
//   - the target must exist (ErrUserNotFound otherwise);
//   - the target must not be the actor (ErrCannotSuspendSelf), so a staffer
//     can't lock themselves out;
//   - the target must not hold a staff role (ErrCannotSuspendStaff), so staff
//     and peers can't be frozen through this endpoint.
//
// Flipping status to suspended takes effect immediately: Auth() rejects
// suspended users on every request.
func (s *Service) SetUserStatus(ctx context.Context, actorStaffID, targetUserID string, status authtypes.UserStatus) error {
	exists, err := s.repos.UserExists(ctx, targetUserID)
	if err != nil {
		return err
	}

	if !exists {
		return ErrUserNotFound
	}

	if actorStaffID != "" && actorStaffID == targetUserID {
		return ErrCannotSuspendSelf
	}

	if s.repos.StaffRole(ctx, targetUserID) != nil {
		return ErrCannotSuspendStaff
	}

	return s.repos.SetUserStatus(ctx, targetUserID, status)
}
