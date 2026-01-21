package xutil

import (
	"github.com/kkz6/launch-go/internal/pkg/token"
)

// GenerateHexToken generates a hex-encoded token.
// Deprecated: Use token.MustHexToken instead.
func GenerateHexToken(length int) string {
	return token.MustHexToken(length / 2)
}

// GenerateBase64Token generates a base64url-encoded token truncated to length.
// Deprecated: Use token.New().WithEncoding(token.Base64URL).Generate() instead.
func GenerateBase64Token(length int) string {
	t := token.New(length).WithEncoding(token.Base64URL).MustGenerate()
	if len(t) > length {
		return t[:length]
	}
	return t
}

// GenerateSecureToken generates a base64url-encoded token from byteLength random bytes.
// Deprecated: Use token.SecureToken or token.New().Generate() instead.
func GenerateSecureToken(byteLength int) (string, error) {
	return token.New(byteLength).WithEncoding(token.Base64URL).Generate()
}

// GenerateAppKey generates a Laravel-compatible application key.
// Deprecated: Use token.AppKey instead.
func GenerateAppKey() string {
	return token.AppKey()
}
