package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
	"github.com/kkz6/launch-go/internal/pkg/cryptoutil"
)

// PasswordResetService handles password reset operations
type PasswordResetService struct {
	repos *repositories.Registry
}

// NewPasswordResetService creates a new PasswordResetService instance
func NewPasswordResetService(repos *repositories.Registry) *PasswordResetService {
	return &PasswordResetService{repos: repos}
}

// SendPasswordResetLink sends a password reset email
func (s *PasswordResetService) SendPasswordResetLink(ctx context.Context, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))

	user, err := s.repos.User().FindByEmail(ctx, email)
	if err != nil {
		return err
	}

	// Always return success to prevent email enumeration
	if user == nil {
		return nil
	}

	// Generate token
	token := cryptoutil.GenerateToken(32)

	// Hash the token for storage
	hashedToken := s.hashToken(token)

	// Store token
	now := time.Now()
	resetToken := &models.PasswordResetToken{
		Email:     email,
		Token:     hashedToken,
		CreatedAt: &now,
	}

	if err := s.repos.PasswordResetToken().Create(ctx, resetToken); err != nil {
		return err
	}

	// In a production environment, you would send the reset email here
	return nil
}

// ResetPassword resets a user's password
func (s *PasswordResetService) ResetPassword(ctx context.Context, req *dto.ResetPasswordRequest) error {
	req.Normalize()

	resetToken, err := s.repos.PasswordResetToken().FindByEmail(ctx, req.Email)
	if err != nil {
		return err
	}

	if resetToken == nil {
		return errors.New("invalid or expired reset token")
	}

	// Check if token expired
	if resetToken.IsExpired() {
		s.repos.PasswordResetToken().Delete(ctx, req.Email)

		return errors.New("invalid or expired reset token")
	}

	// Verify token
	hashedToken := s.hashToken(req.Token)
	if hashedToken != resetToken.Token {
		return errors.New("invalid or expired reset token")
	}

	// Find user
	user, err := s.repos.User().FindByEmail(ctx, req.Email)
	if err != nil {
		return err
	}

	if user == nil {
		return errors.New("user not found")
	}

	// Hash new password
	hashedPassword, err := cryptoutil.HashPassword(req.Password)
	if err != nil {
		return err
	}

	user.Password = hashedPassword

	if err := s.repos.User().Update(ctx, user); err != nil {
		return err
	}

	// Delete the reset token
	return s.repos.PasswordResetToken().Delete(ctx, req.Email)
}

// hashToken hashes a token for secure storage
func (s *PasswordResetService) hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))

	return hex.EncodeToString(hash[:])
}
