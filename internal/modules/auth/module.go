package auth

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
)

// Module represents the auth module with all its dependencies
type Module struct {
	handler    *Handler
	service    *Service
	repository *Repository
	config     *config.Config
	logger     *zerolog.Logger
}

// NewModule creates a new auth Module instance
func NewModule(db *gorm.DB, cfg *config.Config, logger *zerolog.Logger) *Module {
	repo := NewRepository(db)
	service := NewService(repo, cfg, logger)
	handler := NewHandler(service)

	return &Module{
		handler:    handler,
		service:    service,
		repository: repo,
		config:     cfg,
		logger:     logger,
	}
}

// RegisterRoutes registers all auth-related routes
func (m *Module) RegisterRoutes(router fiber.Router) {
	auth := router.Group("/auth")

	// Public routes (no authentication required)
	m.registerPublicRoutes(auth)

	// Protected routes (authentication required)
	protected := auth.Group("", AuthMiddleware(m.config.JWT.Secret))
	m.registerProtectedRoutes(protected)

	// Team routes (require authentication)
	teams := router.Group("/teams", AuthMiddleware(m.config.JWT.Secret))
	m.registerTeamRoutes(teams)
}

// registerPublicRoutes registers routes that don't require authentication
func (m *Module) registerPublicRoutes(router fiber.Router) {
	// Registration and Login
	router.Post("/register", m.handler.Register)
	router.Post("/login", m.handler.Login)
	router.Post("/refresh", m.handler.RefreshToken)

	// Password Reset
	router.Post("/forgot-password", m.handler.ForgotPassword)
	router.Post("/reset-password", m.handler.ResetPassword)

	// Email Verification (public for verification links)
	router.Get("/verify-email/:id/:hash", m.handler.VerifyEmail)

	// User Status Check (for login flow)
	router.Post("/check-user-status", m.handler.CheckUserStatus)
}

// registerProtectedRoutes registers routes that require authentication
func (m *Module) registerProtectedRoutes(router fiber.Router) {
	// User Management
	router.Get("/user", m.handler.User)
	router.Put("/profile", m.handler.UpdateProfile)
	router.Put("/password", m.handler.ChangePassword)
	router.Delete("/account", m.handler.DeleteAccount)
	router.Post("/logout", m.handler.Logout)

	// Email Verification (resend)
	router.Post("/email/verification-notification", m.handler.ResendVerificationEmail)

	// Two-Factor Authentication
	twoFactor := router.Group("/two-factor")
	twoFactor.Post("/enable", m.handler.EnableTwoFactor)
	twoFactor.Post("/confirm", m.handler.ConfirmTwoFactor)
	twoFactor.Delete("/disable", m.handler.DisableTwoFactor)
	twoFactor.Post("/challenge", m.handler.TwoFactorChallenge)
	twoFactor.Get("/recovery-codes", m.handler.GetRecoveryCodes)
	twoFactor.Post("/recovery-codes", m.handler.RegenerateRecoveryCodes)

	// Current User's Teams
	router.Get("/teams", m.handler.GetUserTeams)
	router.Post("/switch-team/:teamId", m.handler.SwitchTeam)

	// Team Invitations (for accepting invitations)
	router.Post("/team-invitations/:invitationId/accept", m.handler.AcceptTeamInvitation)
}

// registerTeamRoutes registers team management routes
func (m *Module) registerTeamRoutes(router fiber.Router) {
	// Team CRUD
	router.Post("/", m.handler.CreateTeam)
	router.Get("/:teamId", TeamMemberMiddleware(m.service), m.handler.GetTeam)
	router.Put("/:teamId", TeamOwnerMiddleware(m.service), m.handler.UpdateTeam)
	router.Delete("/:teamId", TeamOwnerMiddleware(m.service), m.handler.DeleteTeam)

	// Team Members
	router.Get("/:teamId/members", TeamMemberMiddleware(m.service), m.handler.GetTeamMembers)
	router.Post("/:teamId/members", TeamAdminMiddleware(m.service), m.handler.InviteTeamMember)
	router.Put("/:teamId/members/:memberId", TeamOwnerMiddleware(m.service), m.handler.UpdateTeamMemberRole)
	router.Delete("/:teamId/members/:memberId", m.handler.RemoveTeamMember)

	// Team Invitations
	router.Get("/:teamId/invitations", TeamAdminMiddleware(m.service), m.handler.GetTeamInvitations)
	router.Delete("/:teamId/invitations/:invitationId", TeamAdminMiddleware(m.service), m.handler.CancelTeamInvitation)
}

// Service returns the auth service
func (m *Module) Service() *Service {
	return m.service
}

// Repository returns the auth repository
func (m *Module) Repository() *Repository {
	return m.repository
}

// Handler returns the auth handler
func (m *Module) Handler() *Handler {
	return m.handler
}

// AutoMigrate runs database migrations for auth models
func (m *Module) AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&User{},
		&Team{},
		&TeamMember{},
		&TeamInvitation{},
		&PersonalAccessToken{},
		&PasswordResetToken{},
	)
}

// GetModels returns all models for the auth module
func (m *Module) GetModels() []interface{} {
	return []interface{}{
		&User{},
		&Team{},
		&TeamMember{},
		&TeamInvitation{},
		&PersonalAccessToken{},
		&PasswordResetToken{},
	}
}
