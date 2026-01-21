package jwtutil

import (
	"errors"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken         = errors.New("invalid token")
	ErrInvalidSigningMethod = errors.New("invalid signing method")
	ErrMissingClaim         = errors.New("missing required claim")
	ErrInvalidTokenType     = errors.New("invalid token type")
)

// ParseToken parses and validates a JWT token string
func ParseToken(tokenString, secret string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidSigningMethod
		}
		return []byte(secret), nil
	})

	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// ExtractClaim extracts a string claim from the claims map
func ExtractClaim(claims jwt.MapClaims, key string) (string, error) {
	value, ok := claims[key].(string)
	if !ok || value == "" {
		return "", ErrMissingClaim
	}
	return value, nil
}

// ValidateTokenType validates that the token has the expected type
func ValidateTokenType(claims jwt.MapClaims, expectedType string) error {
	tokenType, err := ExtractClaim(claims, "type")
	if err != nil {
		return err
	}
	if tokenType != expectedType {
		return ErrInvalidTokenType
	}
	return nil
}
