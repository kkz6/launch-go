package services

import (
	"context"
	"errors"

	"gorm.io/gorm"

	authmodels "github.com/kkz6/launch-go/internal/modules/auth/models"
)

// Human-readable reasons surfaced to the 409 when a user is not deletable.
const (
	reasonStaffRole        = "user holds a staff role"
	reasonPaidOrder        = "user has a paid order"
	reasonPaidSubscription = "user has or had a paid subscription"
)

// CanDeleteUser reports whether the target user may be hard-deleted, and if not,
// a human-readable reason. A user is deletable ONLY if, across every team they
// own:
//   - they hold no staff role, AND
//   - none of their teams has a paid order, AND
//   - none of their teams has a non-trial subscription (active, past_due,
//     unpaid, cancelled or expired). A pure trial / no subscription is fine.
//
// Deleting a user cascades at the DB level to all their owned resources, so the
// guard is deliberately conservative: any signal that the user ever paid blocks
// the delete. A missing target returns ErrUserNotFound (mapped to 404).
func (s *Service) CanDeleteUser(ctx context.Context, userID string) (bool, string, error) {
	exists, err := s.repos.UserExists(ctx, userID)
	if err != nil {
		return false, "", err
	}

	if !exists {
		return false, "", ErrUserNotFound
	}

	if s.repos.StaffRole(ctx, userID) != nil {
		return false, reasonStaffRole, nil
	}

	teamIDs, err := s.repos.TeamIDsOwnedBy(ctx, userID)
	if err != nil {
		return false, "", err
	}

	hasPaidOrder, err := s.repos.HasPaidOrderForTeams(ctx, teamIDs)
	if err != nil {
		return false, "", err
	}

	if hasPaidOrder {
		return false, reasonPaidOrder, nil
	}

	hasPaidSubscription, err := s.repos.HasNonTrialSubscriptionForTeams(ctx, teamIDs)
	if err != nil {
		return false, "", err
	}

	if hasPaidSubscription {
		return false, reasonPaidSubscription, nil
	}

	return true, "", nil
}

// DeleteUser hard-deletes the target user. The delete is destructive and
// cascades at the DB level to every resource the user owns, so it is guarded
// tightly:
//   - the actor may not delete their own account (ErrCannotDeleteSelf);
//   - the whole operation runs in a transaction, and the deletable guards are
//     re-evaluated INSIDE that transaction against the same rows that will be
//     deleted, so a paid order/subscription created between the handler's
//     CanDeleteUser check and here still blocks the delete (returns a
//     NotDeletableError carrying the reason);
//   - a missing target returns ErrUserNotFound.
//
// The handler is expected to call CanDeleteUser first to surface the reason for
// a 409; this method re-checks regardless so a lost race can never delete a
// paying user.
func (s *Service) DeleteUser(ctx context.Context, actorStaffID, targetUserID string) error {
	if actorStaffID != "" && actorStaffID == targetUserID {
		return ErrCannotDeleteSelf
	}

	return s.repos.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var exists int64
		if err := tx.Model(&authmodels.User{}).Where("id = ?", targetUserID).Count(&exists).Error; err != nil {
			return err
		}

		if exists == 0 {
			return ErrUserNotFound
		}

		reason, err := s.deletableReasonInTx(tx, targetUserID)
		if err != nil {
			return err
		}

		if reason != "" {
			return newNotDeletableError(reason)
		}

		return s.repos.DeleteUser(tx, targetUserID)
	})
}

// deletableReasonInTx re-evaluates the delete guards against the supplied
// transaction handle and returns a non-empty reason when the user must NOT be
// deleted. It mirrors CanDeleteUser's rules but reads through tx so the checks
// and the delete observe a single consistent snapshot.
func (s *Service) deletableReasonInTx(tx *gorm.DB, userID string) (string, error) {
	isStaff, err := s.repos.HasStaffRoleTx(tx, userID)
	if err != nil {
		return "", err
	}

	if isStaff {
		return reasonStaffRole, nil
	}

	var teamIDs []string
	if err := tx.Model(&authmodels.Team{}).Where("user_id = ?", userID).Pluck("id", &teamIDs).Error; err != nil {
		return "", err
	}

	hasPaidOrder, err := s.repos.HasPaidOrderForTeamsTx(tx, teamIDs)
	if err != nil {
		return "", err
	}

	if hasPaidOrder {
		return reasonPaidOrder, nil
	}

	hasPaidSubscription, err := s.repos.HasNonTrialSubscriptionForTeamsTx(tx, teamIDs)
	if err != nil {
		return "", err
	}

	if hasPaidSubscription {
		return reasonPaidSubscription, nil
	}

	return "", nil
}

// asNotDeletable extracts the reason from a NotDeletableError, reporting whether
// err was one. Convenience for callers/tests that need the reason.
func asNotDeletable(err error) (*NotDeletableError, bool) {
	var nde *NotDeletableError
	if errors.As(err, &nde) {
		return nde, true
	}

	return nil, false
}
