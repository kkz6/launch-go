package cryptoutil

import (
	"github.com/kkz6/launch-go/internal/pkg/token"
)

// GenerateToken generates a base64url-encoded token.
// Deprecated: Use token.New().WithEncoding(token.Base64URL).Generate() instead.
func GenerateToken(length int) string {
	return token.New(length).WithEncoding(token.Base64URL).MustGenerate()
}

// GenerateSecureBytes generates cryptographically secure random bytes.
// Deprecated: Use token.SecureBytes instead.
func GenerateSecureBytes(length int) ([]byte, error) {
	return token.SecureBytes(length)
}

// GenerateHexToken generates a hex-encoded token.
// Deprecated: Use token.HexToken instead.
func GenerateHexToken(length int) (string, error) {
	return token.HexToken(length)
}

// MustGenerateHexToken generates a hex-encoded token or panics.
// Deprecated: Use token.MustHexToken instead.
func MustGenerateHexToken(length int) string {
	return token.MustHexToken(length)
}

// GenerateWebhookSecret generates a 32-byte hex-encoded webhook secret.
// Deprecated: Use token.WebhookSecret instead.
func GenerateWebhookSecret() (string, error) {
	return token.WebhookSecret()
}

// GenerateAPIKey generates a base64url-encoded API key.
// Deprecated: Use token.MustSecureToken instead.
func GenerateAPIKey() string {
	return token.MustSecureToken()
}
