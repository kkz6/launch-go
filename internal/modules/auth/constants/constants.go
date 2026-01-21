package constants

import "time"

// Defaults
const (
	DefaultTimezone = "UTC"
	DefaultPageSize = 15
	MaxPageSize     = 100
)

// Role names
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleEditor = "editor"
	RoleMember = "member"
)

// RoleDescriptions maps role names to their descriptions
var RoleDescriptions = map[string]string{
	RoleOwner:  "Full ownership with billing access",
	RoleAdmin:  "Full access to all resources",
	RoleEditor: "Can edit but not delete resources",
	RoleMember: "Read-only access to resources",
}

// AllRoles returns all available role names
var AllRoles = []string{RoleOwner, RoleAdmin, RoleEditor, RoleMember}

// Token types
const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

// Token durations
const (
	AccessTokenDuration  = 15 * time.Minute
	RefreshTokenDuration = 7 * 24 * time.Hour
)

// Password requirements
const (
	MinPasswordLength = 8
	MaxPasswordLength = 128
)

// Two-factor authentication
const (
	TwoFactorCodeLength   = 6
	TwoFactorCodeValidity = 5 * time.Minute
	RecoveryCodeCount     = 8
	RecoveryCodeLength    = 10
)

// Session settings
const (
	MaxActiveSessions = 5
	SessionTimeout    = 24 * time.Hour
)

// Team limits
const (
	MaxTeamNameLength        = 100
	MaxTeamMembersDefault    = 5
	MaxTeamMembersPremium    = 50
	MaxTeamMembersEnterprise = 500
)

// Invitation settings
const (
	InvitationExpiry = 7 * 24 * time.Hour
)

// Email verification
const (
	EmailVerificationExpiry = 24 * time.Hour
)

// Password reset
const (
	PasswordResetExpiry = time.Hour
)

// OAuth providers
const (
	ProviderGithub = "github"
	ProviderGoogle = "google"
	ProviderGitlab = "gitlab"
)
