package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/auth/contracts"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	pkgdto "github.com/kkz6/launch-go/internal/pkg/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/util"
)

// PATHandler handles personal access token CRUD operations
type PATHandler struct {
	patRepo   contracts.PersonalAccessTokenRepository
	twoFactor *services.TwoFactorService
	userRepo  contracts.UserRepository
}

// NewPATHandler creates a new PAT handler
func NewPATHandler(patRepo contracts.PersonalAccessTokenRepository, twoFactor *services.TwoFactorService, userRepo contracts.UserRepository) *PATHandler {
	return &PATHandler{patRepo: patRepo, twoFactor: twoFactor, userRepo: userRepo}
}

type patResponse struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Abilities  []string   `json:"abilities"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedAt  *time.Time `json:"created_at,omitempty"`
}

type patCreateResponse struct {
	patResponse
	PlainTextToken string `json:"plain_text_token"`
}

type createPATRequest struct {
	Name      string   `json:"name" validate:"required,min=1,max=255"`
	Scopes    []string `json:"scopes"`
	ExpiresAt *string  `json:"expires_at"`
	Code      string   `json:"code"`
}

func toPATResponse(rec *models.PersonalAccessToken) patResponse {
	var abilities []string
	if rec.Abilities != nil {
		_ = json.Unmarshal([]byte(*rec.Abilities), &abilities)
	}
	if abilities == nil {
		abilities = []string{}
	}

	return patResponse{
		ID:         rec.ID,
		Name:       rec.Name,
		Abilities:  abilities,
		LastUsedAt: rec.LastUsedAt,
		ExpiresAt:  rec.ExpiresAt,
		CreatedAt:  rec.CreatedAt,
	}
}

// List returns all PATs for the authenticated user
func (h *PATHandler) List(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	tokens, err := h.patRepo.GetByUser(c.Context(), userID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	results := pkgdto.TransformSlice(tokens, toPATResponse)

	return fiberctx.OK(c, "Tokens retrieved", results)
}

// Create creates a new PAT for the authenticated user
func (h *PATHandler) Create(c *fiber.Ctx, req *createPATRequest) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	// If user has 2FA enabled, require a valid TOTP/recovery code
	user, err := h.userRepo.FindByID(c.Context(), userID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	if user != nil && user.HasEnabledTwoFactorAuthentication() {
		if req.Code == "" {
			return fiberctx.RespondValidationError(c, map[string][]string{
				"code": {"Two-factor authentication code is required"},
			})
		}

		valid, err := h.twoFactor.VerifyTwoFactor(c.Context(), userID, req.Code)
		if err != nil {
			return fiberctx.HandleError(c, err)
		}

		if !valid {
			return fiberctx.RespondValidationError(c, map[string][]string{
				"code": {"Invalid two-factor authentication code"},
			})
		}
	}

	scopes := req.Scopes
	if len(scopes) == 0 {
		scopes = []string{"*"}
	}
	abilitiesJSON, _ := json.Marshal(scopes)
	abilitiesStr := string(abilitiesJSON)

	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			return fiberctx.RespondBadRequest(c, "Invalid expires_at format, use RFC3339")
		}
		expiresAt = &t
	}

	plainToken, hashedToken := generatePATToken()

	now := time.Now()
	record := &models.PersonalAccessToken{
		TokenableType: "User",
		TokenableID:   userID,
		Name:          req.Name,
		Token:         hashedToken,
		Abilities:     &abilitiesStr,
		ExpiresAt:     expiresAt,
	}
	record.ID = util.NewULID()
	record.CreatedAt = &now
	record.UpdatedAt = &now

	if err := h.patRepo.Create(c.Context(), record); err != nil {
		return fiberctx.HandleError(c, err)
	}

	resp := patCreateResponse{
		patResponse:    toPATResponse(record),
		PlainTextToken: plainToken,
	}

	return fiberctx.Created(c, "Token created", resp)
}

// Delete revokes a PAT
func (h *PATHandler) Delete(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	tokenID := c.Params("id")
	if tokenID == "" {
		return fiberctx.RespondBadRequest(c, "Token ID is required")
	}

	rowsAffected, err := h.patRepo.DeleteByUser(c.Context(), tokenID, userID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}
	if rowsAffected == 0 {
		return fiberctx.RespondNotFound(c, "Token not found")
	}

	return fiberctx.NoContent(c)
}

func generatePATToken() (plainText, hashed string) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	plainText = hex.EncodeToString(b)
	hash := sha256.Sum256([]byte(plainText))
	hashed = hex.EncodeToString(hash[:])
	return plainText, hashed
}
