package auth

import (
	"time"

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
	passkeyHandler := handlers.NewPasskeyHandler(m.service)

	authMiddleware := middleware.Auth(deps.Config.JWT.Secret, deps.DB)
	adapter := NewMiddlewareAdapter(m.service)

	// Auth routes
	auth := router.Group("/auth")

	// Public routes (no authentication required)
	m.setupPublicRoutes(auth, handler, passkeyHandler)

	// Protected routes (authentication required)
	protected := auth.Group("", authMiddleware)
	m.registerProtectedRoutes(protected, handler)

	// Team routes (require authentication)
	teams := router.Group("/teams", authMiddleware)
	m.registerTeamRoutes(teams, handler, adapter)

	// User routes (require authentication)
	user := router.Group("/user", authMiddleware)
	m.registerUserRoutes(user, passkeyHandler)
}

// authRateLimit returns a rate limiter for authentication endpoints.
// Limits to maxReqs per window to prevent brute-force attacks.
func (m *Module) authRateLimit(maxReqs int, window time.Duration) fiber.Handler {
	return middleware.RateLimitWithCache(middleware.RateLimitConfig{
		Max:    maxReqs,
		Window: window,
		Cache:  m.cache,
	})
}

// setupPublicRoutes registers routes that don't require authentication
func (m *Module) setupPublicRoutes(router fiber.Router, handler *handlers.Handler, passkeyHandler *handlers.PasskeyHandler) {
	// Rate limiters for sensitive endpoints
	authRL := m.authRateLimit(10, time.Minute)     // 10 req/min for login/register
	resetRL := m.authRateLimit(5, time.Minute)     // 5 req/min for password reset
	statusRL := m.authRateLimit(20, time.Minute)   // 20 req/min for status check
	passkeyRL := m.authRateLimit(10, time.Minute)  // 10 req/min for passkey auth
	twoFactorRL := m.authRateLimit(5, time.Minute) // 5 req/min for 2FA challenge

	// Registration and Login
	router.Post("/register", authRL, handler.Auth.Register)
	router.Post("/login", authRL, handler.Auth.Login)
	router.Post("/refresh", handler.Auth.RefreshToken)
	router.Post("/token", authRL, handler.Auth.TokenExchange)

	// Password Reset
	router.Post("/forgot-password", resetRL, handler.Password.ForgotPassword)
	router.Post("/reset-password", resetRL, handler.Password.ResetPassword)

	// Email Verification (public for verification links, requires signed URL)
	router.Get("/verify-email/:id/:hash", signedurl.RequireSignedURL(nil), handler.Email.VerifyEmail)

	// User Status Check (for login flow)
	router.Post("/check-user-status", statusRL, handler.User.CheckUserStatus)

	// Two-Factor Challenge (public — uses challenge token from login, not auth)
	router.Post("/two-factor/challenge", twoFactorRL, handler.TwoFactor.TwoFactorChallenge)

	// Passkey Authentication (guest)
	router.Post("/passkey/login/options", passkeyRL, passkeyHandler.BeginLogin)
	router.Post("/passkey/login/verify", passkeyRL, passkeyHandler.FinishLogin)
}

// registerProtectedRoutes registers routes that require authentication
func (m *Module) registerProtectedRoutes(router fiber.Router, handler *handlers.Handler) {
	// User Management
	router.Get("/user", handler.User.User)
	router.Put("/profile", handler.User.UpdateProfile)
	router.Put("/password", handler.User.ChangePassword)
	router.Post("/reset-onboarding", handler.User.ResetOnboarding)
	router.Delete("/account", handler.User.DeleteAccount)
	router.Post("/logout", handler.Auth.Logout)

	// Email Verification (resend)
	router.Post("/email/verification-notification", handler.Email.ResendVerificationEmail)

	// Two-Factor Authentication (management — requires auth)
	twoFactor := router.Group("/two-factor")
	twoFactor.Post("/enable", handler.TwoFactor.EnableTwoFactor)
	twoFactor.Post("/confirm", handler.TwoFactor.ConfirmTwoFactor)
	twoFactor.Delete("/disable", handler.TwoFactor.DisableTwoFactor)
	twoFactor.Get("/recovery-codes", handler.TwoFactor.GetRecoveryCodes)
	twoFactor.Post("/recovery-codes", handler.TwoFactor.RegenerateRecoveryCodes)

	// Current User's Teams
	router.Get("/teams", handler.Team.GetUserTeams)
	router.Put("/current-team", handler.Team.SwitchTeam)

	// Team Invitations (for accepting invitations, requires signed URL)
	router.Post("/team-invitations/:invitationId/accept", signedurl.RequireSignedURL(nil), handler.TeamMember.AcceptTeamInvitation)
}

// registerTeamRoutes registers team management routes
func (m *Module) registerTeamRoutes(router fiber.Router, handler *handlers.Handler, adapter *MiddlewareAdapter) {
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

	// Team Invitations (POST /:teamId/invitations is an alias for POST /:teamId/members for API compatibility)
	router.Post("/:teamId/invitations", middleware.TeamAdmin(adapter), handler.TeamMember.InviteTeamMember)
	router.Get("/:teamId/invitations", middleware.TeamAdmin(adapter), handler.TeamMember.GetTeamInvitations)
	router.Delete("/:teamId/invitations/:invitationId", middleware.TeamAdmin(adapter), handler.TeamMember.CancelTeamInvitation)
}

// registerUserRoutes registers user-related routes under /user
func (m *Module) registerUserRoutes(router fiber.Router, passkeyHandler *handlers.PasskeyHandler) {
	// Passkeys
	passkeys := router.Group("/passkeys")
	passkeys.Get("/", passkeyHandler.Index)
	passkeys.Post("/register/options", passkeyHandler.BeginRegistration)
	passkeys.Post("/register", passkeyHandler.FinishRegistration)
	passkeys.Patch("/:id", passkeyHandler.Update)
	passkeys.Delete("/:id", passkeyHandler.Delete)

	// Personal Access Tokens
	patHandler := handlers.NewPATHandler(m.repos.PersonalAccessToken(), m.service.TwoFactor, m.repos.User())
	tokens := router.Group("/tokens")
	tokens.Get("/", patHandler.List)
	tokens.Post("/", patHandler.Create)
	tokens.Delete("/:id", patHandler.Delete)

	// Sessions
	sessionHandler := handlers.NewSessionHandler(m.repos.Session())
	sessions := router.Group("/sessions")
	sessions.Get("/", sessionHandler.List)
	sessions.Delete("/", sessionHandler.RevokeOthers)
	sessions.Delete("/:id", sessionHandler.Revoke)
}
