package services

import (
	"context"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/security"
)

// TwoFactorService handles two-factor authentication operations
type TwoFactorService struct {
	repos  *repositories.Registry
	config *config.Config
}

// NewTwoFactorService creates a new TwoFactorService instance
func NewTwoFactorService(repos *repositories.Registry, cfg *config.Config) *TwoFactorService {
	return &TwoFactorService{
		repos:  repos,
		config: cfg,
	}
}

// EnableTwoFactor initiates 2FA setup
func (s *TwoFactorService) EnableTwoFactor(ctx context.Context, userID string) (*dto.TwoFactorResponse, error) {
	user, err := s.repos.User().FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, fiberutil.NotFound()
	}

	// Generate TOTP secret
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      s.config.App.Name,
		AccountName: user.Email,
	})
	if err != nil {
		return nil, err
	}

	// Store secret (not confirmed yet)
	secret := key.Secret()
	user.TwoFactorSecret = &secret
	user.TwoFactorConfirmedAt = nil

	if err := s.repos.User().Update(ctx, user); err != nil {
		return nil, err
	}

	return &dto.TwoFactorResponse{
		QRCodeURL: key.URL(),
		SecretKey: key.Secret(),
	}, nil
}

// ConfirmTwoFactor confirms 2FA setup
func (s *TwoFactorService) ConfirmTwoFactor(ctx context.Context, userID, code string) error {
	user, err := s.repos.User().FindByID(ctx, userID)
	if err != nil {
		return err
	}

	if user == nil {
		return fiberutil.NotFound()
	}

	if user.TwoFactorSecret == nil {
		return errors.New("two-factor authentication not initiated")
	}

	// Verify code
	valid := totp.Validate(code, *user.TwoFactorSecret)
	if !valid {
		return errors.New("invalid verification code")
	}

	// Confirm 2FA
	now := time.Now()
	user.TwoFactorConfirmedAt = &now

	// Generate recovery codes
	recoveryCodes, err := s.generateRecoveryCodes()
	if err != nil {
		return err
	}

	// Hash recovery codes before storage
	hashedCodes := make([]string, len(recoveryCodes))
	for i, code := range recoveryCodes {
		hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash recovery code: %w", err)
		}
		hashedCodes[i] = string(hash)
	}

	codesStr := strings.Join(hashedCodes, ",")
	user.TwoFactorRecoveryCodes = &codesStr

	return s.repos.User().Update(ctx, user)
}

// DisableTwoFactor disables 2FA
func (s *TwoFactorService) DisableTwoFactor(ctx context.Context, userID, password string) error {
	user, err := s.repos.User().FindByID(ctx, userID)
	if err != nil {
		return err
	}

	if user == nil {
		return fiberutil.NotFound()
	}

	// Verify password
	if !verifyPassword(user.Password, password) {
		return errors.New("invalid password")
	}

	user.TwoFactorSecret = nil
	user.TwoFactorConfirmedAt = nil
	user.TwoFactorRecoveryCodes = nil

	return s.repos.User().Update(ctx, user)
}

// VerifyTwoFactor verifies a 2FA code
func (s *TwoFactorService) VerifyTwoFactor(ctx context.Context, userID, code string) (bool, error) {
	user, err := s.repos.User().FindByID(ctx, userID)
	if err != nil {
		return false, err
	}

	if user == nil {
		return false, fiberutil.NotFound()
	}

	if !user.HasEnabledTwoFactorAuthentication() {
		return true, nil // 2FA not enabled, consider verified
	}

	// Try TOTP code first
	if totp.Validate(code, *user.TwoFactorSecret) {
		return true, nil
	}

	// Try recovery code
	if user.TwoFactorRecoveryCodes != nil {
		codes := strings.Split(*user.TwoFactorRecoveryCodes, ",")
		for i, hashedCode := range codes {
			if bcrypt.CompareHashAndPassword([]byte(hashedCode), []byte(code)) == nil {
				// Remove used recovery code
				codes = append(codes[:i], codes[i+1:]...)
				codesStr := strings.Join(codes, ",")
				user.TwoFactorRecoveryCodes = &codesStr

				if err := s.repos.User().Update(ctx, user); err != nil {
					return false, fmt.Errorf("failed to remove used recovery code: %w", err)
				}

				return true, nil
			}
		}
	}

	return false, nil
}

// GetRecoveryCodes regenerates recovery codes and returns the plaintext versions.
// Stored codes are hashed and cannot be retrieved, so this always generates fresh codes.
func (s *TwoFactorService) GetRecoveryCodes(ctx context.Context, userID string) ([]string, error) {
	return s.RegenerateRecoveryCodes(ctx, userID)
}

// RegenerateRecoveryCodes generates new recovery codes
func (s *TwoFactorService) RegenerateRecoveryCodes(ctx context.Context, userID string) ([]string, error) {
	user, err := s.repos.User().FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, fiberutil.NotFound()
	}

	if !user.HasEnabledTwoFactorAuthentication() {
		return nil, errors.New("two-factor authentication not enabled")
	}

	codes, err := s.generateRecoveryCodes()
	if err != nil {
		return nil, err
	}

	// Hash recovery codes before storage
	hashedCodes := make([]string, len(codes))
	for i, code := range codes {
		hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("failed to hash recovery code: %w", err)
		}
		hashedCodes[i] = string(hash)
	}

	codesStr := strings.Join(hashedCodes, ",")
	user.TwoFactorRecoveryCodes = &codesStr

	if err := s.repos.User().Update(ctx, user); err != nil {
		return nil, err
	}

	return codes, nil
}

// HasTwoFactorEnabled checks if user has 2FA enabled
func (s *TwoFactorService) HasTwoFactorEnabled(ctx context.Context, userID string) (bool, error) {
	user, err := s.repos.User().FindByID(ctx, userID)
	if err != nil {
		return false, err
	}

	if user == nil {
		return false, nil
	}

	return user.HasEnabledTwoFactorAuthentication(), nil
}

// generateRecoveryCodes generates random recovery codes
func (s *TwoFactorService) generateRecoveryCodes() ([]string, error) {
	codes := make([]string, 8)
	for i := range codes {
		t, err := security.NewTokenGenerator(8).WithEncoding(security.TokenBase64URLRaw).Generate()
		if err != nil {
			return nil, errors.New("failed to generate recovery code token")
		}

		// Convert to a readable format
		codes[i] = fmt.Sprintf("%s-%s",
			base32.StdEncoding.EncodeToString([]byte(t[:4]))[:5],
			base32.StdEncoding.EncodeToString([]byte(t[4:]))[:5],
		)
	}

	return codes, nil
}
