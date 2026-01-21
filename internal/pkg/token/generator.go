package token

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
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
