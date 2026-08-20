package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/auth/contracts"
	"github.com/kkz6/launch-go/internal/modules/auth/dto"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	"github.com/kkz6/launch-go/internal/pkg/cache"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
	launchcache "github.com/kkz6/launch-go/internal/pkg/launch/cache"
	"github.com/kkz6/launch-go/internal/pkg/security"
	"github.com/kkz6/launch-go/internal/pkg/util"
)

const (
	defaultRefreshTokenHours    = 24 * 30 // 30 days
	twoFactorChallengeTTL       = 5 * time.Minute
	twoFactorChallengeKeyPrefix = "2fa_challenge:"
)

// TwoFactorChallengeData is stored in cache when a 2FA challenge is pending
type TwoFactorChallengeData struct {
	UserID    string `json:"user_id"`
	IPAddress string `json:"ip_address"`
	UserAgent string `json:"user_agent"`
}

// AuthService handles authentication-related operations
type AuthService struct {
	repos           contracts.RepositoryRegistry
	config          *config.Config
	logger          *zerolog.Logger
	cache           cache.Cache
	membershipCache *launchcache.TeamMembershipCache

	// platformInvites consumes a platform invite during registration (grants an
	// on_trial subscription on the new personal team). Wired from main.go via
	// SetPlatformInviteReader. When nil, the trial-grant step is skipped.
	platformInvites PlatformInviteReader
}

func (s *AuthService) SetMembershipCache(c *launchcache.TeamMembershipCache) {
	s.membershipCache = c
}

// NewAuthService creates a new AuthService instance
func NewAuthService(repos contracts.RepositoryRegistry, cfg *config.Config, logger *zerolog.Logger, c cache.Cache) *AuthService {
	return &AuthService{
		repos:  repos,
		config: cfg,
		logger: logger,
		cache:  c,
	}
}

// Register creates a new user account
func (s *AuthService) Register(ctx context.Context, req *dto.RegisterRequest) (*dto.AuthResponse, error) {
	req.Normalize()

	// Check if user exists
	exists, err := s.repos.User().ExistsByEmail(ctx, req.Email)
	if err != nil {
		return nil, err
	}

	if exists {
		return nil, errors.New("email already registered")
	}

	// Hash password
	hashedPassword, err := security.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	// Set default timezone
	timezone := req.Timezone
	if timezone == "" {
		timezone = "UTC"
	}

	// Create user, team, and handle invitation in a transaction
	user := &models.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: hashedPassword,
		Timezone: &timezone,
	}

	err = s.repos.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}

		// Handle invitation if provided
		if req.InvitationID != nil && *req.InvitationID != "" {
			if err := s.handleInvitation(ctx, tx, user, *req.InvitationID); err != nil {
				return err
			}
		}

		// Create personal team if requested and not joining via invitation
		if req.CreatePersonalTeam && (req.InvitationID == nil || *req.InvitationID == "") {
			if err := s.createPersonalTeam(ctx, tx, user); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Consume a platform invite if one was supplied. A bad/expired/used/
	// mismatched token must NEVER block signup — on any problem we simply
	// register normally without a trial.
	s.consumePlatformInvite(ctx, req, user)

	// Create session
	sessionID, err := s.createSession(ctx, user.ID, req.IPAddress, req.UserAgent)
	if err != nil {
		return nil, err
	}

	return s.buildAuthResponse(ctx, user, sessionID)
}

// consumePlatformInvite grants an on_trial subscription on the user's personal
// team when a valid, email-matched platform invite token was supplied during
// registration. It is best-effort: any error or mismatch is skipped so the
// registration still succeeds without a trial.
func (s *AuthService) consumePlatformInvite(ctx context.Context, req *dto.RegisterRequest, user *models.User) {
	if req.PlatformInviteToken == "" || s.platformInvites == nil {
		return
	}

	// A platform invite grants a trial on the user's personal team. If no
	// personal team was created (e.g. joined via a team invitation), there is
	// nothing to attach the trial to.
	if user.CurrentTeamID == nil {
		return
	}

	email, _, ok, err := s.platformInvites.PlatformInviteByToken(ctx, req.PlatformInviteToken)
	if err != nil || !ok {
		return
	}

	if !platformInviteEmailMatches(email, req.Email) {
		return
	}

	if err := s.platformInvites.AcceptPlatformInviteWithTrial(ctx, req.PlatformInviteToken, *user.CurrentTeamID); err != nil {
		s.logger.Error().
			Err(err).
			Str("user_id", user.ID).
			Str("team_id", *user.CurrentTeamID).
			Msg("failed to grant platform-invite trial during registration")
	}
}

// platformInviteEmailMatches reports whether the invite's email matches the
// registering user's email, case-insensitively. The trial is only granted on a
// match so a leaked token cannot be redeemed by a different account.
func platformInviteEmailMatches(inviteEmail, registerEmail string) bool {
	return strings.EqualFold(inviteEmail, registerEmail)
}

// Login authenticates a user. If the user has 2FA enabled, a challenge token
// is returned instead of auth tokens. The client must complete the 2FA challenge
// via the /auth/two-factor/challenge endpoint to receive tokens.
func (s *AuthService) Login(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResult, error) {
	req.Normalize()

	user, err := s.repos.User().FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, fiberutil.Unauthorized()
	}

	if user == nil {
		return nil, fiberutil.Unauthorized()
	}

	if !security.VerifyPassword(user.Password, req.Password) {
		return nil, fiberutil.Unauthorized()
	}

	// Block suspended accounts before issuing any token or 2FA challenge.
	if user.IsSuspended() {
		return nil, fiberutil.Forbidden("Account suspended")
	}

	// If 2FA is enabled, issue a challenge token instead of auth tokens
	if user.HasEnabledTwoFactorAuthentication() {
		challengeToken, err := s.createTwoFactorChallenge(ctx, user.ID, req.IPAddress, req.UserAgent)
		if err != nil {
			return nil, err
		}

		return &dto.LoginResult{
			TwoFactorRequired: true,
			ChallengeToken:    challengeToken,
			PreferredLocale:   user.Locale,
		}, nil
	}

	// No 2FA — create session and return tokens
	sessionID, err := s.createSession(ctx, user.ID, req.IPAddress, req.UserAgent)
	if err != nil {
		return nil, err
	}

	authResp, err := s.buildAuthResponse(ctx, user, sessionID)
	if err != nil {
		return nil, err
	}

	return &dto.LoginResult{PreferredLocale: user.Locale, AuthResponse: authResp}, nil
}

// createTwoFactorChallenge generates a challenge token and stores the pending
// challenge data in cache with a 5-minute TTL.
func (s *AuthService) createTwoFactorChallenge(ctx context.Context, userID, ipAddress, userAgent string) (string, error) {
	token, err := security.SecureToken()
	if err != nil {
		return "", fmt.Errorf("failed to generate challenge token: %w", err)
	}

	data := TwoFactorChallengeData{
		UserID:    userID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
	}

	encoded, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal challenge data: %w", err)
	}

	if err := s.cache.Set(ctx, twoFactorChallengeKeyPrefix+token, string(encoded), twoFactorChallengeTTL); err != nil {
		return "", fmt.Errorf("failed to store challenge token: %w", err)
	}

	return token, nil
}

// LookupTwoFactorChallenge retrieves and consumes a challenge token from cache.
// Returns the challenge data if found, or an unauthorized error if expired/invalid.
func (s *AuthService) LookupTwoFactorChallenge(ctx context.Context, challengeToken string) (*TwoFactorChallengeData, error) {
	key := twoFactorChallengeKeyPrefix + challengeToken
	raw, err := s.cache.Get(ctx, key)
	if err != nil {
		return nil, fiberutil.Unauthorized()
	}

	var data TwoFactorChallengeData
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, fiberutil.Unauthorized()
	}

	// Delete the challenge token to prevent reuse
	if err := s.cache.Delete(ctx, key); err != nil && s.logger != nil {
		s.logger.Warn().Err(err).Str("key", key).Msg("Failed to consume 2FA challenge token")
	}

	return &data, nil
}

// CompleteTwoFactorLogin creates a session and returns auth tokens for a
// user who has successfully passed 2FA verification.
func (s *AuthService) CompleteTwoFactorLogin(ctx context.Context, userID, ipAddress, userAgent string) (*dto.AuthResponse, error) {
	user, err := s.repos.User().FindByID(ctx, userID)
	if err != nil || user == nil {
		return nil, fiberutil.Unauthorized()
	}

	sessionID, err := s.createSession(ctx, user.ID, ipAddress, userAgent)
	if err != nil {
		return nil, err
	}

	return s.buildAuthResponse(ctx, user, sessionID)
}

// Logout invalidates the user's session
func (s *AuthService) Logout(ctx context.Context, userID, sessionID string) error {
	if sessionID != "" {
		return s.repos.Session().DeleteByUser(ctx, sessionID, userID)
	}

	return nil
}

// RefreshToken generates new access and refresh tokens
func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*dto.AuthResponse, error) {
	// Parse and validate refresh token
	token, err := jwt.Parse(refreshToken, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}

		return []byte(s.config.JWT.Secret), nil
	})

	if err != nil || !token.Valid {
		return nil, fiberutil.Unauthorized()
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fiberutil.Unauthorized()
	}

	// Check token type
	tokenType, ok := claims["type"].(string)
	if !ok || tokenType != "refresh" {
		return nil, fiberutil.Unauthorized()
	}

	userID, ok := claims["sub"].(string)
	if !ok {
		return nil, fiberutil.Unauthorized()
	}

	user, err := s.repos.User().FindByID(ctx, userID)
	if err != nil || user == nil {
		return nil, fiberutil.Unauthorized()
	}

	// Extract session_id from refresh token claims and validate session still exists
	var sessionID string
	if sid, ok := claims["session_id"].(string); ok {
		sessionID = sid

		// Verify the session has not been revoked
		exists, err := s.repos.Session().Exists(ctx, sessionID)
		if err != nil || !exists {
			return nil, fiberutil.Unauthorized()
		}

		if err := s.repos.Session().UpdateLastActivity(ctx, sessionID); err != nil && s.logger != nil {
			s.logger.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to update refresh session activity")
		}
	}

	accessToken, err := s.generateAccessToken(user, sessionID)
	if err != nil {
		return nil, err
	}

	newRefreshToken, err := s.generateRefreshToken(user, sessionID)
	if err != nil {
		return nil, err
	}

	isSubscribed := s.isTeamSubscribedOrUserAdmin(ctx, user)

	return &dto.AuthResponse{
		User:         dto.ToUserResponseWithStatus(user, isSubscribed, user.Onboarded),
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    s.config.JWT.Expiration * 3600,
		TokenType:    "Bearer",
	}, nil
}

// LoginWithPasskey creates a session and tokens for an already-verified passkey user
func (s *AuthService) LoginWithPasskey(ctx context.Context, user *models.User, ipAddress, userAgent string) (*dto.AuthResponse, error) {
	sessionID, err := s.createSession(ctx, user.ID, ipAddress, userAgent)
	if err != nil {
		return nil, err
	}

	return s.buildAuthResponse(ctx, user, sessionID)
}

// TokenExchange validates a Personal Access Token and returns JWT tokens
func (s *AuthService) TokenExchange(ctx context.Context, token string) (*dto.AuthResponse, error) {
	hash := sha256.Sum256([]byte(token))
	hashedToken := hex.EncodeToString(hash[:])

	pat, err := s.repos.PersonalAccessToken().FindByToken(ctx, hashedToken)
	if err != nil {
		return nil, fiberutil.Unauthorized()
	}

	if pat == nil {
		return nil, fiberutil.Unauthorized()
	}

	if pat.IsExpired() {
		return nil, fiberutil.Unauthorized()
	}

	if err := s.repos.PersonalAccessToken().UpdateLastUsed(ctx, pat.ID); err != nil && s.logger != nil {
		s.logger.Warn().Err(err).Str("token_id", pat.ID).Msg("Failed to update personal access token usage")
	}

	user, err := s.repos.User().FindByID(ctx, pat.TokenableID)
	if err != nil || user == nil {
		return nil, fiberutil.Unauthorized()
	}

	return s.buildAuthResponse(ctx, user, "")
}

// handleInvitation handles team invitation during registration
func (s *AuthService) handleInvitation(ctx context.Context, tx *gorm.DB, user *models.User, invitationID string) error {
	invitation, err := s.repos.TeamInvitation().FindByID(ctx, invitationID)
	if err != nil {
		return fmt.Errorf("failed to find invitation: %w", err)
	}

	if invitation == nil {
		// Invitation not found — not a hard error, just skip
		s.logger.Warn().Str("invitation_id", invitationID).Msg("Invitation not found during registration")
		return nil
	}

	if invitation.Email != user.Email {
		// Invitation is for a different email — skip silently
		return nil
	}

	// Add user to team
	role := "member"
	if invitation.Role != nil {
		role = *invitation.Role
	}

	if err := tx.Create(&models.TeamMember{
		TeamID: invitation.TeamID,
		UserID: user.ID,
		Role:   &role,
	}).Error; err != nil {
		return fmt.Errorf("failed to add user to team: %w", err)
	}

	// Set current team
	if err := tx.Model(user).Update("current_team_id", invitation.TeamID).Error; err != nil {
		return fmt.Errorf("failed to set current team: %w", err)
	}

	user.CurrentTeamID = &invitation.TeamID

	// Delete invitation
	if err := tx.Delete(invitation).Error; err != nil {
		return fmt.Errorf("failed to delete invitation: %w", err)
	}

	return nil
}

func (s *AuthService) AcceptTeamInvitation(
	ctx context.Context, invitationToken, name, password, ip, userAgent string,
) (*dto.AuthResponse, error) {
	invitation, err := s.repos.TeamInvitation().FindByID(ctx, invitationToken)
	if err != nil {
		return nil, err
	}
	if invitation == nil {
		return nil, fiberutil.NotFound()
	}

	existing, err := s.repos.User().FindByEmail(ctx, invitation.Email)
	if err != nil {
		return nil, err
	}

	var result *dto.AuthResponse
	if existing != nil {
		result, err = s.acceptTeamInvitationExistingUser(ctx, invitation, existing, password, ip, userAgent)
	} else {
		if strings.TrimSpace(name) == "" {
			return nil, fiberutil.Validation("Name is required to create your account")
		}
		result, err = s.Register(ctx, &dto.RegisterRequest{
			Name:                 name,
			Email:                invitation.Email,
			Password:             password,
			PasswordConfirmation: password,
			InvitationID:         &invitationToken,
			CreatePersonalTeam:   false,
			IPAddress:            ip,
			UserAgent:            userAgent,
		})
	}
	if err != nil {
		return nil, err
	}

	s.invalidateInvitationMembership(ctx, result.User.ID, invitation.TeamID)
	if invitation.Team != nil {
		team := dto.ToTeamResponse(*invitation.Team)
		result.User.CurrentTeam = &team
		result.User.CurrentTeamID = &invitation.TeamID
		activity.RecordWithLog(ctx, "auth", "joined", result.User.ID, invitation.Team, "Team invitation was accepted")
	}
	return result, nil
}

func (s *AuthService) acceptTeamInvitationExistingUser(
	ctx context.Context, invitation *models.TeamInvitation, user *models.User, password, ip, userAgent string,
) (*dto.AuthResponse, error) {
	if user.IsSuspended() {
		return nil, fiberutil.Forbidden("Account suspended")
	}
	if !security.VerifyPassword(user.Password, password) {
		return nil, fiberutil.Unauthorized()
	}

	role := "member"
	if invitation.Role != nil {
		role = *invitation.Role
	}

	err := s.repos.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&models.TeamMember{}).
			Where("team_id = ? AND user_id = ?", invitation.TeamID, user.ID).
			Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			if err := tx.Create(&models.TeamMember{
				TeamID: invitation.TeamID,
				UserID: user.ID,
				Role:   &role,
			}).Error; err != nil {
				return fmt.Errorf("failed to add user to team: %w", err)
			}
		}
		if err := tx.Model(&models.User{}).
			Where("id = ?", user.ID).
			Update("current_team_id", invitation.TeamID).Error; err != nil {
			return fmt.Errorf("failed to set current team: %w", err)
		}
		return tx.Delete(invitation).Error
	})
	if err != nil {
		return nil, err
	}
	user.CurrentTeamID = &invitation.TeamID
	user.CurrentTeam = invitation.Team

	sessionID, err := s.createSession(ctx, user.ID, ip, userAgent)
	if err != nil {
		return nil, err
	}
	return s.buildAuthResponse(ctx, user, sessionID)
}

func (s *AuthService) invalidateInvitationMembership(ctx context.Context, userID, teamID string) {
	if s.membershipCache != nil {
		_ = s.membershipCache.InvalidateMembership(ctx, userID, teamID)
	}
}

// createPersonalTeam creates a personal team for a new user
func (s *AuthService) createPersonalTeam(_ context.Context, tx *gorm.DB, user *models.User) error {
	team := &models.Team{
		Name:         user.Name + "'s Team",
		UserID:       user.ID,
		PersonalTeam: true,
	}

	if err := tx.Create(team).Error; err != nil {
		return err
	}

	// Add user to team as owner
	ownerRole := authtypes.TeamRoleOwner.String()
	if err := tx.Create(&models.TeamMember{
		TeamID: team.ID,
		UserID: user.ID,
		Role:   &ownerRole,
	}).Error; err != nil {
		return err
	}

	// Set current team
	if err := tx.Model(user).Update("current_team_id", team.ID).Error; err != nil {
		return err
	}

	user.CurrentTeamID = &team.ID
	user.CurrentTeam = team

	return nil
}

// createSession creates a session record and returns the session ID
func (s *AuthService) createSession(ctx context.Context, userID, ipAddress, userAgent string) (string, error) {
	session := &models.Session{
		UserID:       &userID,
		IPAddress:    nilIfEmpty(ipAddress),
		UserAgent:    nilIfEmpty(userAgent),
		Payload:      "",
		LastActivity: int(time.Now().Unix()),
	}

	if err := s.repos.Session().Create(ctx, session); err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	return session.ID, nil
}

func (s *AuthService) buildAuthResponse(ctx context.Context, user *models.User, sessionID string) (*dto.AuthResponse, error) {
	accessToken, err := s.generateAccessToken(user, sessionID)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.generateRefreshToken(user, sessionID)
	if err != nil {
		return nil, err
	}

	isSubscribed := s.isTeamSubscribedOrUserAdmin(ctx, user)

	return &dto.AuthResponse{
		User:         dto.ToUserResponseWithStatus(user, isSubscribed, user.Onboarded),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    s.config.JWT.Expiration * 3600,
		TokenType:    "Bearer",
	}, nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

// generateAccessToken generates a JWT access token
// Note: team_id is NOT included in the token. Team context is passed via X-Team-ID header
// and validated by the TeamContext middleware with cached membership checks.
func (s *AuthService) generateAccessToken(user *models.User, sessionID string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"jti":   util.NewULID(),
		"sub":   user.ID,
		"email": user.Email,
		"name":  user.Name,
		"type":  "access",
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour * time.Duration(s.config.JWT.Expiration)).Unix(),
	}

	if sessionID != "" {
		claims["session_id"] = sessionID
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(s.config.JWT.Secret))
}

// generateRefreshToken generates a JWT refresh token
func (s *AuthService) generateRefreshToken(user *models.User, sessionID string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"jti":  util.NewULID(),
		"sub":  user.ID,
		"type": "refresh",
		"iat":  now.Unix(),
		"exp":  now.Add(time.Hour * defaultRefreshTokenHours).Unix(),
	}

	if sessionID != "" {
		claims["session_id"] = sessionID
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(s.config.JWT.Secret))
}

// isTeamSubscribedOrUserAdmin checks if the user's current team is subscribed or user is admin
func (s *AuthService) isTeamSubscribedOrUserAdmin(ctx context.Context, user *models.User) bool {
	if user.CurrentTeamID == nil {
		return false
	}

	// Admin users bypass subscription check
	if s.repos.IsUserAdmin(ctx, user.ID) {
		return true
	}

	return s.repos.IsTeamSubscribed(ctx, *user.CurrentTeamID)
}
