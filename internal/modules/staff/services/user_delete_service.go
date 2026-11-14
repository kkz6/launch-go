package services

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	authmodels "github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
)

// Human-readable reasons surfaced to the 409 when a user is not deletable.
const (
	reasonStaffRole        = "user holds a staff role"
	reasonPaidOrder        = "user has a paid order"
	reasonPaidSubscription = "user has or had a paid subscription"
)

// CanDeleteUser reports whether deletion can proceed without a billing
// acknowledgment, along with the reason when one is required or staff access
// prevents deletion.
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

// DeleteUser hard-deletes the account and its owned resources. Billing history
// requires an acknowledgment from the actor.
func (s *Service) DeleteUser(ctx context.Context, actorStaffID, targetUserID string) error {
	return s.DeleteUserWithBillingConfirmation(ctx, actorStaffID, targetUserID, "")
}

func (s *Service) DeleteUserWithBillingConfirmation(ctx context.Context, actorStaffID, targetUserID, billingConfirmation string) error {
	if actorStaffID != "" && actorStaffID == targetUserID {
		return ErrCannotDeleteSelf
	}

	err := s.repos.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Share the target-row lock with staff-role changes so a concurrent
		// promotion cannot slip between the role guard and the final DELETE.
		var target authmodels.User
		err := tx.Select("id").Clauses(clause.Locking{Strength: "UPDATE"}).First(&target, "id = ?", targetUserID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		if err != nil {
			return err
		}

		reason, err := s.deletableReasonInTx(tx, targetUserID)
		if err != nil {
			return err
		}

		if reason != "" {
			refusal := &NotDeletableError{Reason: reason}
			if code := refusal.BillingHistoryCode(); code == "" || code != billingConfirmation {
				return refusal
			}
		}

		return s.repos.DeleteUser(tx, targetUserID)
	})
	if err != nil {
		return err
	}
	s.broadcastAdminChange(broadcast.AdminResourceChange{Resource: "user", Action: "deleted", UserID: targetUserID})
	return nil
}

// deletableReasonInTx checks the same conditions as CanDeleteUser while the
// account is locked for deletion.
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
