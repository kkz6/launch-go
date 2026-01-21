package token

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

// Encoding represents the output encoding for generated tokens.
type Encoding int

const (
	// Hex encoding produces lowercase hexadecimal output.
	Hex Encoding = iota
	// Base64 encoding uses standard base64 with padding.
	Base64
	// Base64URL encoding uses URL-safe base64 with padding.
	Base64URL
	// Base64URLRaw encoding uses URL-safe base64 without padding.
	Base64URLRaw
)

// Generator creates cryptographically secure random tokens.
type Generator struct {
	byteLength int
	encoding   Encoding
	prefix     string
}

// New creates a new Generator with the specified byte length.
func New(byteLength int) *Generator {
	return &Generator{
		byteLength: byteLength,
		encoding:   Hex,
		prefix:     "",
	}
}

// WithEncoding sets the encoding for the generated token.
func (g *Generator) WithEncoding(e Encoding) *Generator {
	g.encoding = e
	return g
}

// WithPrefix sets a prefix to prepend to the generated token.
func (g *Generator) WithPrefix(prefix string) *Generator {
	g.prefix = prefix
	return g
}

// Generate creates a new random token and returns it as a string.
func (g *Generator) Generate() (string, error) {
	bytes := make([]byte, g.byteLength)

	_, err := rand.Read(bytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	var encoded string
	switch g.encoding {
	case Hex:
		encoded = hex.EncodeToString(bytes)
	case Base64:
		encoded = base64.StdEncoding.EncodeToString(bytes)
	case Base64URL:
		encoded = base64.URLEncoding.EncodeToString(bytes)
	case Base64URLRaw:
		encoded = base64.RawURLEncoding.EncodeToString(bytes)
	default:
		encoded = hex.EncodeToString(bytes)
	}

	return g.prefix + encoded, nil
}

// MustGenerate creates a new random token or panics if generation fails.
func (g *Generator) MustGenerate() string {
	token, err := g.Generate()
	if err != nil {
		panic(err)
	}
	return token
}

// APIToken generates a token suitable for API authentication.
// Returns a 24-byte token with Base64URLRaw encoding and "lnch_" prefix.
func APIToken() string {
	return New(24).WithEncoding(Base64URLRaw).WithPrefix("lnch_").MustGenerate()
}

// WebhookSecret generates a secret suitable for webhook signature verification.
// Returns a 32-byte token with Hex encoding.
func WebhookSecret() string {
	return New(32).WithEncoding(Hex).MustGenerate()
}

// CallbackToken generates a token suitable for callback verification.
// Returns a 16-byte token with Base64URLRaw encoding.
func CallbackToken() string {
	return New(16).WithEncoding(Base64URLRaw).MustGenerate()
}

// VerificationCode generates a numeric verification code with the specified number of digits.
func VerificationCode(digits int) string {
	if digits <= 0 {
		digits = 6
	}

	var sb strings.Builder
	sb.Grow(digits)

	for i := 0; i < digits; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			panic(fmt.Errorf("failed to generate random digit: %w", err))
		}
		_ = sb.WriteByte('0' + byte(n.Int64()))
	}

	return sb.String()
}

// EncryptionKey generates 32 random bytes suitable for use as an encryption key.
func EncryptionKey() []byte {
	key := make([]byte, 32)

	_, err := rand.Read(key)
	if err != nil {
		panic(fmt.Errorf("failed to generate encryption key: %w", err))
	}

	return key
}
