package cryptoutil

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

func GenerateToken(length int) string {
	bytes, err := GenerateSecureBytes(length)
	if err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(bytes)
}

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

func GenerateHexToken(length int) (string, error) {
	bytes, err := GenerateSecureBytes(length)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func MustGenerateHexToken(length int) string {
	token, err := GenerateHexToken(length)
	if err != nil {
		panic(err)
	}
	return token
}

func GenerateWebhookSecret() (string, error) {
	return GenerateHexToken(32)
}

func GenerateAPIKey() string {
	return GenerateToken(32)
}
