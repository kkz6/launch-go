package services

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	"github.com/kkz6/launch-go/internal/pkg/mail/templates"
	"github.com/kkz6/launch-go/internal/pkg/security"
)

// PasswordResetService handles password reset operations
type PasswordResetService struct {
	repos       *repositories.Registry
	config      *config.Config
	emailSender channels.EmailSender
}

// NewPasswordResetService creates a new PasswordResetService instance
func NewPasswordResetService(repos *repositories.Registry, cfg *config.Config, emailSender channels.EmailSender) *PasswordResetService {
	return &PasswordResetService{
		repos:       repos,
		config:      cfg,
		emailSender: emailSender,
	}
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
	tokenStr := security.NewTokenGenerator(32).WithEncoding(security.TokenBase64URL).MustGenerate()

	// Hash the token for storage
	hashedToken := s.hashToken(tokenStr)

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

	if s.emailSender == nil {
		return nil
	}

	resetURL := fmt.Sprintf("%s/reset-password?token=%s&email=%s", s.config.App.URL, url.QueryEscape(tokenStr), url.QueryEscape(email))

	htmlContent, _, err := templates.PasswordResetEmail(resetURL, 60)
	if err != nil {
		return err
	}

	return s.emailSender.Send(ctx, email, "Reset Your Password", htmlContent, true)
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

	// Verify token using constant-time comparison to prevent timing attacks
	hashedToken := s.hashToken(req.Token)
	if subtle.ConstantTimeCompare([]byte(hashedToken), []byte(resetToken.Token)) != 1 {
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
	hashedPassword, err := security.HashPassword(req.Password)
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
func (s *PasswordResetService) hashToken(t string) string {
	hash := sha256.Sum256([]byte(t))

	return hex.EncodeToString(hash[:])
}
