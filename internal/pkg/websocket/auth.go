package websocket

import (
	"context"
	"errors"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/golang-jwt/jwt/v5"

	launchcache "github.com/kkz6/launch-go/internal/pkg/launch/cache"
	"github.com/kkz6/launch-go/internal/pkg/security"
)

// Claims represents the JWT claims for WebSocket authentication
type Claims struct {
	UserID   string
	TeamID   string
	TeamRole string
	ReadOnly bool
	// Zero for spectate/undated tokens. Only a normal unexpired access
	// token may subscribe to the cross-team staff invalidation stream.
	StaffTokenExpiresAt time.Time
}

var (
	ErrMissingToken          = errors.New("missing authentication token")
	ErrMissingTeamID         = errors.New("missing team_id parameter")
	ErrNotTeamMember         = errors.New("not a member of this team")
	ErrMembershipUnavailable = errors.New("team membership validation unavailable")
	ErrAccountSuspended      = errors.New("account suspended")
)

const websocketMembershipTimeout = 5 * time.Second

// AuthenticateWebSocket validates the token and team_id from WebSocket query params
// Connection URL: /ws?token=xxx&team_id=xxx
func AuthenticateWebSocket(c *websocket.Conn, jwtSecret string, membershipCache *launchcache.TeamMembershipCache) (*Claims, error) {
	token := c.Query("token")
	teamID := c.Query("team_id")

	if token == "" {
		return nil, ErrMissingToken
	}
	identity, err := security.ParseAccessToken(token, jwtSecret)
	if err != nil {
		return nil, err
	}
	userID := identity["sub"].(string)
	if security.IsUserSuspended(userID) {
		return nil, ErrAccountSuspended
	}

	// Validate team_id parameter
	if teamID == "" {
		return nil, ErrMissingTeamID
	}

	ctx, cancel := context.WithTimeout(context.Background(), websocketMembershipTimeout)
	defer cancel()
	membership, err := validateTeamMembership(ctx, userID, teamID, membershipCache)
	if err != nil {
		return nil, err
	}

	return &Claims{
		UserID:              userID,
		TeamID:              teamID,
		TeamRole:            membership.Role,
		ReadOnly:            security.IsReadOnlyImpersonation(identity),
		StaffTokenExpiresAt: staffTokenExpiry(identity),
	}, nil
}

func validateTeamMembership(ctx context.Context, userID, teamID string, membershipCache *launchcache.TeamMembershipCache) (*launchcache.TeamMembership, error) {
	if membershipCache == nil {
		return nil, ErrMembershipUnavailable
	}
	membership, err := membershipCache.GetMembership(ctx, userID, teamID)
	if err != nil {
		return nil, err
	}
	if !membership.IsMember {
		return nil, ErrNotTeamMember
	}
	return membership, nil
}

// staffTokenExpiry applies the stricter staff-channel rules to validated claims.
func staffTokenExpiry(claims jwt.MapClaims) time.Time {
	readOnly, _ := claims["read_only"].(bool)
	if claims["type"] != "access" || readOnly {
		return time.Time{}
	}
	if impersonator, ok := claims["impersonator_id"].(string); ok && impersonator != "" {
		return time.Time{}
	}
	if session, ok := claims["impersonation_sid"].(string); ok && session != "" {
		return time.Time{}
	}
	expires, err := claims.GetExpirationTime()
	if err != nil || expires == nil {
		return time.Time{}
	}
	return expires.Time
}
