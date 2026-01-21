package security

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// TokenEncoding represents the output encoding for generated tokens.
type TokenEncoding int

const (
	// TokenHex encoding produces lowercase hexadecimal output.
	TokenHex TokenEncoding = iota
	// TokenBase64 encoding uses standard base64 with padding.
	TokenBase64
	// TokenBase64URL encoding uses URL-safe base64 with padding.
	TokenBase64URL
	// TokenBase64URLRaw encoding uses URL-safe base64 without padding.
	TokenBase64URLRaw
)

// TokenGenerator creates cryptographically secure random tokens.
type TokenGenerator struct {
	byteLength int
	encoding   TokenEncoding
	prefix     string
}

// NewTokenGenerator creates a new TokenGenerator with the specified byte length.
func NewTokenGenerator(byteLength int) *TokenGenerator {
	return &TokenGenerator{
		byteLength: byteLength,
		encoding:   TokenHex,
		prefix:     "",
	}
}

// WithEncoding sets the encoding for the generated token.
func (g *TokenGenerator) WithEncoding(e TokenEncoding) *TokenGenerator {
	g.encoding = e
	return g
}

// WithPrefix sets a prefix to prepend to the generated token.
func (g *TokenGenerator) WithPrefix(prefix string) *TokenGenerator {
	g.prefix = prefix
	return g
}

// Generate creates a new random token and returns it as a string.
func (g *TokenGenerator) Generate() (string, error) {
	bytes := make([]byte, g.byteLength)

	_, err := rand.Read(bytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	var encoded string
	switch g.encoding {
	case TokenHex:
		encoded = hex.EncodeToString(bytes)
	case TokenBase64:
		encoded = base64.StdEncoding.EncodeToString(bytes)
	case TokenBase64URL:
		encoded = base64.URLEncoding.EncodeToString(bytes)
	case TokenBase64URLRaw:
		encoded = base64.RawURLEncoding.EncodeToString(bytes)
	default:
		encoded = hex.EncodeToString(bytes)
	}

	return g.prefix + encoded, nil
}

// MustGenerate creates a new random token or panics if generation fails.
func (g *TokenGenerator) MustGenerate() string {
	token, err := g.Generate()
	if err != nil {
		panic(err)
	}
	return token
}

// SecureBytes generates cryptographically secure random bytes.
func SecureBytes(length int) ([]byte, error) {
	if length <= 0 {
		return nil, fmt.Errorf("length must be positive")
	}

	bytes := make([]byte, length)

	_, err := rand.Read(bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}

	return bytes, nil
}

// MustSecureBytes generates cryptographically secure random bytes or panics.
func MustSecureBytes(length int) []byte {
	bytes, err := SecureBytes(length)
	if err != nil {
		panic(err)
	}

	return bytes
}

// Convenience functions for common token generation patterns.

// SecureToken generates a 32-byte base64url-encoded token.
func SecureToken() (string, error) {
	return NewTokenGenerator(32).WithEncoding(TokenBase64URL).Generate()
}

// MustSecureToken generates a 32-byte base64url-encoded token or panics.
func MustSecureToken() string {
	return NewTokenGenerator(32).WithEncoding(TokenBase64URL).MustGenerate()
}

// HexToken generates a hex-encoded token of the specified byte length.
func HexToken(byteLength int) (string, error) {
	return NewTokenGenerator(byteLength).WithEncoding(TokenHex).Generate()
}

// MustHexToken generates a hex-encoded token or panics.
func MustHexToken(byteLength int) string {
	return NewTokenGenerator(byteLength).WithEncoding(TokenHex).MustGenerate()
}

// AppKey generates a Laravel-compatible application key (base64:...).
func AppKey() string {
	encoded := NewTokenGenerator(32).WithEncoding(TokenBase64).MustGenerate()
	return "base64:" + encoded
}

// WebhookSecret generates a 32-byte hex-encoded webhook secret.
func WebhookSecret() (string, error) {
	return HexToken(32)
}
