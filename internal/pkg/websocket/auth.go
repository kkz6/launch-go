package websocket

import (
	"context"
	"errors"
	"time"

	"github.com/gofiber/contrib/websocket"

	launchcache "github.com/kkz6/launch-go/internal/pkg/launch/cache"
	"github.com/kkz6/launch-go/internal/pkg/security"
)

// Claims represents the JWT claims for WebSocket authentication
type Claims struct {
	UserID string
	TeamID string
}

var (
	ErrMissingToken          = errors.New("missing authentication token")
	ErrMissingTeamID         = errors.New("missing team_id parameter")
	ErrNotTeamMember         = errors.New("not a member of this team")
	ErrMembershipUnavailable = errors.New("team membership validation unavailable")
)

const websocketMembershipTimeout = 5 * time.Second

// ValidateToken validates a JWT token and extracts user claims
// Note: team_id is no longer in the JWT, it's passed separately
func ValidateToken(tokenString, jwtSecret string) (string, error) {
	if tokenString == "" {
		return "", ErrMissingToken
	}

	claims, err := security.ParseJWTToken(tokenString, jwtSecret)
	if err != nil {
		return "", security.ErrInvalidToken
	}

	userID, err := security.ExtractJWTClaim(claims, "sub")
	if err != nil {
		return "", security.ErrMissingClaim
	}

	return userID, nil
}

// AuthenticateWebSocket validates the token and team_id from WebSocket query params
// Connection URL: /ws?token=xxx&team_id=xxx
func AuthenticateWebSocket(c *websocket.Conn, jwtSecret string, membershipCache *launchcache.TeamMembershipCache) (*Claims, error) {
	token := c.Query("token")
	teamID := c.Query("team_id")

	// Validate JWT token
	userID, err := ValidateToken(token, jwtSecret)
	if err != nil {
		return nil, err
	}

	// Validate team_id parameter
	if teamID == "" {
		return nil, ErrMissingTeamID
	}

	ctx, cancel := context.WithTimeout(context.Background(), websocketMembershipTimeout)
	defer cancel()
	if err := validateTeamMembership(ctx, userID, teamID, membershipCache); err != nil {
		return nil, err
	}

	return &Claims{
		UserID: userID,
		TeamID: teamID,
	}, nil
}

func validateTeamMembership(ctx context.Context, userID, teamID string, membershipCache *launchcache.TeamMembershipCache) error {
	if membershipCache == nil {
		return ErrMembershipUnavailable
	}
	isMember, err := membershipCache.IsMember(ctx, userID, teamID)
	if err != nil {
		return err
	}
	if !isMember {
		return ErrNotTeamMember
	}
	return nil
}

// AuthenticateWebSocketLegacy validates token with team_id in JWT (for backward compatibility)
// Deprecated: Use AuthenticateWebSocket with team_id query parameter instead
func AuthenticateWebSocketLegacy(c *websocket.Conn, jwtSecret string) (*Claims, error) {
	token := c.Query("token")

	if token == "" {
		return nil, ErrMissingToken
	}

	claims, err := security.ParseJWTToken(token, jwtSecret)
	if err != nil {
		return nil, security.ErrInvalidToken
	}

	userID, err := security.ExtractJWTClaim(claims, "sub")
	if err != nil {
		return nil, security.ErrMissingClaim
	}

	// Try to get team_id from query param first (new approach)
	teamID := c.Query("team_id")

	// Fall back to JWT claim (legacy support)
	if teamID == "" {
		teamID, _ = claims["team_id"].(string)
	}

	if teamID == "" {
		return nil, ErrMissingTeamID
	}

	return &Claims{
		UserID: userID,
		TeamID: teamID,
	}, nil
}
