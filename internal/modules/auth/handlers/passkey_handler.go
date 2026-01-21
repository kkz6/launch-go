package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/auth/contracts"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
	"github.com/kkz6/launch-go/internal/pkg/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/response"
)

// PasskeyHandler handles passkey-related HTTP requests
type PasskeyHandler struct {
	repos contracts.RepositoryRegistry
}

// NewPasskeyHandler creates a new PasskeyHandler instance
func NewPasskeyHandler(repos *repositories.Registry) *PasskeyHandler {
	return &PasskeyHandler{
		repos: repos,
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
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	passkeys, err := h.repos.Passkey().FindByUserID(c.Context(), userID)
	if err != nil {
		return response.HandleError(c, err)
	}

	passkeyResponses := make([]PasskeyResponse, len(passkeys))
	for i, p := range passkeys {
		name := "Passkey"
		createdAtStr := dto.FormatDisplayTimeOrEmpty(p.CreatedAt)

		if p.Name != nil && *p.Name != "" {
			name = *p.Name
		} else if createdAtStr != "" {
			name = "Passkey created on " + createdAtStr
		}

		resp := PasskeyResponse{
			ID:        p.ID,
			Name:      name,
			CreatedAt: createdAtStr,
		}

		if p.LastUsedAt != nil {
			resp.LastUsedAt = dto.FormatDisplayDateTime(p.LastUsedAt)
		}

		passkeyResponses[i] = resp
	}

	return c.JSON(fiber.Map{
		"passkeys": passkeyResponses,
	})
}

// Delete removes a passkey
func (h *PasskeyHandler) Delete(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}
	passkeyID := c.Params("id")

	err = h.repos.Passkey().DeleteByUserID(c.Context(), passkeyID, userID)
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
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}
	passkeyID := c.Params("id")

	req, err := fiberctx.MustParseAndValidate[UpdateRequest](c)
	if err != nil {
		return err
	}

	passkey, err := h.repos.Passkey().FindByID(c.Context(), passkeyID)
	if err != nil {
		return response.HandleError(c, err)
	}

	if passkey.UserID != userID {
		return response.NotFound(c, response.MsgResourceNotFound)
	}

	passkey.Name = &req.Name
	if err := h.repos.Passkey().Update(c.Context(), passkey); err != nil {
		return response.HandleError(c, err)
	}

	return response.OK(c, "Passkey updated successfully", nil)
}
