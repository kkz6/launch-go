package repositories

import (
	"context"
	"database/sql"
	"strings"

	"gorm.io/gorm"

	authmodels "github.com/kkz6/launch-go/internal/modules/auth/models"
	billingmodels "github.com/kkz6/launch-go/internal/modules/billing/models"
	billingtypes "github.com/kkz6/launch-go/internal/modules/billing/types"
	stafftypes "github.com/kkz6/launch-go/internal/modules/staff/types"
)

// HasStaffRoleTx reports whether the given user holds a staff role, reading
// through the supplied gorm handle. The delete transaction uses this (instead
// of StaffRole, which reads through the registry's own handle) so the staff
// guard observes the same snapshot as the rest of the in-tx re-check. Mirrors
// StaffRole's defensive parsing: an unparseable or NULL value is treated as no
// staff role.
func (r *Registry) HasStaffRoleTx(db *gorm.DB, userID string) (bool, error) {
	if userID == "" {
		return false, nil
	}

	var raw sql.NullString
	err := db.
		Model(&authmodels.User{}).
		Where("id = ?", userID).
		Limit(1).
		Pluck("staff_role", &raw).Error
	if err != nil {
		return false, err
	}

	if !raw.Valid || strings.TrimSpace(raw.String) == "" {
		return false, nil
	}

	if _, err := stafftypes.ParseStaffRole(raw.String); err != nil {
		return false, nil
	}

	return true, nil
}

// paidSubscriptionStatuses are the subscription statuses that mark a team as
// having ever held a paying (non-trial) subscription. A pure trial (on_trial)
// or paused state is intentionally excluded: on_trial means they never paid,
// and paused is not in the "ever paid" set the delete guard checks for. The
// delete rule treats any of these as a hard block.
func paidSubscriptionStatuses() []billingtypes.SubscriptionStatus {
	return []billingtypes.SubscriptionStatus{
		billingtypes.SubscriptionStatusActive,
		billingtypes.SubscriptionStatusPastDue,
		billingtypes.SubscriptionStatusUnpaid,
		billingtypes.SubscriptionStatusCancelled,
		billingtypes.SubscriptionStatusExpired,
	}
}

// TeamIDsOwnedBy returns the ids of every team owned by the given user
// (teams.user_id = userID). Returns an empty slice when the user owns none.
func (r *Registry) TeamIDsOwnedBy(ctx context.Context, userID string) ([]string, error) {
	if userID == "" {
		return []string{}, nil
	}

	var ids []string
	err := r.db.WithContext(ctx).
		Model(&authmodels.Team{}).
		Where("user_id = ?", userID).
		Pluck("id", &ids).Error
	if err != nil {
		return nil, err
	}

	return ids, nil
}

// HasPaidOrderForTeams reports whether any of the given teams has a paid order
// (orders.status = 'paid' AND billable_type IN TeamBillableTypes() AND
// billable_id IN teamIDs). Returns false when teamIDs is empty.
func (r *Registry) HasPaidOrderForTeams(ctx context.Context, teamIDs []string) (bool, error) {
	if len(teamIDs) == 0 {
		return false, nil
	}

	return r.HasPaidOrderForTeamsTx(r.db.WithContext(ctx), teamIDs)
}

// HasPaidOrderForTeamsTx runs the paid-order count on the supplied gorm handle
// so the same query can be reused inside the service's delete transaction.
func (r *Registry) HasPaidOrderForTeamsTx(db *gorm.DB, teamIDs []string) (bool, error) {
	if len(teamIDs) == 0 {
		return false, nil
	}

	var count int64
	err := db.
		Model(&billingmodels.Order{}).
		Where("status = ?", billingtypes.OrderStatusPaid).
		Where("billable_type IN ?", billingmodels.TeamBillableTypes()).
		Where("billable_id IN ?", teamIDs).
		Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// HasNonTrialSubscriptionForTeams reports whether any of the given teams has a
// subscription whose status is in the "ever paid" set (active, past_due,
// unpaid, cancelled, expired) — i.e. anything beyond a pure trial. Returns
// false when teamIDs is empty.
func (r *Registry) HasNonTrialSubscriptionForTeams(ctx context.Context, teamIDs []string) (bool, error) {
	if len(teamIDs) == 0 {
		return false, nil
	}

	return r.HasNonTrialSubscriptionForTeamsTx(r.db.WithContext(ctx), teamIDs)
}

// HasNonTrialSubscriptionForTeamsTx runs the non-trial-subscription count on the
// supplied gorm handle so the same query can be reused inside the service's
// delete transaction.
func (r *Registry) HasNonTrialSubscriptionForTeamsTx(db *gorm.DB, teamIDs []string) (bool, error) {
	if len(teamIDs) == 0 {
		return false, nil
	}

	var count int64
	err := db.
		Model(&billingmodels.Subscription{}).
		Where("status IN ?", paidSubscriptionStatuses()).
		Where("billable_type IN ?", billingmodels.TeamBillableTypes()).
		Where("billable_id IN ?", teamIDs).
		Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// DeleteUser deletes the users row with the given id on the supplied gorm
// handle (which is the service's transaction). Deleting a users row cascades at
// the DB level to every resource the user owns, so this must only ever run
// after the delete guards have passed. Returns gorm.ErrRecordNotFound when no
// row matched the id.
func (r *Registry) DeleteUser(db *gorm.DB, userID string) error {
	res := db.Delete(&authmodels.User{}, "id = ?", userID)
	if res.Error != nil {
		return res.Error
	}

	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}
