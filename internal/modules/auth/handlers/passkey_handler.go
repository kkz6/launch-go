package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// PasskeyHandler handles passkey-related HTTP requests
type PasskeyHandler struct {
	passkeyRepo *repositories.PasskeyRepository
}

// NewPasskeyHandler creates a new PasskeyHandler instance
func NewPasskeyHandler(passkeyRepo *repositories.PasskeyRepository) *PasskeyHandler {
	return &PasskeyHandler{
		passkeyRepo: passkeyRepo,
	}
}

// PasskeyResponse represents a passkey in API responses
type PasskeyResponse struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	CreatedAt  string   `json:"created_at"`
	LastUsedAt *string  `json:"last_used_at,omitempty"`
	Transports []string `json:"transports,omitempty"`
}

// Index returns all passkeys for the authenticated user
func (h *PasskeyHandler) Index(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	passkeys, err := h.passkeyRepo.FindByUserID(c.Context(), userID)
	if err != nil {
		return response.HandleError(c, err)
	}

	passkeyResponses := make([]PasskeyResponse, len(passkeys))
	for i, p := range passkeys {
		name := "Passkey"
		if p.Name != nil && *p.Name != "" {
			name = *p.Name
		} else {
			name = "Passkey created on " + p.CreatedAt.Format("Jan 2, 2006")
		}

		resp := PasskeyResponse{
			ID:        p.ID,
			Name:      name,
			CreatedAt: p.CreatedAt.Format("Jan 2, 2006"),
		}

		if p.LastUsedAt != nil {
			lastUsed := p.LastUsedAt.Format("Jan 2, 2006 3:04 PM")
			resp.LastUsedAt = &lastUsed
		}

		passkeyResponses[i] = resp
	}

	return c.JSON(fiber.Map{
		"passkeys": passkeyResponses,
	})
}

// Delete removes a passkey
func (h *PasskeyHandler) Delete(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	passkeyID := c.Params("id")

	err := h.passkeyRepo.DeleteByUserID(c.Context(), passkeyID, userID)
	if err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Passkey deleted successfully", nil)
}

// UpdateRequest represents the request body for updating a passkey
type UpdateRequest struct {
	Name string `json:"name" validate:"required,max=255"`
}

// Update updates a passkey's name
func (h *PasskeyHandler) Update(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	passkeyID := c.Params("id")

	var req UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	passkey, err := h.passkeyRepo.FindByID(c.Context(), passkeyID)
	if err != nil {
		return response.HandleError(c, err)
	}

	if passkey.UserID != userID {
		return response.Error(c, fiber.StatusNotFound, "Passkey not found")
	}

	passkey.Name = &req.Name
	if err := h.passkeyRepo.Update(c.Context(), passkey); err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Passkey updated successfully", nil)
}
