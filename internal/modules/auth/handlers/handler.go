package handlers

import (
	"github.com/kkz6/launch-go/internal/modules/auth/services"
)

// Handler aggregates all auth-related handlers
type Handler struct {
	Auth       *AuthHandler
	User       *UserHandler
	Email      *EmailHandler
	Password   *PasswordHandler
	TwoFactor  *TwoFactorHandler
	Team       *TeamHandler
	TeamMember *TeamMemberHandler
}

// NewHandler creates a new Handler instance
func NewHandler(service *services.Service) *Handler {
	return &Handler{
		Auth:       NewAuthHandler(service),
		User:       NewUserHandler(service),
		Email:      NewEmailHandler(service),
		Password:   NewPasswordHandler(service),
		TwoFactor:  NewTwoFactorHandler(service),
		Team:       NewTeamHandler(service),
		TeamMember: NewTeamMemberHandler(service),
	}
}
