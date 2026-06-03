package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"

	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"

	"github.com/kkz6/launch-go/internal/pkg/security"
)

// bearerToken extracts the raw bearer token from the Authorization header.
func bearerToken(c *fiber.Ctx) (string, error) {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return "", errors.New("missing authorization header")
	}

	parts := strings.Fields(authHeader)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("invalid authorization header format")
	}

	return parts[1], nil
}

// setAuthContext sets the user context from validated JWT claims.
func setAuthContext(c *fiber.Ctx, claims jwt.MapClaims) {
	userID, _ := security.ExtractJWTClaim(claims, "sub")
	fiberctx.SetUserContext(c, userID, nil)
	c.Locals("email", claims["email"])
	c.Locals("name", claims["name"])

	if sessionID, ok := claims["session_id"].(string); ok {
		c.Locals("sessionID", sessionID)
	}

	// Surface the read-only flag so the auth chokepoint can enforce the
	// spectate contract. Only present on minted impersonation tokens; absent
	// on normal staff/customer tokens.
	if _, ok := claims["impersonation_sid"].(string); ok {
		if ro, ok := claims["read_only"].(bool); ok {
			c.Locals("impersonationReadOnly", ro)
		}
	}
}

// validatePAT validates a bearer token as a Personal Access Token.
// Returns the user ID if valid, or an error if not found/expired.
func validatePAT(ctx context.Context, db *gorm.DB, token string) (string, error) {
	hash := sha256.Sum256([]byte(token))
	hashedToken := hex.EncodeToString(hash[:])

	var pat struct {
		ID          string     `gorm:"column:id"`
		TokenableID string     `gorm:"column:tokenable_id"`
		ExpiresAt   *time.Time `gorm:"column:expires_at"`
	}

	err := db.WithContext(ctx).
		Table("personal_access_tokens").
		Select("id, tokenable_id, expires_at").
		Where("token = ?", hashedToken).
		First(&pat).Error
	if err != nil {
		return "", errors.New("token not found")
	}

	if pat.ExpiresAt != nil && pat.ExpiresAt.Before(time.Now()) {
		return "", errors.New("token expired")
	}

	// Update last_used_at
	now := time.Now()
	_ = db.WithContext(ctx).
		Table("personal_access_tokens").
		Where("id = ?", pat.ID).
		Update("last_used_at", &now).Error

	return pat.TokenableID, nil
}

func tryAuthenticate(c *fiber.Ctx, jwtSecret string, db *gorm.DB) bool {
	token, err := bearerToken(c)
	if err != nil {
		return false
	}

	claims, err := security.ParseJWTToken(token, jwtSecret)
	if err == nil {
		if err := security.ValidateJWTTokenType(claims, "access"); err == nil {
			setAuthContext(c, claims)
			return true
		}
	}

	userID, patErr := validatePAT(c.Context(), db, token)
	if patErr == nil && userID != "" {
		fiberctx.SetUserContext(c, userID, nil)
		return true
	}

	return false
}

// Auth middleware requires a valid JWT token or PAT in the Authorization header.
// It sets userID and email in request context (c.Locals).
//
// Authentication order: JWT -> PAT lookup.
func Auth(jwtSecret string, db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if tryAuthenticate(c, jwtSecret, db) {
			// Enforce the read-only impersonation ("spectate") contract at the
			// authentication chokepoint. This guarantees every authenticated
			// route — whether wired via the AuthenticatedChain helpers or with a
			// bare authMiddleware inline — rejects mutating requests made under a
			// read-only impersonation token. No-op for normal tokens.
			if isImpersonationWriteBlocked(c) {
				return fiberctx.RespondForbidden(c, readOnlyImpersonationMessage)
			}

			return c.Next()
		}

		return fiberctx.RespondUnauthorized(c, "Unauthorized")
	}
}

// OptionalAuth is like Auth but doesn't require authentication.
// If a valid token is provided, it sets the user context.
// If no token or invalid token, it continues without user context.
func OptionalAuth(jwtSecret string, db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		_ = tryAuthenticate(c, jwtSecret, db)
		return c.Next()
	}
}

// EmailVerifiedService defines the interface for checking email verification status
type EmailVerifiedService interface {
	GetUser(ctx context.Context, userID string) (UserInfo, error)
}

// UserInfo represents minimal user information needed by middlewares
type UserInfo interface {
	HasVerifiedEmail() bool
}

// TwoFactorService defines the interface for 2FA operations needed by middlewares
type TwoFactorService interface {
	HasTwoFactorEnabled(ctx context.Context, userID string) (bool, error)
}

// EmailVerified checks if the user's email is verified.
// Returns 403 with requires_verification flag if not verified.
func EmailVerified(service EmailVerifiedService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals("userID").(string)
		if !ok || userID == "" {
			return fiberctx.RespondUnauthorized(c, "User not authenticated")
		}

		user, err := service.GetUser(c.Context(), userID)
		if err != nil {
			return fiberctx.Error(c, fiber.StatusInternalServerError, "Failed to get user")
		}

		if user == nil {
			return fiberctx.RespondNotFound(c, "User not found")
		}

		if !user.HasVerifiedEmail() {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success":               false,
				"message":               "Email verification required",
				"email_verified":        false,
				"requires_verification": true,
			})
		}

		return c.Next()
	}
}

// TwoFactor checks if 2FA is required and verified for the current session.
// Returns 423 Locked if 2FA is enabled but the session has not been 2FA-verified.
func TwoFactor(service TwoFactorService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals("userID").(string)
		if !ok || userID == "" {
			return c.Next()
		}

		has2FA, err := service.HasTwoFactorEnabled(c.Context(), userID)
		if err != nil {
			return fiberctx.Error(c, fiber.StatusInternalServerError, "Failed to check 2FA status")
		}

		if !has2FA {
			return c.Next()
		}

		return c.Status(fiber.StatusLocked).JSON(fiber.Map{
			"success":             false,
			"message":             "Two-factor authentication required",
			"two_factor_required": true,
		})
	}
}
