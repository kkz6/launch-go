package models

import (
	"crypto/rand"
	"encoding/base64"
)

// GenerateRandomToken generates a random token of the specified length
func GenerateRandomToken(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}

	return base64.URLEncoding.EncodeToString(bytes)[:length]
}

// GenerateAppKey generates a Laravel-style application key
func GenerateAppKey() string {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return ""
	}

	return "base64:" + base64.StdEncoding.EncodeToString(key)
}
