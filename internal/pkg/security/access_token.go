package security

import (
	"errors"

	"github.com/golang-jwt/jwt/v5"
)

var ErrSessionRevoked = errors.New("session revoked")

// Identity lookups are configured once at startup and shared by HTTP and
// WebSocket authentication. Nil lookups preserve contexts without account storage.
var identityChecks struct {
	sessionLive   func(string) bool
	userSuspended func(string) bool
}

func SetSessionValidator(validate func(string) bool) {
	identityChecks.sessionLive = validate
}

func SetUserSuspensionChecker(check func(string) bool) {
	identityChecks.userSuspended = check
}

// ParseAccessToken validates identity and session liveness. Account restrictions
// are separate so optional authentication can still expose a valid identity.
func ParseAccessToken(tokenString, secret string) (jwt.MapClaims, error) {
	claims, err := ParseJWTToken(tokenString, secret)
	if err != nil {
		return nil, err
	}
	if err := ValidateJWTTokenType(claims, "access"); err != nil {
		return nil, err
	}
	if _, err := ExtractJWTClaim(claims, "sub"); err != nil {
		return nil, err
	}
	if sessionID, ok := claims["session_id"].(string); ok && sessionID != "" {
		if identityChecks.sessionLive != nil && !identityChecks.sessionLive(sessionID) {
			return nil, ErrSessionRevoked
		}
	}
	return claims, nil
}

func IsUserSuspended(userID string) bool {
	return userID != "" && identityChecks.userSuspended != nil && identityChecks.userSuspended(userID)
}

func IsReadOnlyImpersonation(claims jwt.MapClaims) bool {
	_, impersonating := claims["impersonation_sid"].(string)
	readOnly, _ := claims["read_only"].(bool)
	return impersonating && readOnly
}
