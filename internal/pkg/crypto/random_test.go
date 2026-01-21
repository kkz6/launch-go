package crypto

import (
	"strings"
	"testing"
)

func TestGenerateSecureBytes(t *testing.T) {
	tests := []struct {
		name    string
		length  int
		wantErr bool
	}{
		{
			name:    "valid length",
			length:  32,
			wantErr: false,
		},
		{
			name:    "length 1",
			length:  1,
			wantErr: false,
		},
		{
			name:    "zero length",
			length:  0,
			wantErr: true,
		},
		{
			name:    "negative length",
			length:  -1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateSecureBytes(tt.length)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateSecureBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) != tt.length {
				t.Errorf("GenerateSecureBytes() length = %d, want %d", len(got), tt.length)
			}
		})
	}
}

func TestGenerateSecureBytes_Uniqueness(t *testing.T) {
	// Generate multiple tokens and ensure they are unique
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		bytes, err := GenerateSecureBytes(32)
		if err != nil {
			t.Fatalf("GenerateSecureBytes() unexpected error: %v", err)
		}
		key := string(bytes)
		if seen[key] {
			t.Errorf("GenerateSecureBytes() produced duplicate value")
		}
		seen[key] = true
	}
}

func TestGenerateToken(t *testing.T) {
	token := GenerateToken(32)
	if token == "" {
		t.Error("GenerateToken() returned empty string")
	}

	// Token should be base64 encoded (longer than input bytes)
	if len(token) < 32 {
		t.Errorf("GenerateToken() length = %d, expected >= 32", len(token))
	}

	// Token should be URL-safe
	if strings.ContainsAny(token, "+/") {
		t.Error("GenerateToken() contains non-URL-safe characters")
	}
}

func TestGenerateHexToken(t *testing.T) {
	token, err := GenerateHexToken(16)
	if err != nil {
		t.Fatalf("GenerateHexToken() unexpected error: %v", err)
	}

	// Hex token should be twice the byte length
	if len(token) != 32 {
		t.Errorf("GenerateHexToken() length = %d, want 32", len(token))
	}

	// Should only contain hex characters
	for _, c := range token {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("GenerateHexToken() contains non-hex character: %c", c)
		}
	}
}

func TestMustGenerateHexToken(t *testing.T) {
	token := MustGenerateHexToken(16)
	if len(token) != 32 {
		t.Errorf("MustGenerateHexToken() length = %d, want 32", len(token))
	}
}

func TestMustGenerateHexToken_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustGenerateHexToken() did not panic with invalid length")
		}
	}()
	MustGenerateHexToken(-1)
}

func TestGenerateWebhookSecret(t *testing.T) {
	secret, err := GenerateWebhookSecret()
	if err != nil {
		t.Fatalf("GenerateWebhookSecret() unexpected error: %v", err)
	}

	// Should be 64 characters (32 bytes * 2 for hex)
	if len(secret) != 64 {
		t.Errorf("GenerateWebhookSecret() length = %d, want 64", len(secret))
	}
}

func TestGenerateAPIKey(t *testing.T) {
	key := GenerateAPIKey()
	if key == "" {
		t.Error("GenerateAPIKey() returned empty string")
	}

	// Should be URL-safe
	if strings.ContainsAny(key, "+/") {
		t.Error("GenerateAPIKey() contains non-URL-safe characters")
	}
}

func BenchmarkGenerateSecureBytes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = GenerateSecureBytes(32)
	}
}

func BenchmarkGenerateHexToken(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = GenerateHexToken(32)
	}
}

func BenchmarkGenerateToken(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = GenerateToken(32)
	}
}
