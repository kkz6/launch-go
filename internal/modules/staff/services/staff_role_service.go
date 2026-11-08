package services

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	authmodels "github.com/kkz6/launch-go/internal/modules/auth/models"
	stafftypes "github.com/kkz6/launch-go/internal/modules/staff/types"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
)

var (
	ErrInvalidStaffRole         = errors.New("staff_role must be null, support, or super_admin")
	ErrStaffRoleForbidden       = errors.New("super administrator access required")
	ErrCannotChangeOwnStaffRole = errors.New("you cannot change your own staff role")
	ErrLastSuperAdmin           = errors.New("the last super administrator cannot be demoted")
)

type StaffRoleResult struct {
	ID        string                `json:"id"`
	StaffRole *stafftypes.StaffRole `json:"staff_role"`
}

// UpdateStaffRole serializes role changes on the current super administrators,
// then locks the actor and target. Rechecking the actor inside the transaction
// prevents an in-flight request from retaining privileges after demotion.
func (s *Service) UpdateStaffRole(ctx context.Context, actorID, targetID string, role *stafftypes.StaffRole) (*StaffRoleResult, error) {
	if role != nil && !role.IsValid() {
		return nil, ErrInvalidStaffRole
	}
	if actorID == "" {
		return nil, ErrStaffRoleForbidden
	}
	result := &StaffRoleResult{ID: targetID, StaffRole: role}
	err := s.repos.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var admins []authmodels.User
		if err := tx.Select("id").Where("staff_role = ?", stafftypes.StaffRoleSuperAdmin).
			Order("id").Clauses(clause.Locking{Strength: "UPDATE"}).Find(&admins).Error; err != nil {
			return err
		}
		var users []authmodels.User
		if err := tx.Select("id", "staff_role", "status").Where("id IN ?", []string{actorID, targetID}).
			Order("id").Clauses(clause.Locking{Strength: "UPDATE"}).Find(&users).Error; err != nil {
			return err
		}
		var actor, target *authmodels.User
		for i := range users {
			if users[i].ID == actorID {
				actor = &users[i]
			}
			if users[i].ID == targetID {
				target = &users[i]
			}
		}
		if actor == nil || actor.StaffRole == nil || *actor.StaffRole != stafftypes.StaffRoleSuperAdmin || actor.IsSuspended() {
			return ErrStaffRoleForbidden
		}
		if target == nil {
			return ErrUserNotFound
		}
		if actorID == targetID {
			return ErrCannotChangeOwnStaffRole
		}
		if target.StaffRole != nil && *target.StaffRole == stafftypes.StaffRoleSuperAdmin && (role == nil || *role != stafftypes.StaffRoleSuperAdmin) && len(admins) <= 1 {
			return ErrLastSuperAdmin
		}
		if roleValue(target.StaffRole) == roleValue(role) {
			return nil
		}
		if err := tx.Model(&authmodels.User{}).Where("id = ?", targetID).Update("staff_role", roleValue(role)).Error; err != nil {
			return err
		}
		_, err := activity.New(tx).WithContext(ctx).UseLog("staff").OnModel("User", targetID).
			CausedByUser(actorID).WithEvent("staff_role_updated").WithProperties(map[string]any{
			"old_staff_role": roleValue(target.StaffRole), "new_staff_role": roleValue(role),
		}).Log("Staff role updated")
		return err
	})
	if err != nil {
		return nil, err
	}
	// RequireStaff reads the role from the database on every request; there is
	// no process-local role cache to invalidate after this commit.
	s.broadcastAdminChange(broadcast.AdminResourceChange{Resource: "user", Action: "staff_role_updated", UserID: targetID})
	return result, nil
}

func roleValue(role *stafftypes.StaffRole) any {
	if role == nil {
		return nil
	}
	return role.String()
}
