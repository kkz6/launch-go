package services

import (
	"bytes"
	"context"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"image/png"
	"strings"
	"time"

	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/auth/contracts"
	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/security"
)

// TwoFactorService handles two-factor authentication operations
type TwoFactorService struct {
	repos       contracts.RepositoryRegistry
	config      *config.Config
	authService *AuthService
}

// NewTwoFactorService creates a new TwoFactorService instance
func NewTwoFactorService(repos contracts.RepositoryRegistry, cfg *config.Config, authService *AuthService) *TwoFactorService {
	return &TwoFactorService{
		repos:       repos,
		config:      cfg,
		authService: authService,
	}
}

// EnableTwoFactor initiates 2FA setup after verifying the user's password
func (s *TwoFactorService) EnableTwoFactor(ctx context.Context, userID, password string) (*dto.TwoFactorResponse, error) {
	user, err := s.repos.User().FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, fiberutil.NotFound()
	}

	if !security.VerifyPassword(user.Password, password) {
		return nil, fiberutil.BadRequest("Invalid password")
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

	// Generate QR code image
	qrImage, err := key.Image(200, 200)
	if err != nil {
		return nil, fmt.Errorf("failed to generate QR code: %w", err)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, qrImage); err != nil {
		return nil, fmt.Errorf("failed to encode QR code: %w", err)
	}

	qrDataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

	return &dto.TwoFactorResponse{
		QRCodeURL: qrDataURI,
		SecretKey: key.Secret(),
	}, nil
}

// ConfirmTwoFactor confirms 2FA setup and returns the plaintext recovery codes.
// This is the only time recovery codes are returned — they are stored hashed and cannot be retrieved later.
func (s *TwoFactorService) ConfirmTwoFactor(ctx context.Context, userID, currentSessionID, code string) ([]string, error) {
	user, err := s.repos.User().FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, fiberutil.NotFound()
	}

	if user.TwoFactorSecret == nil {
		return nil, errors.New("two-factor authentication not initiated")
	}

	// Verify code
	valid := totp.Validate(code, *user.TwoFactorSecret)
	if !valid {
		return nil, errors.New("invalid verification code")
	}

	// Confirm 2FA
	now := time.Now()
	user.TwoFactorConfirmedAt = &now

	// Generate recovery codes
	recoveryCodes, err := s.generateRecoveryCodes()
	if err != nil {
		return nil, err
	}

	// Hash recovery codes before storage
	hashedCodes := make([]string, len(recoveryCodes))
	for i, code := range recoveryCodes {
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

	// Invalidate all other sessions — 2FA status change is a security event
	if currentSessionID != "" {
		_, _ = s.repos.Session().DeleteAllByUserExcept(ctx, userID, currentSessionID)
	}

	return recoveryCodes, nil
}

// DisableTwoFactor disables 2FA
func (s *TwoFactorService) DisableTwoFactor(ctx context.Context, userID, currentSessionID, password string) error {
	user, err := s.repos.User().FindByID(ctx, userID)
	if err != nil {
		return err
	}

	if user == nil {
		return fiberutil.NotFound()
	}

	// Verify password
	if !security.VerifyPassword(user.Password, password) {
		return errors.New("invalid password")
	}

	user.TwoFactorSecret = nil
	user.TwoFactorConfirmedAt = nil
	user.TwoFactorRecoveryCodes = nil

	if err := s.repos.User().Update(ctx, user); err != nil {
		return err
	}

	// Invalidate all other sessions — 2FA status change is a security event
	if currentSessionID != "" {
		_, _ = s.repos.Session().DeleteAllByUserExcept(ctx, userID, currentSessionID)
	}

	return nil
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

	// Try recovery code within a transaction with row-level locking
	// to prevent concurrent use of the same recovery code
	if user.TwoFactorRecoveryCodes != nil {
		var verified bool
		err := s.repos.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			// Lock the user row to prevent concurrent recovery code usage
			var lockedUser models.User
			if err := tx.Set("gorm:query_option", "FOR UPDATE").
				First(&lockedUser, "id = ?", userID).Error; err != nil {
				return err
			}

			if lockedUser.TwoFactorRecoveryCodes == nil {
				return nil
			}

			codes := strings.Split(*lockedUser.TwoFactorRecoveryCodes, ",")
			for i, hashedCode := range codes {
				if bcrypt.CompareHashAndPassword([]byte(hashedCode), []byte(code)) == nil {
					// Remove used recovery code
					codes = append(codes[:i], codes[i+1:]...)
					codesStr := strings.Join(codes, ",")

					if err := tx.Model(&lockedUser).
						Update("two_factor_recovery_codes", codesStr).Error; err != nil {
						return fmt.Errorf("failed to remove used recovery code: %w", err)
					}

					verified = true

					return nil
				}
			}

			return nil
		})
		if err != nil {
			return false, err
		}
		if verified {
			return true, nil
		}
	}

	return false, nil
}

// GetRecoveryCodeCount returns the number of remaining recovery codes.
// Stored codes are hashed and cannot be retrieved. Use RegenerateRecoveryCodes to generate new ones.
func (s *TwoFactorService) GetRecoveryCodeCount(ctx context.Context, userID string) (int, error) {
	user, err := s.repos.User().FindByID(ctx, userID)
	if err != nil {
		return 0, err
	}

	if user == nil {
		return 0, fiberutil.NotFound()
	}

	if !user.HasEnabledTwoFactorAuthentication() {
		return 0, errors.New("two-factor authentication not enabled")
	}

	if user.TwoFactorRecoveryCodes == nil {
		return 0, nil
	}

	codes := strings.Split(*user.TwoFactorRecoveryCodes, ",")
	count := 0
	for _, c := range codes {
		if strings.TrimSpace(c) != "" {
			count++
		}
	}

	return count, nil
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

// CompleteTwoFactorChallenge verifies a 2FA code and completes the login flow.
// It looks up the challenge token from cache, verifies the TOTP/recovery code,
// creates a session, and returns auth tokens on success.
func (s *TwoFactorService) CompleteTwoFactorChallenge(ctx context.Context, challengeToken, code string) (*dto.AuthResponse, error) {
	// Look up the challenge data (consumes the token)
	challenge, err := s.authService.LookupTwoFactorChallenge(ctx, challengeToken)
	if err != nil {
		return nil, err
	}

	// Verify the 2FA code
	valid, err := s.VerifyTwoFactor(ctx, challenge.UserID, code)
	if err != nil {
		return nil, err
	}

	if !valid {
		return nil, fiberutil.Unauthorized()
	}

	// Code verified — create session and return tokens
	return s.authService.CompleteTwoFactorLogin(ctx, challenge.UserID, challenge.IPAddress, challenge.UserAgent)
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
