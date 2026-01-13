package models

import (
	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// GenerateRandomToken generates a random token of the specified length.
// Deprecated: Use utils.GenerateBase64Token instead.
func GenerateRandomToken(length int) string {
	return utils.GenerateBase64Token(length)
}

// GenerateAppKey generates a Laravel-style application key.
// Deprecated: Use utils.GenerateAppKey instead.
func GenerateAppKey() string {
	return utils.GenerateAppKey()
}
