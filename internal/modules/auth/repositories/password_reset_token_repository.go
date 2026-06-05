package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// PasswordResetTokenRepository handles password reset token database operations
type PasswordResetTokenRepository struct {
	repository.Base[models.PasswordResetToken]
}

// NewPasswordResetTokenRepository creates a new PasswordResetTokenRepository instance
func NewPasswordResetTokenRepository(db *gorm.DB) *PasswordResetTokenRepository {
	return &PasswordResetTokenRepository{
		Base: repository.NewBase[models.PasswordResetToken](db),
	}
}

// Create creates a new password reset token
func (r *PasswordResetTokenRepository) Create(ctx context.Context, token *models.PasswordResetToken) error {
	// Delete any existing token for this email first
	r.DB.WithContext(ctx).Delete(&models.PasswordResetToken{}, "email = ?", token.Email)

	return r.Base.Create(ctx, token)
}

// FindByEmail finds a password reset token by email
func (r *PasswordResetTokenRepository) FindByEmail(ctx context.Context, email string) (*models.PasswordResetToken, error) {
	return repository.FindOneOrNil[models.PasswordResetToken](ctx, r.DB,
		repository.WithEmail(email),
	)
}

// Delete deletes a password reset token by email
func (r *PasswordResetTokenRepository) Delete(ctx context.Context, email string) error {
	return r.DB.WithContext(ctx).Delete(&models.PasswordResetToken{}, "email = ?", email).Error
}
