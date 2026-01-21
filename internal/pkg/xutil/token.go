package xutil

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
)

func GenerateHexToken(length int) string {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	return hex.EncodeToString(bytes)
}

func GenerateBase64Token(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length]
}

func GenerateSecureToken(byteLength int) (string, error) {
	bytes := make([]byte, byteLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

func GenerateAppKey() string {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return ""
	}
	return "base64:" + base64.StdEncoding.EncodeToString(key)
}
