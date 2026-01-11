package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/auth/models"
)

// PasswordResetTokenRepository handles password reset token database operations
type PasswordResetTokenRepository struct {
	db *gorm.DB
}

// NewPasswordResetTokenRepository creates a new PasswordResetTokenRepository instance
func NewPasswordResetTokenRepository(db *gorm.DB) *PasswordResetTokenRepository {
	return &PasswordResetTokenRepository{db: db}
}

// Create creates a new password reset token
func (r *PasswordResetTokenRepository) Create(ctx context.Context, token *models.PasswordResetToken) error {
	// Delete any existing token for this email first
	r.db.WithContext(ctx).Delete(&models.PasswordResetToken{}, "email = ?", token.Email)

	return r.db.WithContext(ctx).Create(token).Error
}

// FindByEmail finds a password reset token by email
func (r *PasswordResetTokenRepository) FindByEmail(ctx context.Context, email string) (*models.PasswordResetToken, error) {
	var token models.PasswordResetToken
	err := r.db.WithContext(ctx).First(&token, "email = ?", email).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &token, nil
}

// Delete deletes a password reset token
func (r *PasswordResetTokenRepository) Delete(ctx context.Context, email string) error {
	return r.db.WithContext(ctx).Delete(&models.PasswordResetToken{}, "email = ?", email).Error
}
