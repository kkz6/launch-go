package auth

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/auth/handlers"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	"github.com/kkz6/launch-go/internal/pkg/app"
)

// Ensure Module implements required interfaces
var (
	_ app.Module              = (*Module)(nil)
	_ app.PublicRouteRegistrar = (*Module)(nil)
)

// Module represents the auth module with all its dependencies
type Module struct {
	handler    *handlers.Handler
	service    *services.Service
	repository *repositories.Repository
	config     *config.Config
	logger     *zerolog.Logger
}

// NewModule creates a new auth Module instance
func NewModule(db *gorm.DB, cfg *config.Config, logger *zerolog.Logger) *Module {
	repo := repositories.NewRepository(db)
	service := services.NewService(repo, cfg, logger)
	handler := handlers.NewHandler(service)

	return &Module{
		handler:    handler,
		service:    service,
		repository: repo,
		config:     cfg,
		logger:     logger,
	}
}

// NewModuleFromContext creates a new auth module from app context
func NewModuleFromContext(ctx *app.Context) *Module {
	return NewModule(ctx.DB, ctx.Config, ctx.Logger)
}

// Name returns the module name (implements app.Module)
func (m *Module) Name() string {
	return "auth"
}

// RegisterRoutes registers all auth-related routes (legacy method)
func (m *Module) RegisterRoutes(router fiber.Router) {
	m.RegisterPublicRoutes(router)
}

// RegisterPublicRoutes implements routing.PublicRouteModule interface.
// Auth module is special because it handles its own auth middleware internally.
func (m *Module) RegisterPublicRoutes(router fiber.Router) {
	auth := router.Group("/auth")
	authMiddleware := middleware.Auth(m.config.JWT.Secret)
	adapter := NewMiddlewareAdapter(m.service)

	// Public routes (no authentication required)
	m.registerPublicRoutes(auth)

	// Protected routes (authentication required)
	protected := auth.Group("", authMiddleware)
	m.registerProtectedRoutes(protected)

	// Team routes (require authentication)
	teams := router.Group("/teams", authMiddleware)
	m.registerTeamRoutes(teams, adapter)
}

// registerPublicRoutes registers routes that don't require authentication
func (m *Module) registerPublicRoutes(router fiber.Router) {
	// Registration and Login
	router.Post("/register", m.handler.Auth.Register)
	router.Post("/login", m.handler.Auth.Login)
	router.Post("/refresh", m.handler.Auth.RefreshToken)

	// Password Reset
	router.Post("/forgot-password", m.handler.Password.ForgotPassword)
	router.Post("/reset-password", m.handler.Password.ResetPassword)

	// Email Verification (public for verification links)
	router.Get("/verify-email/:id/:hash", m.handler.Email.VerifyEmail)

	// User Status Check (for login flow)
	router.Post("/check-user-status", m.handler.User.CheckUserStatus)
}

// registerProtectedRoutes registers routes that require authentication
func (m *Module) registerProtectedRoutes(router fiber.Router) {
	// User Management
	router.Get("/user", m.handler.User.User)
	router.Put("/profile", m.handler.User.UpdateProfile)
	router.Put("/password", m.handler.User.ChangePassword)
	router.Delete("/account", m.handler.User.DeleteAccount)
	router.Post("/logout", m.handler.Auth.Logout)

	// Email Verification (resend)
	router.Post("/email/verification-notification", m.handler.Email.ResendVerificationEmail)

	// Two-Factor Authentication
	twoFactor := router.Group("/two-factor")
	twoFactor.Post("/enable", m.handler.TwoFactor.EnableTwoFactor)
	twoFactor.Post("/confirm", m.handler.TwoFactor.ConfirmTwoFactor)
	twoFactor.Delete("/disable", m.handler.TwoFactor.DisableTwoFactor)
	twoFactor.Post("/challenge", m.handler.TwoFactor.TwoFactorChallenge)
	twoFactor.Get("/recovery-codes", m.handler.TwoFactor.GetRecoveryCodes)
	twoFactor.Post("/recovery-codes", m.handler.TwoFactor.RegenerateRecoveryCodes)

	// Current User's Teams
	router.Get("/teams", m.handler.Team.GetUserTeams)
	router.Post("/switch-team/:teamId", m.handler.Team.SwitchTeam)

	// Team Invitations (for accepting invitations)
	router.Post("/team-invitations/:invitationId/accept", m.handler.TeamMember.AcceptTeamInvitation)
}

// registerTeamRoutes registers team management routes
func (m *Module) registerTeamRoutes(router fiber.Router, adapter *MiddlewareAdapter) {
	// List all teams for current user
	router.Get("/", m.handler.Team.GetUserTeams)

	// Team CRUD
	router.Post("/", m.handler.Team.CreateTeam)
	router.Get("/:teamId", middleware.TeamMember(adapter), m.handler.Team.GetTeam)
	router.Put("/:teamId", middleware.TeamOwner(adapter), m.handler.Team.UpdateTeam)
	router.Delete("/:teamId", middleware.TeamOwner(adapter), m.handler.Team.DeleteTeam)

	// Team Members
	router.Get("/:teamId/members", middleware.TeamMember(adapter), m.handler.TeamMember.GetTeamMembers)
	router.Post("/:teamId/members", middleware.TeamAdmin(adapter), m.handler.TeamMember.InviteTeamMember)
	router.Put("/:teamId/members/:memberId", middleware.TeamOwner(adapter), m.handler.TeamMember.UpdateTeamMemberRole)
	router.Delete("/:teamId/members/:memberId", m.handler.TeamMember.RemoveTeamMember)

	// Team Invitations
	router.Get("/:teamId/invitations", middleware.TeamAdmin(adapter), m.handler.TeamMember.GetTeamInvitations)
	router.Delete("/:teamId/invitations/:invitationId", middleware.TeamAdmin(adapter), m.handler.TeamMember.CancelTeamInvitation)
}

// Service returns the auth service
func (m *Module) Service() *services.Service {
	return m.service
}

// Repository returns the auth repository
func (m *Module) Repository() *repositories.Repository {
	return m.repository
}

// Handler returns the auth handler
func (m *Module) Handler() *handlers.Handler {
	return m.handler
}

// AutoMigrate runs database migrations for auth models
func (m *Module) AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.Team{},
		&models.TeamMember{},
		&models.TeamInvitation{},
		&models.PersonalAccessToken{},
		&models.PasswordResetToken{},
	)
}

// GetModels returns all models for the auth module
func (m *Module) GetModels() []interface{} {
	return []interface{}{
		&models.User{},
		&models.Team{},
		&models.TeamMember{},
		&models.TeamInvitation{},
		&models.PersonalAccessToken{},
		&models.PasswordResetToken{},
	}
}
