package utils

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
)

// GenerateHexToken generates a random hex-encoded token of the specified length.
// The length specifies the number of hex characters (each byte = 2 hex chars).
//
// Deprecated: Use cryptoutil.MustGenerateHexToken(length/2) instead.
// Note: cryptoutil.MustGenerateHexToken takes byte count, this function takes hex char count.
func GenerateHexToken(length int) string {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	return hex.EncodeToString(bytes)
}

// GenerateBase64Token generates a random URL-safe base64-encoded token.
// The length specifies the final token length (truncated from the base64 output).
//
// Deprecated: Use cryptoutil.GenerateToken instead.
func GenerateBase64Token(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length]
}

// GenerateSecureToken generates a random URL-safe base64-encoded token
// with full-length output (no truncation). Useful for cryptographic purposes.
//
// Deprecated: Use cryptoutil.GenerateToken instead.
func GenerateSecureToken(byteLength int) (string, error) {
	bytes := make([]byte, byteLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// GenerateAppKey generates a Laravel-style application key.
func GenerateAppKey() string {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return ""
	}
	return "base64:" + base64.StdEncoding.EncodeToString(key)
}
