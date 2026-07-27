package handlers

import (
	"github.com/gofiber/fiber/v2"

	authdto "github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	"github.com/kkz6/launch-go/internal/pkg/dto"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// PasskeyHandler handles passkey-related HTTP requests
type PasskeyHandler struct {
	BaseHandler
}

// NewPasskeyHandler creates a new PasskeyHandler instance
func NewPasskeyHandler(service *services.Service) *PasskeyHandler {
	return &PasskeyHandler{BaseHandler: NewBaseHandler(service)}
}

// Index returns all passkeys for the authenticated user
func (h *PasskeyHandler) Index(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	passkeys, err := h.Service().Passkey.GetUserPasskeys(c.Context(), userID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	responses := make([]authdto.PasskeyResponse, len(passkeys))
	for i, p := range passkeys {
		createdAtStr := dto.FormatDisplayTimeOrEmpty(p.CreatedAt)

		resp := authdto.PasskeyResponse{
			ID:        p.ID,
			Name:      p.DisplayName(),
			CreatedAt: createdAtStr,
		}

		if p.LastUsedAt != nil {
			resp.LastUsedAt = dto.FormatDisplayDateTime(p.LastUsedAt)
		}

		resp.Transports = p.ParseTransports()

		responses[i] = resp
	}

	return c.JSON(fiber.Map{
		"data": responses,
	})
}

// BeginRegistration generates WebAuthn registration options
func (h *PasskeyHandler) BeginRegistration(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	options, err := h.Service().Passkey.BeginRegistration(c.Context(), userID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return c.JSON(options)
}

// FinishRegistration completes WebAuthn registration
func (h *PasskeyHandler) FinishRegistration(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	var nameReq authdto.PasskeyRegisterRequest
	if err := c.BodyParser(&nameReq); err != nil {
		return fiberctx.BadRequest("invalid passkey registration payload")
	}

	var name *string
	if nameReq.Name != "" {
		name = &nameReq.Name
	}

	passkey, err := h.Service().Passkey.FinishRegistration(c.Context(), userID, c.Body(), name)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Passkey registered successfully", authdto.PasskeyResponse{
		ID:        passkey.ID,
		Name:      passkey.DisplayName(),
		CreatedAt: dto.FormatDisplayTimeOrEmpty(passkey.CreatedAt),
	})
}

// BeginLogin generates WebAuthn authentication options
func (h *PasskeyHandler) BeginLogin(c *fiber.Ctx) error {
	var req authdto.PasskeyLoginRequest
	if err := c.BodyParser(&req); err != nil {
		return fiberctx.BadRequest("invalid passkey login payload")
	}
	req.Normalize()

	options, err := h.Service().Passkey.BeginLogin(c.Context(), req.Email)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return c.JSON(options)
}

// FinishLogin completes WebAuthn authentication
func (h *PasskeyHandler) FinishLogin(c *fiber.Ctx) error {
	user, err := h.Service().Passkey.FinishLogin(c.Context(), c.Body())
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	authResponse, err := h.Service().Auth.LoginWithPasskey(c.Context(), user, c.IP(), string(c.Request().Header.UserAgent()))
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return c.JSON(authResponse)
}

// Update updates a passkey's name
func (h *PasskeyHandler) Update(c *fiber.Ctx, req *authdto.PasskeyUpdateRequest) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	passkeyID := c.Params("id")

	if err := h.Service().Passkey.UpdatePasskeyName(c.Context(), passkeyID, userID, req.Name); err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Passkey updated successfully", nil)
}

// Delete removes a passkey
func (h *PasskeyHandler) Delete(c *fiber.Ctx) error {
	userID, err := fiberctx.MustGetUserID(c)
	if err != nil {
		return err
	}

	passkeyID := c.Params("id")

	err = h.Service().Passkey.DeletePasskey(c.Context(), passkeyID, userID)
	if err != nil {
		return fiberctx.HandleError(c, err)
	}

	return fiberctx.OK(c, "Passkey deleted successfully", nil)
}
