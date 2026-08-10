package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestS3CredentialsApplyLegacyDefaults(t *testing.T) {
	tests := []struct {
		name        string
		endpoint    string
		credentials map[string]any
		initial     bool
		expected    bool
	}{
		{
			name:        "legacy compatible endpoint defaults to path style",
			endpoint:    "https://eu2.contabostorage.com",
			credentials: map[string]any{},
			expected:    true,
		},
		{
			name:        "explicit false is preserved for compatible endpoint",
			endpoint:    "https://eu2.contabostorage.com",
			credentials: map[string]any{"force_path_style": false},
			expected:    false,
		},
		{
			name:        "explicit true is preserved for AWS endpoint",
			endpoint:    "https://s3.eu-west-1.amazonaws.com",
			credentials: map[string]any{"force_path_style": true},
			initial:     true,
			expected:    true,
		},
		{
			name:        "legacy AWS endpoint keeps virtual host style",
			endpoint:    "s3.eu-west-1.amazonaws.com",
			credentials: map[string]any{},
			expected:    false,
		},
		{
			name:        "empty endpoint keeps AWS SDK default",
			credentials: map[string]any{},
			expected:    false,
		},
		{
			name:        "malformed endpoint defaults to path style",
			endpoint:    "https://%",
			credentials: map[string]any{},
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			credentials := S3Credentials{
				Endpoint:       tt.endpoint,
				ForcePathStyle: tt.initial,
			}

			credentials.ApplyLegacyDefaults(tt.credentials)

			require.Equal(t, tt.expected, credentials.ForcePathStyle)
		})
	}
}
