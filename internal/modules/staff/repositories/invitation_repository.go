package repositories

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	authmodels "github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/pkg/util"
)

// CreatePlatformInvitation inserts a new platform invitation. It assigns a ULID
// id when one is not already set on the record.
func (r *Registry) CreatePlatformInvitation(ctx context.Context, invitation *authmodels.PlatformInvitation) error {
	if invitation.ID == "" {
		invitation.ID = util.NewULID()
	}

	return r.db.WithContext(ctx).Create(invitation).Error
}

// FindPendingInvitationByEmail returns the newest non-accepted, non-expired
// invitation for the given email, or (nil, nil) when none exists.
func (r *Registry) FindPendingInvitationByEmail(ctx context.Context, email string) (*authmodels.PlatformInvitation, error) {
	if email == "" {
		return nil, nil
	}

	var invitation authmodels.PlatformInvitation
	err := r.db.WithContext(ctx).
		Where("email = ? AND accepted_at IS NULL AND expires_at > ?", email, time.Now()).
		Order("created_at DESC").
		First(&invitation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return &invitation, nil
}

// FindInvitationByToken returns the invitation matching the given token, or
// (nil, nil) when none exists. Used by the registration flow (Task 11) to
// consume an invite.
func (r *Registry) FindInvitationByToken(ctx context.Context, token string) (*authmodels.PlatformInvitation, error) {
	if token == "" {
		return nil, nil
	}

	var invitation authmodels.PlatformInvitation
	err := r.db.WithContext(ctx).
		Where("token = ?", token).
		First(&invitation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return &invitation, nil
}

// UserExistsByEmail reports whether a user row with the given email exists.
func (r *Registry) UserExistsByEmail(ctx context.Context, email string) (bool, error) {
	if email == "" {
		return false, nil
	}

	var count int64
	err := r.db.WithContext(ctx).
		Model(&authmodels.User{}).
		Where("email = ?", email).
		Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// ListPendingInvitations returns a page of non-accepted, non-expired
// invitations ordered by created_at desc, together with the total count.
func (r *Registry) ListPendingInvitations(ctx context.Context, limit, offset int) ([]authmodels.PlatformInvitation, int64, error) {
	now := time.Now()

	var total int64
	if err := r.db.WithContext(ctx).
		Model(&authmodels.PlatformInvitation{}).
		Where("accepted_at IS NULL AND expires_at > ?", now).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var invitations []authmodels.PlatformInvitation
	err := r.db.WithContext(ctx).
		Model(&authmodels.PlatformInvitation{}).
		Where("accepted_at IS NULL AND expires_at > ?", now).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&invitations).Error
	if err != nil {
		return nil, 0, err
	}

	return invitations, total, nil
}

// DeleteInvitation deletes an invitation by id. Returns gorm.ErrRecordNotFound
// when no row matched so callers can surface a 404.
func (r *Registry) DeleteInvitation(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&authmodels.PlatformInvitation{})
	if res.Error != nil {
		return res.Error
	}

	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}
