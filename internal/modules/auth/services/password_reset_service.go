package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
)

// PasswordResetService handles password reset operations
type PasswordResetService struct {
	repo *repositories.Repository
}

// NewPasswordResetService creates a new PasswordResetService instance
func NewPasswordResetService(repo *repositories.Repository) *PasswordResetService {
	return &PasswordResetService{repo: repo}
}

// SendPasswordResetLink sends a password reset email
func (s *PasswordResetService) SendPasswordResetLink(ctx context.Context, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))

	user, err := s.repo.FindUserByEmail(ctx, email)
	if err != nil {
		return err
	}

	// Always return success to prevent email enumeration
	if user == nil {
		return nil
	}

	// Generate token
	token, err := models.GenerateToken(32)
	if err != nil {
		return err
	}

	// Hash the token for storage
	hashedToken := s.hashToken(token)

	// Store token
	resetToken := &models.PasswordResetToken{
		Email:     email,
		Token:     hashedToken,
		CreatedAt: time.Now(),
	}

	if err := s.repo.CreatePasswordResetToken(ctx, resetToken); err != nil {
		return err
	}

	// In a production environment, you would send the reset email here
	return nil
}

// ResetPassword resets a user's password
func (s *PasswordResetService) ResetPassword(ctx context.Context, req *dto.ResetPasswordRequest) error {
	req.Normalize()

	resetToken, err := s.repo.FindPasswordResetToken(ctx, req.Email)
	if err != nil {
		return err
	}

	if resetToken == nil {
		return errors.New("invalid or expired reset token")
	}

	// Check if token expired
	if resetToken.IsExpired() {
		s.repo.DeletePasswordResetToken(ctx, req.Email)

		return errors.New("invalid or expired reset token")
	}

	// Verify token
	hashedToken := s.hashToken(req.Token)
	if hashedToken != resetToken.Token {
		return errors.New("invalid or expired reset token")
	}

	// Find user
	user, err := s.repo.FindUserByEmail(ctx, req.Email)
	if err != nil {
		return err
	}

	if user == nil {
		return errors.New("user not found")
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.Password = string(hashedPassword)

	if err := s.repo.UpdateUser(ctx, user); err != nil {
		return err
	}

	// Delete the reset token
	return s.repo.DeletePasswordResetToken(ctx, req.Email)
}

// hashToken hashes a token for secure storage
func (s *PasswordResetService) hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))

	return hex.EncodeToString(hash[:])
}
