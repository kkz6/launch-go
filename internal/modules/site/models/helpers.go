package models

import (
	"github.com/kkz6/launch-go/internal/pkg/cryptoutil"
)

// GenerateRandomToken generates a random token of the specified length.
// Deprecated: Use cryptoutil.GenerateToken instead.
func GenerateRandomToken(length int) string {
	return cryptoutil.GenerateToken(length)
}

// GenerateAppKey generates a Laravel-style application key.
// Deprecated: Use cryptoutil.GenerateAPIKey or implement Laravel-specific key generation.
func GenerateAppKey() string {
	return "base64:" + cryptoutil.GenerateToken(32)
}
