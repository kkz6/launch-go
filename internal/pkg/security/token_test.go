package security

import (
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

func TestNewTokenGenerator(t *testing.T) {
	g := NewTokenGenerator(16)

	if g.byteLength != 16 {
		t.Errorf("expected byteLength 16, got %d", g.byteLength)
	}

	if g.encoding != TokenHex {
		t.Errorf("expected default encoding Hex, got %d", g.encoding)
	}

	if g.prefix != "" {
		t.Errorf("expected empty prefix, got %q", g.prefix)
	}
}

func TestWithEncoding(t *testing.T) {
	g := NewTokenGenerator(16).WithEncoding(TokenBase64URL)

	if g.encoding != TokenBase64URL {
		t.Errorf("expected encoding Base64URL, got %d", g.encoding)
	}
}

func TestWithPrefix(t *testing.T) {
	g := NewTokenGenerator(16).WithPrefix("test_")

	if g.prefix != "test_" {
		t.Errorf("expected prefix 'test_', got %q", g.prefix)
	}
}

func TestGenerateHex(t *testing.T) {
	byteLength := 16
	g := NewTokenGenerator(byteLength).WithEncoding(TokenHex)

	token, err := g.Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	expectedLen := byteLength * 2
	if len(token) != expectedLen {
		t.Errorf("expected hex token length %d, got %d", expectedLen, len(token))
	}

	_, err = hex.DecodeString(token)
	if err != nil {
		t.Errorf("token is not valid hex: %v", err)
	}
}

func TestGenerateRejectsNonPositiveLength(t *testing.T) {
	for _, length := range []int{0, -1} {
		_, err := NewTokenGenerator(length).Generate()
		if err == nil {
			t.Errorf("expected error for length %d", length)
		}
	}
}

func TestGenerateBase64(t *testing.T) {
	byteLength := 24
	g := NewTokenGenerator(byteLength).WithEncoding(TokenBase64)

	token, err := g.Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	expectedLen := base64.StdEncoding.EncodedLen(byteLength)
	if len(token) != expectedLen {
		t.Errorf("expected base64 token length %d, got %d", expectedLen, len(token))
	}

	decoded, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		t.Errorf("token is not valid base64: %v", err)
	}

	if len(decoded) != byteLength {
		t.Errorf("decoded length %d doesn't match byte length %d", len(decoded), byteLength)
	}
}

func TestGenerateBase64URL(t *testing.T) {
	byteLength := 24
	g := NewTokenGenerator(byteLength).WithEncoding(TokenBase64URL)

	token, err := g.Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	expectedLen := base64.URLEncoding.EncodedLen(byteLength)
	if len(token) != expectedLen {
		t.Errorf("expected base64url token length %d, got %d", expectedLen, len(token))
	}

	decoded, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		t.Errorf("token is not valid base64url: %v", err)
	}

	if len(decoded) != byteLength {
		t.Errorf("decoded length %d doesn't match byte length %d", len(decoded), byteLength)
	}
}

func TestGenerateBase64URLRaw(t *testing.T) {
	byteLength := 24
	g := NewTokenGenerator(byteLength).WithEncoding(TokenBase64URLRaw)

	token, err := g.Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	expectedLen := base64.RawURLEncoding.EncodedLen(byteLength)
	if len(token) != expectedLen {
		t.Errorf("expected base64url-raw token length %d, got %d", expectedLen, len(token))
	}

	if strings.Contains(token, "=") {
		t.Error("base64url-raw token should not contain padding")
	}

	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Errorf("token is not valid base64url-raw: %v", err)
	}

	if len(decoded) != byteLength {
		t.Errorf("decoded length %d doesn't match byte length %d", len(decoded), byteLength)
	}
}

func TestGenerateWithPrefix(t *testing.T) {
	prefix := "lnch_"
	byteLength := 16
	g := NewTokenGenerator(byteLength).WithEncoding(TokenHex).WithPrefix(prefix)

	token, err := g.Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if !strings.HasPrefix(token, prefix) {
		t.Errorf("token should start with prefix %q, got %q", prefix, token)
	}

	withoutPrefix := strings.TrimPrefix(token, prefix)
	expectedLen := byteLength * 2
	if len(withoutPrefix) != expectedLen {
		t.Errorf("expected encoded part length %d, got %d", expectedLen, len(withoutPrefix))
	}
}

func TestMustGenerate(t *testing.T) {
	g := NewTokenGenerator(16).WithEncoding(TokenHex)

	token := g.MustGenerate()
	if len(token) != 32 {
		t.Errorf("expected token length 32, got %d", len(token))
	}
}

func TestSecureBytes(t *testing.T) {
	t.Run("generates correct length", func(t *testing.T) {
		bytes, err := SecureBytes(32)
		if err != nil {
			t.Fatalf("SecureBytes() error: %v", err)
		}

		if len(bytes) != 32 {
			t.Errorf("expected length 32, got %d", len(bytes))
		}
	})

	t.Run("returns error for zero length", func(t *testing.T) {
		_, err := SecureBytes(0)
		if err == nil {
			t.Error("expected error for zero length")
		}
	})

	t.Run("returns error for negative length", func(t *testing.T) {
		_, err := SecureBytes(-1)
		if err == nil {
			t.Error("expected error for negative length")
		}
	})

	t.Run("generates unique bytes", func(t *testing.T) {
		bytes1, _ := SecureBytes(32)
		bytes2, _ := SecureBytes(32)

		if string(bytes1) == string(bytes2) {
			t.Error("expected unique bytes")
		}
	})
}

func TestMustSecureBytes(t *testing.T) {
	bytes := MustSecureBytes(32)

	if len(bytes) != 32 {
		t.Errorf("expected length 32, got %d", len(bytes))
	}
}

func TestMustSecureBytesPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for zero length")
		}
	}()

	MustSecureBytes(0)
}

func TestTokenUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	g := NewTokenGenerator(16).WithEncoding(TokenHex)

	for i := 0; i < 1000; i++ {
		token, err := g.Generate()
		if err != nil {
			t.Fatalf("Generate() error: %v", err)
		}

		if seen[token] {
			t.Errorf("duplicate token generated: %s", token)
		}
		seen[token] = true
	}
}

func TestBuilderChaining(t *testing.T) {
	token := NewTokenGenerator(24).
		WithEncoding(TokenBase64URLRaw).
		WithPrefix("test_").
		MustGenerate()

	if !strings.HasPrefix(token, "test_") {
		t.Errorf("expected prefix 'test_', got %q", token)
	}

	withoutPrefix := strings.TrimPrefix(token, "test_")
	expectedLen := base64.RawURLEncoding.EncodedLen(24)
	if len(withoutPrefix) != expectedLen {
		t.Errorf("expected encoded part length %d, got %d", expectedLen, len(withoutPrefix))
	}
}
