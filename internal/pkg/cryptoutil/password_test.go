package cryptoutil

import (
	"testing"
)

func TestHashPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{
			name:     "valid password",
			password: "securePassword123!",
			wantErr:  false,
		},
		{
			name:     "empty password",
			password: "",
			wantErr:  false,
		},
		{
			name:     "long password",
			password: "a" + string(make([]byte, 71)), // 72 bytes is bcrypt max
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashPassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("HashPassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && hash == "" {
				t.Error("HashPassword() returned empty hash")
			}
			if !tt.wantErr && hash == tt.password {
				t.Error("HashPassword() returned plaintext password")
			}
		})
	}
}

func TestVerifyPassword(t *testing.T) {
	password := "testPassword123!"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	tests := []struct {
		name     string
		hash     string
		password string
		want     bool
	}{
		{
			name:     "correct password",
			hash:     hash,
			password: password,
			want:     true,
		},
		{
			name:     "incorrect password",
			hash:     hash,
			password: "wrongPassword",
			want:     false,
		},
		{
			name:     "empty password against valid hash",
			hash:     hash,
			password: "",
			want:     false,
		},
		{
			name:     "invalid hash format",
			hash:     "not-a-valid-hash",
			password: password,
			want:     false,
		},
		{
			name:     "empty hash",
			hash:     "",
			password: password,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := VerifyPassword(tt.hash, tt.password); got != tt.want {
				t.Errorf("VerifyPassword() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHashPassword_Uniqueness(t *testing.T) {
	password := "samePassword"

	hash1, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	hash2, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	if hash1 == hash2 {
		t.Error("HashPassword() should generate unique hashes for the same password (due to salt)")
	}

	// Both should still verify correctly
	if !VerifyPassword(hash1, password) {
		t.Error("First hash should verify correctly")
	}
	if !VerifyPassword(hash2, password) {
		t.Error("Second hash should verify correctly")
	}
}
