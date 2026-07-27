package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/auth/contracts"
	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/mail/templates"
	"github.com/kkz6/launch-go/internal/pkg/signedurl"
)

// EmailVerificationService handles email verification operations
type EmailVerificationService struct {
	repos       contracts.RepositoryRegistry
	config      *config.Config
	emailSender channels.EmailSender
}

// NewEmailVerificationService creates a new EmailVerificationService instance

func NewEmailVerificationService(repos contracts.RepositoryRegistry, cfg *config.Config, senders ...channels.EmailSender) *EmailVerificationService {
	var sender channels.EmailSender
	if len(senders) > 0 {
		sender = senders[0]
	}
	return &EmailVerificationService{
		repos:       repos,
		config:      cfg,
		emailSender: sender,
	}
}

// VerifyEmail verifies a user's email address
func (s *EmailVerificationService) VerifyEmail(ctx context.Context, userID, hash string) error {
	user, err := s.repos.User().FindByID(ctx, userID)
	if err != nil {
		return err
	}

	if user == nil {
		return fiberutil.NotFound()
	}

	// Verify hash matches email
	expectedHash := s.generateEmailHash(user.Email)
	if hash != expectedHash {
		return errors.New("invalid verification link")
	}

	if user.HasVerifiedEmail() {
		return nil // Already verified
	}

	return s.repos.User().MarkEmailAsVerified(ctx, userID)
}

// ResendVerificationEmail resends the email verification
func (s *EmailVerificationService) ResendVerificationEmail(ctx context.Context, userID string) error {
	user, err := s.repos.User().FindByID(ctx, userID)
	if err != nil {
		return err
	}

	if user == nil {
		return fiberutil.NotFound()
	}

	if user.HasVerifiedEmail() {
		return errors.New("email already verified")
	}

	// Some self-hosted installations intentionally run without email
	// configured; preserve the existing successful no-op for those setups.
	if s.emailSender == nil {
		return nil
	}
	verifyURL := s.GenerateVerificationURL(user.ID, user.Email)
	html, plain, err := templates.EmailVerificationEmail(verifyURL)
	if err != nil {
		return s.emailSender.Send(ctx, user.Email, "Verify your email address", plain, false)
	}
	if err := s.emailSender.Send(ctx, user.Email, "Verify your email address", html, true); err != nil {
		return fmt.Errorf("send verification email: %w", err)
	}
	return nil
}

// generateEmailHash generates an HMAC hash for email verification.
// Uses the app key as the HMAC secret so verification links survive JWT secret rotation.
func (s *EmailVerificationService) generateEmailHash(email string) string {
	key := s.config.App.Key
	if key == "" {
		// Fallback to JWT secret if app key is not set
		key = s.config.JWT.Secret
	}

	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(email))

	return hex.EncodeToString(mac.Sum(nil))
}

// GenerateVerificationURL generates a signed URL for email verification
// The URL expires after 60 minutes (similar to Laravel's default)
func (s *EmailVerificationService) GenerateVerificationURL(userID, email string) string {
	hash := s.generateEmailHash(email)
	path := fmt.Sprintf("/auth/verify-email/%s/%s", userID, hash)

	return signedurl.TemporarySign(path, nil, 60*time.Minute)
}
