package websocket

import (
	"errors"

	"github.com/gofiber/contrib/websocket"
	"github.com/golang-jwt/jwt/v5"
)

// Claims represents the JWT claims for WebSocket authentication
type Claims struct {
	UserID string
	TeamID string
}

var (
	ErrMissingToken   = errors.New("missing authentication token")
	ErrInvalidToken   = errors.New("invalid authentication token")
	ErrMissingClaims  = errors.New("missing required claims")
)

// ValidateToken validates a JWT token and extracts claims
func ValidateToken(tokenString, jwtSecret string) (*Claims, error) {
	if tokenString == "" {
		return nil, ErrMissingToken
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	})

	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidToken
	}

	userID, ok := mapClaims["sub"].(string)
	if !ok || userID == "" {
		return nil, ErrMissingClaims
	}

	teamID, ok := mapClaims["team_id"].(string)
	if !ok || teamID == "" {
		return nil, ErrMissingClaims
	}

	return &Claims{
		UserID: userID,
		TeamID: teamID,
	}, nil
}

// AuthenticateWebSocket validates the token from WebSocket query params
func AuthenticateWebSocket(c *websocket.Conn, jwtSecret string) (*Claims, error) {
	token := c.Query("token")
	return ValidateToken(token, jwtSecret)
}
