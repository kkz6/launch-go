package token

import (
	"encoding/base64"
	"encoding/hex"
	"regexp"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	g := New(16)

	if g.byteLength != 16 {
		t.Errorf("expected byteLength 16, got %d", g.byteLength)
	}

	if g.encoding != Hex {
		t.Errorf("expected default encoding Hex, got %d", g.encoding)
	}

	if g.prefix != "" {
		t.Errorf("expected empty prefix, got %q", g.prefix)
	}
}

func TestWithEncoding(t *testing.T) {
	g := New(16).WithEncoding(Base64URL)

	if g.encoding != Base64URL {
		t.Errorf("expected encoding Base64URL, got %d", g.encoding)
	}
}

func TestWithPrefix(t *testing.T) {
	g := New(16).WithPrefix("test_")

	if g.prefix != "test_" {
		t.Errorf("expected prefix 'test_', got %q", g.prefix)
	}
}

func TestGenerateHex(t *testing.T) {
	byteLength := 16
	g := New(byteLength).WithEncoding(Hex)

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

func TestGenerateBase64(t *testing.T) {
	byteLength := 24
	g := New(byteLength).WithEncoding(Base64)

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
	g := New(byteLength).WithEncoding(Base64URL)

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
	g := New(byteLength).WithEncoding(Base64URLRaw)

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
	g := New(byteLength).WithEncoding(Hex).WithPrefix(prefix)

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
	g := New(16).WithEncoding(Hex)

	token := g.MustGenerate()
	if len(token) != 32 {
		t.Errorf("expected token length 32, got %d", len(token))
	}
}

func TestAPIToken(t *testing.T) {
	token := APIToken()

	if !strings.HasPrefix(token, "lnch_") {
		t.Errorf("API token should have 'lnch_' prefix, got %q", token)
	}

	withoutPrefix := strings.TrimPrefix(token, "lnch_")
	expectedLen := base64.RawURLEncoding.EncodedLen(24)
	if len(withoutPrefix) != expectedLen {
		t.Errorf("expected encoded part length %d, got %d", expectedLen, len(withoutPrefix))
	}

	_, err := base64.RawURLEncoding.DecodeString(withoutPrefix)
	if err != nil {
		t.Errorf("token is not valid base64url-raw: %v", err)
	}
}

func TestWebhookSecret(t *testing.T) {
	secret := WebhookSecret()

	expectedLen := 64
	if len(secret) != expectedLen {
		t.Errorf("expected webhook secret length %d, got %d", expectedLen, len(secret))
	}

	_, err := hex.DecodeString(secret)
	if err != nil {
		t.Errorf("webhook secret is not valid hex: %v", err)
	}
}

func TestCallbackToken(t *testing.T) {
	token := CallbackToken()

	expectedLen := base64.RawURLEncoding.EncodedLen(16)
	if len(token) != expectedLen {
		t.Errorf("expected callback token length %d, got %d", expectedLen, len(token))
	}

	_, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Errorf("callback token is not valid base64url-raw: %v", err)
	}
}

func TestVerificationCode(t *testing.T) {
	tests := []struct {
		name   string
		digits int
		want   int
	}{
		{"6 digits", 6, 6},
		{"4 digits", 4, 4},
		{"8 digits", 8, 8},
		{"10 digits", 10, 10},
		{"zero defaults to 6", 0, 6},
		{"negative defaults to 6", -1, 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := VerificationCode(tt.digits)

			if len(code) != tt.want {
				t.Errorf("expected code length %d, got %d", tt.want, len(code))
			}

			matched, err := regexp.MatchString(`^\d+$`, code)
			if err != nil {
				t.Fatalf("regex error: %v", err)
			}
			if !matched {
				t.Errorf("verification code should only contain digits, got %q", code)
			}
		})
	}
}

func TestEncryptionKey(t *testing.T) {
	key := EncryptionKey()

	if len(key) != 32 {
		t.Errorf("expected encryption key length 32, got %d", len(key))
	}

	key2 := EncryptionKey()
	if string(key) == string(key2) {
		t.Error("encryption keys should be unique")
	}
}

func TestTokenUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	g := New(16).WithEncoding(Hex)

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
	token := New(24).
		WithEncoding(Base64URLRaw).
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
