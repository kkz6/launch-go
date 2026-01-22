package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/signedurl"
)

// EmailVerificationService handles email verification operations
type EmailVerificationService struct {
	repos  *repositories.Registry
	config *config.Config
}

// NewEmailVerificationService creates a new EmailVerificationService instance
func NewEmailVerificationService(repos *repositories.Registry, cfg *config.Config) *EmailVerificationService {
	return &EmailVerificationService{
		repos:  repos,
		config: cfg,
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

	// In a production environment, you would send the verification email here
	return nil
}

// generateEmailHash generates a hash for email verification
func (s *EmailVerificationService) generateEmailHash(email string) string {
	hash := sha256.Sum256([]byte(email + s.config.JWT.Secret))

	return hex.EncodeToString(hash[:])
}

// GenerateVerificationURL generates a signed URL for email verification
// The URL expires after 60 minutes (similar to Laravel's default)
func (s *EmailVerificationService) GenerateVerificationURL(userID, email string) string {
	hash := s.generateEmailHash(email)
	path := fmt.Sprintf("/api/auth/verify-email/%s/%s", userID, hash)

	return signedurl.TemporarySign(path, nil, 60*time.Minute)
}
