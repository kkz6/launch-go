package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/security"
)

const (
	deviceGrantTTL     = 10 * time.Minute
	devicePollInterval = 5 * time.Second
	deviceMaxAttempts  = 5
)

type deviceTwoFactor interface {
	VerifyTwoFactor(context.Context, string, string) (bool, error)
}

type DeviceHandler struct {
	db          *gorm.DB
	twoFactor   deviceTwoFactor
	jwtSecret   string
	frontendURL string
	now         func() time.Time
}

func NewDeviceHandler(db *gorm.DB, twoFactor deviceTwoFactor, jwtSecret, frontendURL string) *DeviceHandler {
	return &DeviceHandler{db: db, twoFactor: twoFactor, jwtSecret: jwtSecret, frontendURL: frontendURL, now: time.Now}
}

type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type deviceTokenRequest struct {
	DeviceCode string `json:"device_code" validate:"required,len=64,hexadecimal"`
}

type deviceTokenResponse struct {
	Status      string            `json:"status"`
	Interval    int               `json:"interval,omitempty"`
	AccessToken string            `json:"access_token,omitempty"`
	User        *dto.UserResponse `json:"user,omitempty"`
}

type deviceAuthorizeRequest struct {
	UserCode string `json:"user_code" validate:"required,len=8,numeric"`
	Approve  *bool  `json:"approve" validate:"required"`
	Code     string `json:"code" validate:"max=255"`
}

func deviceHash(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}

// limit uses a database upsert instead of a cache get/set pair so parallel
// requests cannot bypass the attempt limit on another API instance.
func (h *DeviceHandler) limit(c *fiber.Ctx, key string, maximum int) error {
	now := h.now().UTC()
	window := now.Truncate(time.Minute)
	limit := models.DeviceAuthorizationLimit{
		ID:        deviceHash(key + ":" + window.Format(time.RFC3339)),
		Attempts:  1,
		ExpiresAt: window.Add(2 * time.Minute),
	}
	err := h.db.WithContext(c.UserContext()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.Assignments(map[string]any{"attempts": gorm.Expr("device_authorization_limits.attempts + 1")}),
		}).Create(&limit).Error; err != nil {
			return err
		}
		// Read while holding the upsert's row lock. MySQL does not support
		// RETURNING, and reading after commit could count a later request.
		return tx.First(&limit, "id = ?", limit.ID).Error
	})
	if err != nil {
		return err
	}
	if limit.Attempts > maximum {
		c.Set("Retry-After", fmt.Sprint(int(window.Add(time.Minute).Sub(now).Seconds())+1))
		return fiber.NewError(fiber.StatusTooManyRequests, "Too many device sign-in attempts. Please try again later.")
	}
	return nil
}

func (h *DeviceHandler) Code(c *fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	if err := h.limit(c, "device-code:"+c.IP(), 5); err != nil {
		return err
	}
	now := h.now().UTC()
	// The indexed cleanup also frees expired eight-digit codes for reuse.
	if err := h.db.WithContext(c.UserContext()).Where("expires_at <= ?", now).Delete(&models.DeviceAuthorization{}).Error; err != nil {
		return err
	}
	if err := h.db.WithContext(c.UserContext()).Where("expires_at <= ?", now).Delete(&models.DeviceAuthorizationLimit{}).Error; err != nil {
		return err
	}
	for attempt := 0; attempt < 5; attempt++ {
		secret, secretHash, err := generatePATToken()
		if err != nil {
			return err
		}
		number, err := rand.Int(rand.Reader, big.NewInt(100000000))
		if err != nil {
			return err
		}
		userCode := fmt.Sprintf("%08d", number.Int64())
		grant := models.DeviceAuthorization{
			DeviceCodeHash: secretHash, UserCodeHash: deviceHash(userCode),
			Status: "pending", ExpiresAt: now.Add(deviceGrantTTL),
		}
		result := h.db.WithContext(c.UserContext()).Clauses(clause.OnConflict{DoNothing: true}).Create(&grant)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			continue
		}
		return fiberctx.OK(c, "Device sign-in started", deviceCodeResponse{
			DeviceCode: secret, UserCode: userCode,
			VerificationURI: strings.TrimRight(h.frontendURL, "/") + "/cli/auth",
			ExpiresIn:       int(deviceGrantTTL.Seconds()), Interval: int(devicePollInterval.Seconds()),
		})
	}
	return fiberctx.Internal("Could not create a device sign-in code")
}

func (h *DeviceHandler) Token(c *fiber.Ctx, req *deviceTokenRequest) error {
	c.Set("Cache-Control", "no-store")
	if err := h.limit(c, "device-token:"+c.IP(), 120); err != nil {
		return err
	}
	now := h.now().UTC()
	response := deviceTokenResponse{Status: "expired"}
	err := h.db.WithContext(c.UserContext()).Transaction(func(tx *gorm.DB) error {
		// Write before reading: competing polls serialize on the grant row.
		// The time predicate also enforces the poll interval across replicas.
		claim := tx.Model(&models.DeviceAuthorization{}).
			Where("device_code_hash = ? AND expires_at > ? AND status IN ?", deviceHash(req.DeviceCode), now, []string{"pending", "approved"}).
			Where("last_polled_at IS NULL OR last_polled_at <= ?", now.Add(-devicePollInterval)).
			Update("last_polled_at", now)
		if claim.Error != nil {
			return claim.Error
		}
		var grant models.DeviceAuthorization
		if err := tx.Where("device_code_hash = ?", deviceHash(req.DeviceCode)).First(&grant).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		now = h.now().UTC()
		if !now.Before(grant.ExpiresAt) || grant.Status == "consumed" {
			return nil
		}
		if grant.Status == "denied" {
			response.Status = "denied"
			return nil
		}
		if claim.RowsAffected != 1 {
			response.Status, response.Interval = "slow_down", int(devicePollInterval.Seconds())
			return nil
		}
		if grant.Status != "approved" {
			response.Status = "pending"
			return nil
		}
		consume := tx.Model(&models.DeviceAuthorization{}).
			Where("id = ? AND status = ? AND expires_at > ?", grant.ID, "approved", now).
			Update("status", "consumed")
		if consume.Error != nil || consume.RowsAffected != 1 {
			return consume.Error
		}
		var user models.User
		if grant.UserID == nil {
			response.Status = "denied"
			return nil
		}
		if err := tx.First(&user, "id = ?", *grant.UserID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				response.Status = "denied"
				return nil
			}
			return err
		}
		if user.IsSuspended() {
			response.Status = "denied"
			return nil
		}
		plain, hash, err := generatePATToken()
		if err != nil {
			return err
		}
		abilities := `["*"]`
		pat := models.PersonalAccessToken{
			TokenableType: "User", TokenableID: user.ID, Name: "lctl (device sign-in)",
			Token: hash, Abilities: &abilities,
		}
		if err := tx.Create(&pat).Error; err != nil {
			return err
		}
		userResponse := dto.ToUserResponse(&user)
		response = deviceTokenResponse{Status: "approved", AccessToken: plain, User: &userResponse}
		return nil
	})
	if err != nil {
		return err
	}
	return fiberctx.OK(c, "Device sign-in status", response)
}

// browserUser requires a real, live browser login. PATs and their sessionless
// exchange JWTs cannot mint a fresh unrestricted credential through this flow.
func (h *DeviceHandler) browserUser(c *fiber.Ctx) (*models.User, error) {
	parts := strings.Fields(c.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return nil, fiberctx.Unauthorized()
	}
	claims, err := security.ParseJWTToken(parts[1], h.jwtSecret)
	if err != nil || security.ValidateJWTTokenType(claims, "access") != nil {
		return nil, fiberctx.Forbidden("Sign in through the browser to authorize the CLI")
	}
	if _, impersonating := claims["impersonation_sid"]; impersonating {
		return nil, fiberctx.Forbidden("Device sign-in is unavailable during impersonation")
	}
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return nil, err
	}
	subject, _ := security.ExtractJWTClaim(claims, "sub")
	sessionID, _ := security.ExtractJWTClaim(claims, "session_id")
	if userID != subject || sessionID == "" {
		return nil, fiberctx.Forbidden("Sign in through the browser to authorize the CLI")
	}
	var session models.Session
	if err := h.db.WithContext(c.UserContext()).First(&session, "id = ? AND user_id = ?", sessionID, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberctx.Unauthorized("Browser session is no longer active")
		}
		return nil, err
	}
	var user models.User
	if err := h.db.WithContext(c.UserContext()).First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiberctx.Unauthorized()
		}
		return nil, err
	}
	if user.IsSuspended() {
		return nil, fiberctx.Forbidden("Account suspended")
	}
	return &user, nil
}

func (h *DeviceHandler) Authorize(c *fiber.Ctx, req *deviceAuthorizeRequest) error {
	c.Set("Cache-Control", "no-store")
	if err := h.limit(c, "device-authorize-ip:"+c.IP(), 20); err != nil {
		return err
	}
	user, err := h.browserUser(c)
	if err != nil {
		return err
	}
	if err := h.limit(c, "device-authorize-user:"+user.ID, 10); err != nil {
		return err
	}
	now := h.now().UTC()
	var grant models.DeviceAuthorization
	if err := h.db.WithContext(c.UserContext()).Where("user_code_hash = ? AND expires_at > ?", deviceHash(req.UserCode), now).First(&grant).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return invalidDeviceUserCode(c)
		}
		return err
	}
	if grant.Status != "pending" {
		return fiberctx.Conflict("This device sign-in request has already been decided")
	}
	attempt := h.db.WithContext(c.UserContext()).Model(&models.DeviceAuthorization{}).
		Where("id = ? AND status = ? AND expires_at > ? AND authorization_attempts < ?", grant.ID, "pending", now, deviceMaxAttempts).
		Update("authorization_attempts", gorm.Expr("authorization_attempts + 1"))
	if attempt.Error != nil {
		return attempt.Error
	}
	if attempt.RowsAffected != 1 {
		var current models.DeviceAuthorization
		if err := h.db.WithContext(c.UserContext()).First(&current, "id = ?", grant.ID).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if h.now().UTC().Before(current.ExpiresAt) && current.Status != "pending" {
			return fiberctx.Conflict("This device sign-in request has already been decided")
		}
		return invalidDeviceUserCode(c)
	}
	if *req.Approve && user.HasEnabledTwoFactorAuthentication() {
		if req.Code == "" {
			return fiberctx.RespondValidationError(c, map[string][]string{"code": {"Two-factor authentication code is required"}})
		}
		if h.twoFactor == nil {
			return fiberctx.Internal()
		}
		valid, err := h.twoFactor.VerifyTwoFactor(c.UserContext(), user.ID, req.Code)
		if err != nil {
			return err
		}
		if !valid {
			return fiberctx.RespondValidationError(c, map[string][]string{"code": {"Invalid two-factor authentication code"}})
		}
	}
	status := "denied"
	if *req.Approve {
		status = "approved"
	}
	decision := h.db.WithContext(c.UserContext()).Model(&models.DeviceAuthorization{}).
		Where("id = ? AND status = ? AND expires_at > ?", grant.ID, "pending", h.now().UTC()).
		Updates(map[string]any{"status": status, "user_id": user.ID})
	if decision.Error != nil {
		return decision.Error
	}
	if decision.RowsAffected != 1 {
		return fiberctx.Conflict("This device sign-in request is no longer available")
	}
	return fiberctx.OK(c, "Device sign-in request updated", map[string]string{"status": status})
}

func invalidDeviceUserCode(c *fiber.Ctx) error {
	return fiberctx.RespondValidationError(c, map[string][]string{"user_code": {"This code is invalid, expired, or has reached its attempt limit"}})
}
