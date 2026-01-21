package websocket

import (
	"context"
	"errors"

	"github.com/gofiber/contrib/websocket"

	"github.com/kkz6/launch-go/internal/pkg/cache"
	"github.com/kkz6/launch-go/internal/pkg/jwt"
)

// Claims represents the JWT claims for WebSocket authentication
type Claims struct {
	UserID string
	TeamID string
}

var (
	ErrMissingToken  = errors.New("missing authentication token")
	ErrMissingTeamID = errors.New("missing team_id parameter")
	ErrNotTeamMember = errors.New("not a member of this team")
)

// ValidateToken validates a JWT token and extracts user claims
// Note: team_id is no longer in the JWT, it's passed separately
func ValidateToken(tokenString, jwtSecret string) (string, error) {
	if tokenString == "" {
		return "", ErrMissingToken
	}

	claims, err := jwt.ParseToken(tokenString, jwtSecret)
	if err != nil {
		return "", jwt.ErrInvalidToken
	}

	userID, err := jwt.ExtractClaim(claims, "sub")
	if err != nil {
		return "", jwt.ErrMissingClaim
	}

	return userID, nil
}

// AuthenticateWebSocket validates the token and team_id from WebSocket query params
// Connection URL: /ws?token=xxx&team_id=xxx
func AuthenticateWebSocket(c *websocket.Conn, jwtSecret string, membershipCache *cache.TeamMembershipCache) (*Claims, error) {
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

	// Validate team membership (with caching)
	if membershipCache != nil {
		isMember, err := membershipCache.IsMember(context.Background(), userID, teamID)
		if err != nil {
			return nil, err
		}
		if !isMember {
			return nil, ErrNotTeamMember
		}
	}

	return &Claims{
		UserID: userID,
		TeamID: teamID,
	}, nil
}

// AuthenticateWebSocketLegacy validates token with team_id in JWT (for backward compatibility)
// Deprecated: Use AuthenticateWebSocket with team_id query parameter instead
func AuthenticateWebSocketLegacy(c *websocket.Conn, jwtSecret string) (*Claims, error) {
	token := c.Query("token")

	if token == "" {
		return nil, ErrMissingToken
	}

	claims, err := jwt.ParseToken(token, jwtSecret)
	if err != nil {
		return nil, jwt.ErrInvalidToken
	}

	userID, err := jwt.ExtractClaim(claims, "sub")
	if err != nil {
		return nil, jwt.ErrMissingClaim
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
