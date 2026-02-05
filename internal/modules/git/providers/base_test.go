package providers

import (
	"testing"
)

func TestNewBaseGitProvider(t *testing.T) {
	config := &ProviderConfig{
		AppID:         "test-app-id",
		PrivateKey:    "test-key",
		WebhookSecret: "test-secret",
	}

	base := NewBaseGitProvider(
		config,
		WithProviderType(GitProviderGitHub),
		WithBaseURL("https://github.com"),
		WithAPIURL("https://api.github.com"),
	)

	if base == nil {
		t.Fatal("NewBaseGitProvider() returned nil")
	}

	if base.Config() != config {
		t.Error("Config should be set")
	}

	if base.GetType() != GitProviderGitHub {
		t.Errorf("GetType() = %v, want %v", base.GetType(), GitProviderGitHub)
	}

	if base.BaseURL() != "https://github.com" {
		t.Errorf("BaseURL() = %v, want https://github.com", base.BaseURL())
	}

	if base.APIURL() != "https://api.github.com" {
		t.Errorf("APIURL() = %v, want https://api.github.com", base.APIURL())
	}

	if base.APIClient() == nil {
		t.Error("APIClient() should not be nil")
	}
}

func TestBaseGitProvider_SetSourceControl(t *testing.T) {
	base := NewBaseGitProvider(&ProviderConfig{})
	sc := &SourceControlData{ID: "test-id"}

	base.SetSourceControl(sc)

	if base.SourceControl() != sc {
		t.Error("SourceControl() should return the set value")
	}
}

func TestBaseGitProvider_GetSSHURL(t *testing.T) {
	tests := []struct {
		name         string
		providerType GitProviderType
		repo         string
		expected     string
	}{
		{
			name:         "GitHub SSH URL",
			providerType: GitProviderGitHub,
			repo:         "owner/repo",
			expected:     "git@github.com:owner/repo.git",
		},
		{
			name:         "GitLab SSH URL",
			providerType: GitProviderGitLab,
			repo:         "owner/repo",
			expected:     "git@gitlab.com:owner/repo.git",
		},
		{
			name:         "Bitbucket SSH URL",
			providerType: GitProviderBitbucket,
			repo:         "owner/repo",
			expected:     "git@bitbucket.org:owner/repo.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := NewBaseGitProvider(
				&ProviderConfig{},
				WithProviderType(tt.providerType),
			)

			url := base.GetSSHURL(tt.repo)
			if url != tt.expected {
				t.Errorf("GetSSHURL() = %v, want %v", url, tt.expected)
			}
		})
	}
}

func TestBaseGitProvider_GetHTTPSURL(t *testing.T) {
	tests := []struct {
		name         string
		providerType GitProviderType
		repo         string
		expected     string
	}{
		{
			name:         "GitHub HTTPS URL",
			providerType: GitProviderGitHub,
			repo:         "owner/repo",
			expected:     "https://github.com/owner/repo.git",
		},
		{
			name:         "GitLab HTTPS URL",
			providerType: GitProviderGitLab,
			repo:         "owner/repo",
			expected:     "https://gitlab.com/owner/repo.git",
		},
		{
			name:         "Bitbucket HTTPS URL",
			providerType: GitProviderBitbucket,
			repo:         "owner/repo",
			expected:     "https://bitbucket.org/owner/repo.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := NewBaseGitProvider(
				&ProviderConfig{},
				WithProviderType(tt.providerType),
			)

			url := base.GetHTTPSURL(tt.repo)
			if url != tt.expected {
				t.Errorf("GetHTTPSURL() = %v, want %v", url, tt.expected)
			}
		})
	}
}

func TestBaseGitProvider_VerifyHMACSHA256Signature(t *testing.T) {
	secret := "test-secret"
	base := NewBaseGitProvider(&ProviderConfig{WebhookSecret: secret})

	payload := []byte(`{"action": "push"}`)

	tests := []struct {
		name      string
		payload   []byte
		signature string
		prefix    string
		expected  bool
	}{
		{
			name:      "Valid signature with sha256 prefix",
			payload:   payload,
			signature: "sha256=efb4c33143f4cb063d5b1556f1b965babcedf9feb3cd47e8b40966cbc54e7b30",
			prefix:    "sha256=",
			expected:  true,
		},
		{
			name:      "Invalid signature",
			payload:   payload,
			signature: "sha256=invalid",
			prefix:    "sha256=",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := base.VerifyHMACSHA256Signature(tt.payload, tt.signature, tt.prefix)
			if result != tt.expected {
				t.Errorf("VerifyHMACSHA256Signature() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestBaseGitProvider_VerifyHMACSHA256Signature_NoSecret(t *testing.T) {
	base := NewBaseGitProvider(&ProviderConfig{WebhookSecret: ""})

	result := base.VerifyHMACSHA256Signature([]byte("test"), "signature", "")
	if result {
		t.Error("VerifyHMACSHA256Signature() should return false when no secret is configured")
	}
}

func TestBaseGitProvider_VerifyTokenSignature(t *testing.T) {
	secret := "test-secret"
	base := NewBaseGitProvider(&ProviderConfig{WebhookSecret: secret})

	tests := []struct {
		name     string
		token    string
		expected bool
	}{
		{
			name:     "Valid token",
			token:    secret,
			expected: true,
		},
		{
			name:     "Invalid token",
			token:    "wrong-secret",
			expected: false,
		},
		{
			name:     "Empty token",
			token:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := base.VerifyTokenSignature(tt.token)
			if result != tt.expected {
				t.Errorf("VerifyTokenSignature() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestBaseGitProvider_VerifyTokenSignature_NoSecret(t *testing.T) {
	base := NewBaseGitProvider(&ProviderConfig{WebhookSecret: ""})

	result := base.VerifyTokenSignature("any-token")
	if result {
		t.Error("VerifyTokenSignature() should return false when no secret is configured")
	}
}

func TestBaseGitProvider_GetOAuthToken(t *testing.T) {
	tests := []struct {
		name          string
		sourceControl *SourceControlData
		expectError   bool
		expectedToken string
	}{
		{
			name:          "No source control",
			sourceControl: nil,
			expectError:   true,
		},
		{
			name: "No provider data",
			sourceControl: &SourceControlData{
				ID:           "test-id",
				ProviderData: nil,
			},
			expectError: true,
		},
		{
			name: "Empty access token",
			sourceControl: &SourceControlData{
				ID:           "test-id",
				ProviderData: map[string]interface{}{},
			},
			expectError: true,
		},
		{
			name: "Valid access token",
			sourceControl: &SourceControlData{
				ID: "test-id",
				ProviderData: map[string]interface{}{
					"access_token": "valid-token",
				},
			},
			expectError:   false,
			expectedToken: "valid-token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := NewBaseGitProvider(&ProviderConfig{})
			if tt.sourceControl != nil {
				base.SetSourceControl(tt.sourceControl)
			}

			token, err := base.GetOAuthToken()

			if tt.expectError {
				if err != ErrAuthenticationFailed {
					t.Errorf("Expected ErrAuthenticationFailed, got %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if token != tt.expectedToken {
					t.Errorf("GetOAuthToken() = %v, want %v", token, tt.expectedToken)
				}
			}
		})
	}
}

func TestParseLinkHeader(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{
			name:     "Empty header",
			header:   "",
			expected: "",
		},
		{
			name:     "Header with next link",
			header:   `<https://api.github.com/installations?page=2>; rel="next", <https://api.github.com/installations?page=5>; rel="last"`,
			expected: "/installations?page=2",
		},
		{
			name:     "Header without next link",
			header:   `<https://api.github.com/installations?page=1>; rel="first", <https://api.github.com/installations?page=5>; rel="last"`,
			expected: "",
		},
		{
			name:     "Only next link",
			header:   `<https://api.github.com/repos?page=2>; rel="next"`,
			expected: "/repos?page=2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseLinkHeader(tt.header)
			if result != tt.expected {
				t.Errorf("ParseLinkHeader() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestHasNextPage(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected bool
	}{
		{
			name:     "Empty header",
			header:   "",
			expected: false,
		},
		{
			name:     "Header with next link",
			header:   `<https://api.github.com/installations?page=2>; rel="next"`,
			expected: true,
		},
		{
			name:     "Header without next link",
			header:   `<https://api.github.com/installations?page=1>; rel="first"`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasNextPage(tt.header)
			if result != tt.expected {
				t.Errorf("HasNextPage() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtractFloatID(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]interface{}
		key      string
		expected string
	}{
		{
			name:     "Float64 ID",
			data:     map[string]interface{}{"id": float64(12345)},
			key:      "id",
			expected: "12345",
		},
		{
			name:     "String ID",
			data:     map[string]interface{}{"id": "12345"},
			key:      "id",
			expected: "12345",
		},
		{
			name:     "Missing key",
			data:     map[string]interface{}{},
			key:      "id",
			expected: "",
		},
		{
			name:     "Invalid type",
			data:     map[string]interface{}{"id": 12345}, // int, not float64 or string
			key:      "id",
			expected: "",
		},
		{
			name:     "Large float ID",
			data:     map[string]interface{}{"id": float64(123456789012)},
			key:      "id",
			expected: "123456789012",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractFloatID(tt.data, tt.key)
			if result != tt.expected {
				t.Errorf("ExtractFloatID() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDecodeJSONBytes(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		expectError bool
	}{
		{
			name:        "Empty data",
			data:        []byte{},
			expectError: false,
		},
		{
			name:        "Valid JSON",
			data:        []byte(`{"key": "value"}`),
			expectError: false,
		},
		{
			name:        "Invalid JSON",
			data:        []byte(`{invalid`),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result map[string]interface{}
			err := DecodeJSONBytes(tt.data, &result)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestEncodeJSON(t *testing.T) {
	data := map[string]interface{}{
		"key": "value",
		"num": 123,
	}

	result, err := EncodeJSON(data)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(result) == 0 {
		t.Error("Expected non-empty result")
	}
}
