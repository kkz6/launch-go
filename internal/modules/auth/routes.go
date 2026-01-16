package auth

import (
	"github.com/gofiber/fiber/v2"

	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/auth/handlers"
	"github.com/kkz6/launch-go/internal/pkg/signedurl"
)

// RegisterRoutes registers all auth-related routes (legacy method)
func (m *Module) RegisterRoutes(router fiber.Router) {
	m.RegisterPublicRoutes(router)
}

// RegisterPublicRoutes implements routing.PublicRouteModule interface.
// Auth module is special because it handles its own auth middleware internally.
func (m *Module) RegisterPublicRoutes(router fiber.Router) {
	deps := m.Deps()

	// Create handlers
	handler := handlers.NewHandler(m.service)
	passkeyHandler := handlers.NewPasskeyHandler(m.passkeyRepo)

	authMiddleware := middleware.Auth(deps.Config.JWT.Secret)
	adapter := NewMiddlewareAdapter(m.service)

	// Auth routes
	auth := router.Group("/auth")

	// Public routes (no authentication required)
	registerPublicRoutes(auth, handler)

	// Protected routes (authentication required)
	protected := auth.Group("", authMiddleware)
	registerProtectedRoutes(protected, handler)

	// Team routes (require authentication)
	teams := router.Group("/teams", authMiddleware)
	registerTeamRoutes(teams, handler, adapter)

	// User routes (require authentication)
	user := router.Group("/user", authMiddleware)
	registerUserRoutes(user, passkeyHandler)
}

// registerPublicRoutes registers routes that don't require authentication
func registerPublicRoutes(router fiber.Router, handler *handlers.Handler) {
	// Registration and Login
	router.Post("/register", handler.Auth.Register)
	router.Post("/login", handler.Auth.Login)
	router.Post("/refresh", handler.Auth.RefreshToken)

	// Password Reset
	router.Post("/forgot-password", handler.Password.ForgotPassword)
	router.Post("/reset-password", handler.Password.ResetPassword)

	// Email Verification (public for verification links, requires signed URL)
	router.Get("/verify-email/:id/:hash", signedurl.RequireSignedURL(nil), handler.Email.VerifyEmail)

	// User Status Check (for login flow)
	router.Post("/check-user-status", handler.User.CheckUserStatus)
}

// registerProtectedRoutes registers routes that require authentication
func registerProtectedRoutes(router fiber.Router, handler *handlers.Handler) {
	// User Management
	router.Get("/user", handler.User.User)
	router.Put("/profile", handler.User.UpdateProfile)
	router.Put("/password", handler.User.ChangePassword)
	router.Delete("/account", handler.User.DeleteAccount)
	router.Post("/logout", handler.Auth.Logout)

	// Email Verification (resend)
	router.Post("/email/verification-notification", handler.Email.ResendVerificationEmail)

	// Two-Factor Authentication
	twoFactor := router.Group("/two-factor")
	twoFactor.Post("/enable", handler.TwoFactor.EnableTwoFactor)
	twoFactor.Post("/confirm", handler.TwoFactor.ConfirmTwoFactor)
	twoFactor.Delete("/disable", handler.TwoFactor.DisableTwoFactor)
	twoFactor.Post("/challenge", handler.TwoFactor.TwoFactorChallenge)
	twoFactor.Get("/recovery-codes", handler.TwoFactor.GetRecoveryCodes)
	twoFactor.Post("/recovery-codes", handler.TwoFactor.RegenerateRecoveryCodes)

	// Current User's Teams
	router.Get("/teams", handler.Team.GetUserTeams)
	router.Put("/current-team", handler.Team.SwitchTeam)

	// Team Invitations (for accepting invitations, requires signed URL)
	router.Post("/team-invitations/:invitationId/accept", signedurl.RequireSignedURL(nil), handler.TeamMember.AcceptTeamInvitation)
}

// registerTeamRoutes registers team management routes
func registerTeamRoutes(router fiber.Router, handler *handlers.Handler, adapter *MiddlewareAdapter) {
	// List all teams for current user
	router.Get("/", handler.Team.GetUserTeams)

	// Team CRUD
	router.Post("/", handler.Team.CreateTeam)
	router.Get("/:teamId", middleware.TeamMember(adapter), handler.Team.GetTeam)
	router.Put("/:teamId", middleware.TeamOwner(adapter), handler.Team.UpdateTeam)
	router.Delete("/:teamId", middleware.TeamOwner(adapter), handler.Team.DeleteTeam)

	// Team switching
	router.Post("/:teamId/switch", handler.Team.SwitchTeamByID)

	// Team Members
	router.Get("/:teamId/members", middleware.TeamMember(adapter), handler.TeamMember.GetTeamMembers)
	router.Post("/:teamId/members", middleware.TeamAdmin(adapter), handler.TeamMember.InviteTeamMember)
	router.Put("/:teamId/members/:userId", middleware.TeamOwner(adapter), handler.TeamMember.UpdateTeamMemberRole)
	router.Delete("/:teamId/members/:userId", handler.TeamMember.RemoveTeamMember)

	// Team Invitations
	router.Get("/:teamId/invitations", middleware.TeamAdmin(adapter), handler.TeamMember.GetTeamInvitations)
	router.Delete("/:teamId/invitations/:invitationId", middleware.TeamAdmin(adapter), handler.TeamMember.CancelTeamInvitation)
}

// registerUserRoutes registers user-related routes under /user
func registerUserRoutes(router fiber.Router, passkeyHandler *handlers.PasskeyHandler) {
	// Passkeys
	passkeys := router.Group("/passkeys")
	passkeys.Get("/", passkeyHandler.Index)
	passkeys.Put("/:id", passkeyHandler.Update)
	passkeys.Delete("/:id", passkeyHandler.Delete)
}
