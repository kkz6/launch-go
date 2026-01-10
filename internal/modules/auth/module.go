package auth

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/middleware"
)

type Module struct {
	handler *Handler
	config  *config.Config
}

func NewModule(db *gorm.DB, cfg *config.Config, logger *zerolog.Logger) *Module {
	repo := NewRepository(db)
	service := NewService(repo, cfg, logger)
	handler := NewHandler(service)

	return &Module{
		handler: handler,
		config:  cfg,
	}
}

func (m *Module) RegisterRoutes(router fiber.Router) {
	auth := router.Group("/auth")

	// Public routes
	auth.Post("/register", m.handler.Register)
	auth.Post("/login", m.handler.Login)

	// Protected routes
	protected := auth.Group("", middleware.Auth(m.config.JWT.Secret))
	protected.Get("/user", m.handler.User)
	protected.Put("/profile", m.handler.UpdateProfile)
	protected.Put("/password", m.handler.ChangePassword)
	protected.Post("/switch-team/:teamId", m.handler.SwitchTeam)
}
