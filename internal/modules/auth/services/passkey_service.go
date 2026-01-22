package services

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
	apperrors "github.com/kkz6/launch-go/internal/pkg/errors"
)

// PasskeyService handles passkey management operations
type PasskeyService struct {
	repos *repositories.Registry
}

// NewPasskeyService creates a new PasskeyService instance
func NewPasskeyService(repos *repositories.Registry) *PasskeyService {
	return &PasskeyService{repos: repos}
}

// GetUserPasskeys returns all passkeys for a user
func (s *PasskeyService) GetUserPasskeys(ctx context.Context, userID string) ([]models.Passkey, error) {
	return s.repos.Passkey().FindByUserID(ctx, userID)
}

// GetPasskey retrieves a passkey by ID
func (s *PasskeyService) GetPasskey(ctx context.Context, passkeyID string) (*models.Passkey, error) {
	return s.repos.Passkey().FindByID(ctx, passkeyID)
}

// UpdatePasskeyName updates a passkey's name
func (s *PasskeyService) UpdatePasskeyName(ctx context.Context, passkeyID, userID, name string) error {
	passkey, err := s.repos.Passkey().FindByID(ctx, passkeyID)
	if err != nil {
		return err
	}

	if passkey == nil {
		return apperrors.ErrNotFound
	}

	if passkey.UserID != userID {
		return apperrors.ErrNotFound
	}

	passkey.Name = &name

	return s.repos.Passkey().Update(ctx, passkey)
}

// DeletePasskey deletes a passkey for a user
func (s *PasskeyService) DeletePasskey(ctx context.Context, passkeyID, userID string) error {
	return s.repos.Passkey().DeleteByUserID(ctx, passkeyID, userID)
}
