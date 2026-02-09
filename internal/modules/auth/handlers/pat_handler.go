package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/util"
)

// PATHandler handles personal access token CRUD operations
type PATHandler struct {
	db *gorm.DB
}

// NewPATHandler creates a new PAT handler
func NewPATHandler(db *gorm.DB) *PATHandler {
	return &PATHandler{db: db}
}

type patRecord struct {
	ID            string     `gorm:"type:char(26);primaryKey" json:"id"`
	TokenableType string     `gorm:"column:tokenable_type" json:"-"`
	TokenableID   string     `gorm:"column:tokenable_id" json:"-"`
	Name          string     `gorm:"type:varchar(255)" json:"name"`
	Token         string     `gorm:"type:varchar(64)" json:"-"`
	Abilities     *string    `gorm:"type:text" json:"abilities,omitempty"`
	LastUsedAt    *time.Time `gorm:"column:last_used_at" json:"last_used_at,omitempty"`
	ExpiresAt     *time.Time `gorm:"column:expires_at" json:"expires_at,omitempty"`
	CreatedAt     *time.Time `json:"created_at,omitempty"`
	UpdatedAt     *time.Time `json:"updated_at,omitempty"`
}

func (patRecord) TableName() string {
	return "personal_access_tokens"
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
}

func toPATResponse(rec *patRecord) patResponse {
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

	var tokens []patRecord
	if err := h.db.WithContext(c.Context()).
		Where("tokenable_type = ? AND tokenable_id = ?", "User", userID).
		Order("created_at DESC").
		Find(&tokens).Error; err != nil {
		return fiberctx.RespondInternalError(c, "Failed to fetch tokens")
	}

	results := make([]patResponse, len(tokens))
	for i, t := range tokens {
		results[i] = toPATResponse(&t)
	}

	return fiberctx.OK(c, "Tokens retrieved", results)
}

// Create creates a new PAT for the authenticated user
func (h *PATHandler) Create(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	req, err := fiberctx.MustParseAndValidate[createPATRequest](c)
	if err != nil {
		return err
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
	record := &patRecord{
		ID:            util.NewULID(),
		TokenableType: "User",
		TokenableID:   userID,
		Name:          req.Name,
		Token:         hashedToken,
		Abilities:     &abilitiesStr,
		ExpiresAt:     expiresAt,
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}

	if err := h.db.WithContext(c.Context()).Create(record).Error; err != nil {
		return fiberctx.RespondInternalError(c, "Failed to create token")
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

	result := h.db.WithContext(c.Context()).
		Where("id = ? AND tokenable_type = ? AND tokenable_id = ?", tokenID, "User", userID).
		Delete(&patRecord{})

	if result.Error != nil {
		return fiberctx.RespondInternalError(c, "Failed to delete token")
	}
	if result.RowsAffected == 0 {
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
