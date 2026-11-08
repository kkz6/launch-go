package handlers

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/modules/staff/services"
	stafftypes "github.com/kkz6/launch-go/internal/modules/staff/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

func (h *AdminHandler) UpdateStaffRole(c *fiber.Ctx) error {
	var request struct {
		StaffRole json.RawMessage `json:"staff_role"`
	}
	if err := c.BodyParser(&request); err != nil {
		return fiberutil.RespondBadRequest(c, "Invalid request body")
	}
	var role *stafftypes.StaffRole
	if len(request.StaffRole) == 0 {
		return invalidStaffRole(c)
	}
	if !bytes.Equal(bytes.TrimSpace(request.StaffRole), []byte("null")) {
		var value stafftypes.StaffRole
		if err := json.Unmarshal(request.StaffRole, &value); err != nil || !value.IsValid() {
			return invalidStaffRole(c)
		}
		role = &value
	}
	actorID, err := fiberutil.MustGetUserID(c)
	if err != nil {
		return err
	}
	result, err := h.service.UpdateStaffRole(c.UserContext(), actorID, c.Params("id"), role)
	switch {
	case errors.Is(err, services.ErrInvalidStaffRole):
		return invalidStaffRole(c)
	case errors.Is(err, services.ErrStaffRoleForbidden):
		return fiberutil.RespondForbidden(c, err.Error())
	case errors.Is(err, services.ErrUserNotFound):
		return fiberutil.RespondNotFound(c, "User not found")
	case errors.Is(err, services.ErrCannotChangeOwnStaffRole), errors.Is(err, services.ErrLastSuperAdmin):
		return fiberutil.RespondConflict(c, err.Error())
	case err != nil:
		return fiberutil.HandleError(c, err)
	default:
		return fiberutil.OK(c, "Staff role updated", result)
	}
}

func invalidStaffRole(c *fiber.Ctx) error {
	return fiberutil.RespondValidationError(c, map[string][]string{"staff_role": {services.ErrInvalidStaffRole.Error()}})
}
