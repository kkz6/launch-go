package cryptoutil

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// GenerateToken generates a cryptographically secure random token
// of the specified length using URL-safe base64 encoding.
//
// The length parameter specifies the number of random bytes to generate.
// The resulting token will be longer due to base64 encoding.
func GenerateToken(length int) string {
	bytes, err := GenerateSecureBytes(length)
	if err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(bytes)
}

// GenerateSecureBytes generates cryptographically secure random bytes
// of the specified length.
//
// Returns an error if the system's random number generator fails.
func GenerateSecureBytes(length int) ([]byte, error) {
	if length <= 0 {
		return nil, fmt.Errorf("length must be positive")
	}

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return bytes, nil
}

// GenerateHexToken generates a cryptographically secure random token
// encoded as a hexadecimal string.
//
// The length parameter specifies the number of random bytes to generate.
// The resulting token will be twice as long (2 hex chars per byte).
func GenerateHexToken(length int) (string, error) {
	bytes, err := GenerateSecureBytes(length)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// MustGenerateHexToken is like GenerateHexToken but panics on error.
// Use this only when you are certain the system's random number generator
// will not fail (e.g., during initialization).
func MustGenerateHexToken(length int) string {
	token, err := GenerateHexToken(length)
	if err != nil {
		panic(err)
	}
	return token
}

// GenerateWebhookSecret generates a cryptographically secure secret
// suitable for webhook signature verification.
// Returns a 32-byte hex-encoded string (64 characters).
func GenerateWebhookSecret() (string, error) {
	return GenerateHexToken(32)
}

// GenerateAPIKey generates a cryptographically secure API key.
// Returns a 32-byte URL-safe base64-encoded string.
func GenerateAPIKey() string {
	return GenerateToken(32)
}
