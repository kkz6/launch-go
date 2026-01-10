package auth

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/pkg/response"
	"github.com/kkz6/launch-go/internal/pkg/validator"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	result, err := h.service.Register(c.Context(), &req)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.Created(c, "Registration successful", result)
}

func (h *Handler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	result, err := h.service.Login(c.Context(), &req)
	if err != nil {
		return response.Unauthorized(c, "Invalid credentials")
	}

	return response.OK(c, "Login successful", result)
}

func (h *Handler) User(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	user, err := h.service.GetUser(c.Context(), userID)
	if err != nil {
		return response.NotFound(c, "User not found")
	}

	return response.OK(c, "User retrieved", ToUserResponse(user))
}

func (h *Handler) UpdateProfile(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	var req UpdateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	user, err := h.service.UpdateProfile(c.Context(), userID, &req)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Profile updated", ToUserResponse(user))
}

func (h *Handler) ChangePassword(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	var req ChangePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if errors := validator.Validate(&req); errors != nil {
		return response.ValidationError(c, errors)
	}

	if err := h.service.ChangePassword(c.Context(), userID, &req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Password changed successfully", nil)
}

func (h *Handler) SwitchTeam(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	teamID := c.Params("teamId")

	user, err := h.service.SwitchTeam(c.Context(), userID, teamID)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error())
	}

	return response.OK(c, "Team switched", ToUserResponse(user))
}
