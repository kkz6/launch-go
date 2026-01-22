package providers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

func TestNewProviderFactory(t *testing.T) {
	factory := NewProviderFactory()
	if factory == nil {
		t.Fatal("NewProviderFactory() returned nil")
	}
	if factory.configs == nil {
		t.Error("ProviderFactory.configs should be initialized")
	}
}

func TestProviderFactory_RegisterConfig(t *testing.T) {
	factory := NewProviderFactory()

	config := &ProviderConfig{
		AppID:         "test-app-id",
		PrivateKey:    "test-key",
		WebhookSecret: "test-secret",
	}

	factory.RegisterConfig(GitProviderGitHub, config)

	if _, ok := factory.configs[GitProviderGitHub]; !ok {
		t.Error("Config should be registered for GitHub")
	}
}

func TestProviderFactory_GetProvider(t *testing.T) {
	factory := NewProviderFactory()

	tests := []struct {
		name        string
		provider    GitProviderType
		config      *ProviderConfig
		expectError bool
	}{
		{
			name:     "GitHub provider",
			provider: GitProviderGitHub,
			config: &ProviderConfig{
				AppID:         "test-app-id",
				PrivateKey:    "test-key",
				WebhookSecret: "test-secret",
				AppSlug:       "test-app",
			},
			expectError: false,
		},
		{
			name:     "GitLab provider",
			provider: GitProviderGitLab,
			config: &ProviderConfig{
				ClientID:      "test-client-id",
				ClientSecret:  "test-secret",
				WebhookSecret: "test-webhook-secret",
			},
			expectError: false,
		},
		{
			name:     "Bitbucket provider",
			provider: GitProviderBitbucket,
			config: &ProviderConfig{
				ClientID:      "test-client-id",
				ClientSecret:  "test-secret",
				WebhookSecret: "test-webhook-secret",
			},
			expectError: false,
		},
		{
			name:        "Unconfigured provider",
			provider:    GitProviderType("unknown"),
			config:      nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.config != nil {
				factory.RegisterConfig(tt.provider, tt.config)
			}

			provider, err := factory.GetProvider(tt.provider)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if err != ErrProviderNotConfigured {
					t.Errorf("Expected ErrProviderNotConfigured, got %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if provider == nil {
					t.Error("Expected provider but got nil")
				}
				if provider.GetType() != tt.provider {
					t.Errorf("Provider type = %v, want %v", provider.GetType(), tt.provider)
				}
			}
		})
	}
}

func TestProviderFactory_GetProviderWithInstallation(t *testing.T) {
	factory := NewProviderFactory()
	factory.RegisterConfig(GitProviderGitHub, &ProviderConfig{
		AppID:         "test-app-id",
		PrivateKey:    "test-key",
		WebhookSecret: "test-secret",
		AppSlug:       "test-app",
	})

	installationID := "12345"
	sc := &SourceControlData{
		ID:             "sc-123",
		Provider:       GitProviderGitHub,
		InstallationID: &installationID,
	}

	provider, err := factory.GetProviderWithInstallation(GitProviderGitHub, sc)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if provider == nil {
		t.Fatal("Expected provider but got nil")
	}

	// Verify that SetSourceControl was called
	ghProvider, ok := provider.(*GitHubProvider)
	if !ok {
		t.Fatal("Expected GitHubProvider")
	}
	if ghProvider.SourceControl() != sc {
		t.Error("Source control should be set on provider")
	}
}

func TestProviderFactory_GetProviderWithInstallation_NotConfigured(t *testing.T) {
	factory := NewProviderFactory()

	_, err := factory.GetProviderWithInstallation(GitProviderGitHub, nil)
	if err != ErrProviderNotConfigured {
		t.Errorf("Expected ErrProviderNotConfigured, got %v", err)
	}
}

// GitHub Provider Tests

func TestNewGitHubProvider(t *testing.T) {
	config := &ProviderConfig{
		AppID:         "test-app-id",
		PrivateKey:    "test-key",
		WebhookSecret: "test-secret",
		AppSlug:       "test-app",
	}

	provider := NewGitHubProvider(config)
	if provider == nil {
		t.Fatal("NewGitHubProvider() returned nil")
	}
	if provider.Config() != config {
		t.Error("Config should be set")
	}
	if provider.APIClient() == nil {
		t.Error("HTTP client should be initialized")
	}
}

func TestGitHubProvider_GetType(t *testing.T) {
	provider := NewGitHubProvider(&ProviderConfig{})
	if provider.GetType() != GitProviderGitHub {
		t.Errorf("GetType() = %v, want %v", provider.GetType(), GitProviderGitHub)
	}
}

func TestGitHubProvider_SetSourceControl(t *testing.T) {
	provider := NewGitHubProvider(&ProviderConfig{})
	sc := &SourceControlData{ID: "test-id"}

	provider.SetSourceControl(sc)

	if provider.SourceControl() != sc {
		t.Error("SetSourceControl() should set source control")
	}
}

func TestGitHubProvider_GetInstallationURL(t *testing.T) {
	tests := []struct {
		name        string
		appSlug     string
		expectError bool
		expected    string
	}{
		{
			name:        "With app slug",
			appSlug:     "my-app",
			expectError: false,
			expected:    "https://github.com/apps/my-app/installations/new",
		},
		{
			name:        "Without app slug",
			appSlug:     "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewGitHubProvider(&ProviderConfig{AppSlug: tt.appSlug})

			url, err := provider.GetInstallationURL()

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if url != tt.expected {
					t.Errorf("GetInstallationURL() = %v, want %v", url, tt.expected)
				}
			}
		})
	}
}

func TestGitHubProvider_ValidateWebhook(t *testing.T) {
	secret := "test-webhook-secret"
	provider := NewGitHubProvider(&ProviderConfig{WebhookSecret: secret})

	payload := []byte(`{"action": "created"}`)

	// Generate valid signature
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	validSignature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	tests := []struct {
		name      string
		payload   []byte
		signature string
		expected  bool
	}{
		{
			name:      "Valid signature",
			payload:   payload,
			signature: validSignature,
			expected:  true,
		},
		{
			name:      "Invalid signature",
			payload:   payload,
			signature: "sha256=invalid",
			expected:  false,
		},
		{
			name:      "Empty signature",
			payload:   payload,
			signature: "",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.ValidateWebhook(tt.payload, tt.signature)
			if result != tt.expected {
				t.Errorf("ValidateWebhook() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGitHubProvider_ValidateWebhook_NoSecret(t *testing.T) {
	provider := NewGitHubProvider(&ProviderConfig{WebhookSecret: ""})

	result := provider.ValidateWebhook([]byte("test"), "signature")
	if result {
		t.Error("ValidateWebhook() should return false when no secret is configured")
	}
}

func TestGitHubProvider_GetSSHURL(t *testing.T) {
	provider := NewGitHubProvider(&ProviderConfig{})

	url := provider.GetSSHURL("owner/repo")
	expected := "git@github.com:owner/repo.git"

	if url != expected {
		t.Errorf("GetSSHURL() = %v, want %v", url, expected)
	}
}

func TestGitHubProvider_GetHTTPSURL(t *testing.T) {
	provider := NewGitHubProvider(&ProviderConfig{})

	url := provider.GetHTTPSURL("owner/repo")
	expected := "https://github.com/owner/repo.git"

	if url != expected {
		t.Errorf("GetHTTPSURL() = %v, want %v", url, expected)
	}
}

func TestGitHubProvider_GetCommitData(t *testing.T) {
	provider := NewGitHubProvider(&ProviderConfig{})

	payload := map[string]interface{}{
		"head_commit": map[string]interface{}{
			"id":      "abc123def456789",
			"message": "Test commit",
			"url":     "https://github.com/owner/repo/commit/abc123",
			"author": map[string]interface{}{
				"name":  "testuser",
				"email": "test@example.com",
			},
		},
		"ref": "refs/heads/main",
	}

	commitData := provider.GetCommitData(payload)

	if commitData == nil {
		t.Fatal("GetCommitData() returned nil")
	}
	if commitData.CommitID != "abc123def456789" {
		t.Errorf("CommitID = %v, want abc123def456789", commitData.CommitID)
	}
	if commitData.SHA != "abc123def456789" {
		t.Errorf("SHA = %v, want abc123def456789", commitData.SHA)
	}
	if commitData.Name != "testuser" {
		t.Errorf("Name = %v, want testuser", commitData.Name)
	}
	if commitData.Branch != "main" {
		t.Errorf("Branch = %v, want main", commitData.Branch)
	}
}

func TestGitHubProvider_DeployKey_NoSourceControl(t *testing.T) {
	provider := NewGitHubProvider(&ProviderConfig{})

	err := provider.DeployKey(context.Background(), "sc-id", "title", "repo", "key")
	if err == nil {
		t.Error("DeployKey() should error when no source control is set")
	}
}

func TestGitHubProvider_GetLastCommit_NoSourceControl(t *testing.T) {
	provider := NewGitHubProvider(&ProviderConfig{})

	_, err := provider.GetLastCommit(context.Background(), "sc-id", "repo", "main")
	if err == nil {
		t.Error("GetLastCommit() should error when no source control is set")
	}
}

func TestGitHubProvider_mapInstallationData(t *testing.T) {
	provider := NewGitHubProvider(&ProviderConfig{})

	data := map[string]interface{}{
		"id": float64(12345),
		"account": map[string]interface{}{
			"id":         float64(67890),
			"login":      "testorg",
			"type":       "Organization",
			"avatar_url": "https://example.com/avatar.png",
		},
		"permissions": map[string]interface{}{
			"contents": "read",
		},
		"repository_selection": "selected",
		"html_url":             "https://github.com/settings/installations/12345",
		"created_at":           "2024-01-01T00:00:00Z",
		"updated_at":           "2024-01-02T00:00:00Z",
		"target_type":          "Organization",
		"app_slug":             "test-app",
		"app_id":               float64(1234),
		"events":               []interface{}{"push", "pull_request"},
	}

	result := provider.mapInstallationData(data)

	if result.ID != "12345" {
		t.Errorf("ID = %v, want 12345", result.ID)
	}
	if result.AccountID != "67890" {
		t.Errorf("AccountID = %v, want 67890", result.AccountID)
	}
	if result.AccountLogin != "testorg" {
		t.Errorf("AccountLogin = %v, want testorg", result.AccountLogin)
	}
	if result.AccountType != "Organization" {
		t.Errorf("AccountType = %v, want Organization", result.AccountType)
	}
	if result.RepositorySelection != "selected" {
		t.Errorf("RepositorySelection = %v, want selected", result.RepositorySelection)
	}
	if !result.HasMultipleRepositories {
		t.Error("HasMultipleRepositories should be true for 'selected'")
	}
	if len(result.Events) != 2 {
		t.Errorf("Events length = %d, want 2", len(result.Events))
	}
	if result.AppID != 1234 {
		t.Errorf("AppID = %v, want 1234", result.AppID)
	}
}

// GitLab Provider Tests

func TestNewGitLabProvider(t *testing.T) {
	config := &ProviderConfig{
		ClientID:      "test-client-id",
		ClientSecret:  "test-secret",
		WebhookSecret: "test-webhook-secret",
	}

	provider := NewGitLabProvider(config)
	if provider == nil {
		t.Fatal("NewGitLabProvider() returned nil")
	}
	if provider.APIClient() == nil {
		t.Error("HTTP client should be initialized")
	}
}

func TestGitLabProvider_GetType(t *testing.T) {
	provider := NewGitLabProvider(&ProviderConfig{})
	if provider.GetType() != GitProviderGitLab {
		t.Errorf("GetType() = %v, want %v", provider.GetType(), GitProviderGitLab)
	}
}

func TestGitLabProvider_SetSourceControl(t *testing.T) {
	provider := NewGitLabProvider(&ProviderConfig{})
	sc := &SourceControlData{ID: "test-id"}

	provider.SetSourceControl(sc)

	if provider.SourceControl() != sc {
		t.Error("SetSourceControl() should set source control")
	}
}

func TestGitLabProvider_GetInstallationURL(t *testing.T) {
	tests := []struct {
		name        string
		clientID    string
		expectError bool
	}{
		{
			name:        "With client ID",
			clientID:    "my-client-id",
			expectError: false,
		},
		{
			name:        "Without client ID",
			clientID:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewGitLabProvider(&ProviderConfig{ClientID: tt.clientID})

			url, err := provider.GetInstallationURL()

			if tt.expectError {
				if err != ErrProviderNotConfigured {
					t.Errorf("Expected ErrProviderNotConfigured, got %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if url == "" {
					t.Error("URL should not be empty")
				}
			}
		})
	}
}

func TestGitLabProvider_ValidateWebhook(t *testing.T) {
	secret := "test-webhook-secret"
	provider := NewGitLabProvider(&ProviderConfig{WebhookSecret: secret})

	tests := []struct {
		name      string
		signature string
		expected  bool
	}{
		{
			name:      "Valid token",
			signature: secret,
			expected:  true,
		},
		{
			name:      "Invalid token",
			signature: "invalid",
			expected:  false,
		},
		{
			name:      "Empty token",
			signature: "",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.ValidateWebhook([]byte("test"), tt.signature)
			if result != tt.expected {
				t.Errorf("ValidateWebhook() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGitLabProvider_ValidateWebhook_NoSecret(t *testing.T) {
	provider := NewGitLabProvider(&ProviderConfig{WebhookSecret: ""})

	result := provider.ValidateWebhook([]byte("test"), "signature")
	if result {
		t.Error("ValidateWebhook() should return false when no secret is configured")
	}
}

func TestGitLabProvider_GetSSHURL(t *testing.T) {
	provider := NewGitLabProvider(&ProviderConfig{})

	url := provider.GetSSHURL("owner/repo")
	expected := "git@gitlab.com:owner/repo.git"

	if url != expected {
		t.Errorf("GetSSHURL() = %v, want %v", url, expected)
	}
}

func TestGitLabProvider_GetHTTPSURL(t *testing.T) {
	provider := NewGitLabProvider(&ProviderConfig{})

	url := provider.GetHTTPSURL("owner/repo")
	expected := "https://gitlab.com/owner/repo.git"

	if url != expected {
		t.Errorf("GetHTTPSURL() = %v, want %v", url, expected)
	}
}

func TestGitLabProvider_GetCommitData(t *testing.T) {
	provider := NewGitLabProvider(&ProviderConfig{})

	payload := map[string]interface{}{
		"checkout_sha": "abc123def456789",
		"user_name":    "testuser",
		"user_email":   "test@example.com",
		"commits": []interface{}{
			map[string]interface{}{
				"id":    "abc123def456789",
				"title": "Test commit",
				"url":   "https://gitlab.com/owner/repo/commit/abc123",
				"author": map[string]interface{}{
					"name":  "testuser",
					"email": "test@example.com",
				},
			},
		},
		"ref": "refs/heads/main",
	}

	commitData := provider.GetCommitData(payload)

	if commitData == nil {
		t.Fatal("GetCommitData() returned nil")
	}
	if commitData.CommitID != "abc123def456789" {
		t.Errorf("CommitID = %v, want abc123def456789", commitData.CommitID)
	}
	if commitData.Name != "testuser" {
		t.Errorf("Name = %v, want testuser", commitData.Name)
	}
}

func TestGitLabProvider_GetInstallation(t *testing.T) {
	provider := NewGitLabProvider(&ProviderConfig{})

	installation, err := provider.GetInstallation(context.Background(), "12345")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if installation == nil {
		t.Fatal("Installation should not be nil")
	}
	if installation.ID != "12345" {
		t.Errorf("ID = %v, want 12345", installation.ID)
	}
}

func TestGitLabProvider_GetAllInstallations(t *testing.T) {
	provider := NewGitLabProvider(&ProviderConfig{})

	installations, err := provider.GetAllInstallations(context.Background())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if installations == nil {
		t.Error("Installations should not be nil")
	}
}

func TestGitLabProvider_GetInstallationRepositories(t *testing.T) {
	provider := NewGitLabProvider(&ProviderConfig{})

	repos, err := provider.GetInstallationRepositories(context.Background(), "12345")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if repos == nil {
		t.Error("Repos should not be nil")
	}
}

func TestGitLabProvider_DeployKey(t *testing.T) {
	provider := NewGitLabProvider(&ProviderConfig{})

	err := provider.DeployKey(context.Background(), "sc-id", "title", "repo", "key")
	if err != ErrProviderNotConfigured {
		t.Errorf("Expected ErrProviderNotConfigured, got %v", err)
	}
}

func TestGitLabProvider_GetLastCommit(t *testing.T) {
	provider := NewGitLabProvider(&ProviderConfig{})

	_, err := provider.GetLastCommit(context.Background(), "sc-id", "repo", "main")
	if err != ErrProviderNotConfigured {
		t.Errorf("Expected ErrProviderNotConfigured, got %v", err)
	}
}

func TestGitLabProvider_GetRepository(t *testing.T) {
	provider := NewGitLabProvider(&ProviderConfig{})

	_, err := provider.GetRepository(context.Background(), "12345", "owner", "repo")
	if err != ErrProviderNotConfigured {
		t.Errorf("Expected ErrProviderNotConfigured, got %v", err)
	}
}

func TestParseGitLabRepository(t *testing.T) {
	project := map[string]interface{}{
		"id":                  float64(12345),
		"path_with_namespace": "owner/repo",
		"name":                "repo",
		"web_url":             "https://gitlab.com/owner/repo",
		"ssh_url_to_repo":     "git@gitlab.com:owner/repo.git",
		"default_branch":      "main",
		"visibility":          "public",
		"description":         "Test repo",
	}

	result := parseGitLabRepository(project)

	if result["full_name"] != "owner/repo" {
		t.Errorf("full_name = %v, want owner/repo", result["full_name"])
	}
	if result["name"] != "repo" {
		t.Errorf("name = %v, want repo", result["name"])
	}
	if result["html_url"] != "https://gitlab.com/owner/repo" {
		t.Errorf("html_url = %v, want https://gitlab.com/owner/repo", result["html_url"])
	}
	if result["private"].(bool) {
		t.Error("private should be false for public visibility")
	}
}

func TestParseGitLabRepository_Private(t *testing.T) {
	project := map[string]interface{}{
		"id":         float64(12345),
		"visibility": "private",
	}

	result := parseGitLabRepository(project)

	if !result["private"].(bool) {
		t.Error("private should be true for private visibility")
	}
}

// Bitbucket Provider Tests

func TestNewBitbucketProvider(t *testing.T) {
	config := &ProviderConfig{
		ClientID:      "test-client-id",
		ClientSecret:  "test-secret",
		WebhookSecret: "test-webhook-secret",
	}

	provider := NewBitbucketProvider(config)
	if provider == nil {
		t.Fatal("NewBitbucketProvider() returned nil")
	}
	if provider.APIClient() == nil {
		t.Error("HTTP client should be initialized")
	}
}

func TestBitbucketProvider_GetType(t *testing.T) {
	provider := NewBitbucketProvider(&ProviderConfig{})
	if provider.GetType() != GitProviderBitbucket {
		t.Errorf("GetType() = %v, want %v", provider.GetType(), GitProviderBitbucket)
	}
}

func TestBitbucketProvider_SetSourceControl(t *testing.T) {
	provider := NewBitbucketProvider(&ProviderConfig{})
	sc := &SourceControlData{ID: "test-id"}

	provider.SetSourceControl(sc)

	if provider.SourceControl() != sc {
		t.Error("SetSourceControl() should set source control")
	}
}

func TestBitbucketProvider_GetInstallationURL(t *testing.T) {
	tests := []struct {
		name        string
		clientID    string
		expectError bool
	}{
		{
			name:        "With client ID",
			clientID:    "my-client-id",
			expectError: false,
		},
		{
			name:        "Without client ID",
			clientID:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewBitbucketProvider(&ProviderConfig{ClientID: tt.clientID})

			url, err := provider.GetInstallationURL()

			if tt.expectError {
				if err != ErrProviderNotConfigured {
					t.Errorf("Expected ErrProviderNotConfigured, got %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if url == "" {
					t.Error("URL should not be empty")
				}
			}
		})
	}
}

func TestBitbucketProvider_ValidateWebhook(t *testing.T) {
	secret := "test-webhook-secret"
	provider := NewBitbucketProvider(&ProviderConfig{WebhookSecret: secret})

	payload := []byte(`{"push": {"changes": []}}`)

	// Generate valid signature (Bitbucket uses HMAC)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	validSignature := hex.EncodeToString(mac.Sum(nil))

	tests := []struct {
		name      string
		payload   []byte
		signature string
		expected  bool
	}{
		{
			name:      "Valid signature",
			payload:   payload,
			signature: validSignature,
			expected:  true,
		},
		{
			name:      "Invalid signature",
			payload:   payload,
			signature: "invalid",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.ValidateWebhook(tt.payload, tt.signature)
			if result != tt.expected {
				t.Errorf("ValidateWebhook() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestBitbucketProvider_ValidateWebhook_NoSecret(t *testing.T) {
	provider := NewBitbucketProvider(&ProviderConfig{WebhookSecret: ""})

	result := provider.ValidateWebhook([]byte("test"), "signature")
	if result {
		t.Error("ValidateWebhook() should return false when no secret is configured")
	}
}

func TestBitbucketProvider_GetSSHURL(t *testing.T) {
	provider := NewBitbucketProvider(&ProviderConfig{})

	url := provider.GetSSHURL("owner/repo")
	expected := "git@bitbucket.org:owner/repo.git"

	if url != expected {
		t.Errorf("GetSSHURL() = %v, want %v", url, expected)
	}
}

func TestBitbucketProvider_GetHTTPSURL(t *testing.T) {
	provider := NewBitbucketProvider(&ProviderConfig{})

	url := provider.GetHTTPSURL("owner/repo")
	expected := "https://bitbucket.org/owner/repo.git"

	if url != expected {
		t.Errorf("GetHTTPSURL() = %v, want %v", url, expected)
	}
}

func TestBitbucketProvider_GetCommitData(t *testing.T) {
	provider := NewBitbucketProvider(&ProviderConfig{})

	// Bitbucket payload structure: push.changes[0].commits[0].hash
	payload := map[string]interface{}{
		"push": map[string]interface{}{
			"changes": []interface{}{
				map[string]interface{}{
					"commits": []interface{}{
						map[string]interface{}{
							"hash":    "abc123def456789",
							"message": "Test commit",
							"links": map[string]interface{}{
								"html": map[string]interface{}{
									"href": "https://bitbucket.org/owner/repo/commits/abc123",
								},
							},
							"author": map[string]interface{}{
								"raw": "testuser <test@example.com>",
							},
						},
					},
					"new": map[string]interface{}{
						"name": "main",
					},
				},
			},
		},
	}

	commitData := provider.GetCommitData(payload)

	if commitData == nil {
		t.Fatal("GetCommitData() returned nil")
	}
	if commitData.CommitID != "abc123def456789" {
		t.Errorf("CommitID = %v, want abc123def456789", commitData.CommitID)
	}
	if commitData.SHA != "abc123def456789" {
		t.Errorf("SHA = %v, want abc123def456789", commitData.SHA)
	}
	if commitData.Name != "testuser" {
		t.Errorf("Name = %v, want testuser", commitData.Name)
	}
	if commitData.Email != "test@example.com" {
		t.Errorf("Email = %v, want test@example.com", commitData.Email)
	}
	if commitData.Branch != "main" {
		t.Errorf("Branch = %v, want main", commitData.Branch)
	}
}

func TestBitbucketProvider_GetInstallation(t *testing.T) {
	provider := NewBitbucketProvider(&ProviderConfig{})

	installation, err := provider.GetInstallation(context.Background(), "12345")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if installation == nil {
		t.Fatal("Installation should not be nil")
	}
	if installation.ID != "12345" {
		t.Errorf("ID = %v, want 12345", installation.ID)
	}
}

func TestBitbucketProvider_GetAllInstallations(t *testing.T) {
	provider := NewBitbucketProvider(&ProviderConfig{})

	installations, err := provider.GetAllInstallations(context.Background())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if installations == nil {
		t.Error("Installations should not be nil")
	}
}

func TestBitbucketProvider_GetInstallationRepositories(t *testing.T) {
	provider := NewBitbucketProvider(&ProviderConfig{})

	repos, err := provider.GetInstallationRepositories(context.Background(), "12345")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if repos == nil {
		t.Error("Repos should not be nil")
	}
}

func TestBitbucketProvider_DeployKey(t *testing.T) {
	provider := NewBitbucketProvider(&ProviderConfig{})

	err := provider.DeployKey(context.Background(), "sc-id", "title", "repo", "key")
	if err != ErrProviderNotConfigured {
		t.Errorf("Expected ErrProviderNotConfigured, got %v", err)
	}
}

func TestBitbucketProvider_GetLastCommit(t *testing.T) {
	provider := NewBitbucketProvider(&ProviderConfig{})

	_, err := provider.GetLastCommit(context.Background(), "sc-id", "repo", "main")
	if err != ErrProviderNotConfigured {
		t.Errorf("Expected ErrProviderNotConfigured, got %v", err)
	}
}

func TestBitbucketProvider_GetRepository(t *testing.T) {
	provider := NewBitbucketProvider(&ProviderConfig{})

	_, err := provider.GetRepository(context.Background(), "12345", "owner", "repo")
	if err != ErrProviderNotConfigured {
		t.Errorf("Expected ErrProviderNotConfigured, got %v", err)
	}
}

func TestParseBitbucketRepository(t *testing.T) {
	repo := map[string]interface{}{
		"uuid":       "{12345}",
		"full_name":  "owner/repo",
		"name":       "repo",
		"is_private": true,
		"links": map[string]interface{}{
			"html": map[string]interface{}{
				"href": "https://bitbucket.org/owner/repo",
			},
			"clone": []interface{}{
				map[string]interface{}{
					"name": "ssh",
					"href": "git@bitbucket.org:owner/repo.git",
				},
			},
		},
		"mainbranch": map[string]interface{}{
			"name": "master",
		},
		"description": "Test repo",
	}

	result := parseBitbucketRepository(repo)

	if result["full_name"] != "owner/repo" {
		t.Errorf("full_name = %v, want owner/repo", result["full_name"])
	}
	if result["name"] != "repo" {
		t.Errorf("name = %v, want repo", result["name"])
	}
	if result["html_url"] != "https://bitbucket.org/owner/repo" {
		t.Errorf("html_url = %v, want https://bitbucket.org/owner/repo", result["html_url"])
	}
	if result["ssh_url"] != "git@bitbucket.org:owner/repo.git" {
		t.Errorf("ssh_url = %v, want git@bitbucket.org:owner/repo.git", result["ssh_url"])
	}
	if !result["private"].(bool) {
		t.Error("private should be true")
	}
	if result["default_branch"] != "master" {
		t.Errorf("default_branch = %v, want master", result["default_branch"])
	}
}

func TestParseBitbucketRepository_DefaultBranch(t *testing.T) {
	repo := map[string]interface{}{
		"uuid":      "{12345}",
		"full_name": "owner/repo",
		"name":      "repo",
	}

	result := parseBitbucketRepository(repo)

	if result["default_branch"] != "main" {
		t.Errorf("default_branch = %v, want main (default)", result["default_branch"])
	}
}

// Error constants tests

func TestErrorConstants(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		errStr string
	}{
		{"ErrInvalidSignature", ErrInvalidSignature, "invalid webhook signature"},
		{"ErrInvalidPayload", ErrInvalidPayload, "invalid webhook payload"},
		{"ErrProviderNotConfigured", ErrProviderNotConfigured, "provider not configured"},
		{"ErrInstallationNotFound", ErrInstallationNotFound, "installation not found"},
		{"ErrRepositoryNotFound", fiberutil.NotFound(), "Resource not found"},
		{"ErrPermissionDenied", ErrPermissionDenied, "permission denied"},
		{"ErrAuthenticationFailed", ErrAuthenticationFailed, "authentication failed"},
		{"ErrRateLimitExceeded", ErrRateLimitExceeded, "rate limit exceeded"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.errStr {
				t.Errorf("%s.Error() = %v, want %v", tt.name, tt.err.Error(), tt.errStr)
			}
		})
	}
}

// Types tests

func TestGitProviderType_String(t *testing.T) {
	tests := []struct {
		name     string
		provider GitProviderType
		expected string
	}{
		{"GitHub", GitProviderGitHub, "github"},
		{"GitLab", GitProviderGitLab, "gitlab"},
		{"Bitbucket", GitProviderBitbucket, "bitbucket"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.provider.String(); got != tt.expected {
				t.Errorf("GitProviderType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGitProviderType_Label(t *testing.T) {
	tests := []struct {
		name     string
		provider GitProviderType
		expected string
	}{
		{"GitHub", GitProviderGitHub, "GitHub"},
		{"GitLab", GitProviderGitLab, "GitLab"},
		{"Bitbucket", GitProviderBitbucket, "Bitbucket"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.provider.Label(); got != tt.expected {
				t.Errorf("GitProviderType.Label() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGitProviderType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		provider GitProviderType
		expected bool
	}{
		{"GitHub is valid", GitProviderGitHub, true},
		{"GitLab is valid", GitProviderGitLab, true},
		{"Bitbucket is valid", GitProviderBitbucket, true},
		{"Unknown is invalid", GitProviderType("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.provider.IsValid(); got != tt.expected {
				t.Errorf("GitProviderType.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAppInstallationData_IsOrganization(t *testing.T) {
	tests := []struct {
		name        string
		accountType string
		expected    bool
	}{
		{"Organization lowercase", "organization", true},
		{"Organization uppercase", "Organization", true},
		{"User", "user", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &AppInstallationData{AccountType: tt.accountType}
			if got := a.IsOrganization(); got != tt.expected {
				t.Errorf("IsOrganization() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAppInstallationData_IsUser(t *testing.T) {
	tests := []struct {
		name        string
		accountType string
		expected    bool
	}{
		{"User lowercase", "user", true},
		{"User uppercase", "User", true},
		{"Organization", "organization", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &AppInstallationData{AccountType: tt.accountType}
			if got := a.IsUser(); got != tt.expected {
				t.Errorf("IsUser() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAppInstallationData_IsSuspended(t *testing.T) {
	tests := []struct {
		name        string
		suspendedAt string
		expected    bool
	}{
		{"Not suspended", "", false},
		{"Suspended", "2024-01-01T00:00:00Z", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &AppInstallationData{SuspendedAt: tt.suspendedAt}
			if got := a.IsSuspended(); got != tt.expected {
				t.Errorf("IsSuspended() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAppInstallationData_ToMap(t *testing.T) {
	a := &AppInstallationData{
		ID:           "123",
		AccountID:    "456",
		AccountLogin: "testuser",
		AccountType:  "User",
	}

	result := a.ToMap()

	if result["id"] != "123" {
		t.Errorf("ToMap() id = %v, want 123", result["id"])
	}
	if result["account_id"] != "456" {
		t.Errorf("ToMap() account_id = %v, want 456", result["account_id"])
	}
}

func TestCommitData_ToMap(t *testing.T) {
	c := &CommitData{
		CommitID: "abc123def456",
		SHA:      "abc123def456",
		Name:     "Test User",
		Email:    "test@example.com",
		Message:  "Test commit",
		URL:      "https://example.com/commit/abc123",
		Branch:   "main",
	}

	result := c.ToMap()

	if result["sha"] != "abc123def456" {
		t.Errorf("ToMap() sha = %v, want abc123def456", result["sha"])
	}
	if result["name"] != "Test User" {
		t.Errorf("ToMap() name = %v, want Test User", result["name"])
	}
	if result["email"] != "test@example.com" {
		t.Errorf("ToMap() email = %v, want test@example.com", result["email"])
	}
	if result["message"] != "Test commit" {
		t.Errorf("ToMap() message = %v, want Test commit", result["message"])
	}
	if result["url"] != "https://example.com/commit/abc123" {
		t.Errorf("ToMap() url = %v, want https://example.com/commit/abc123", result["url"])
	}
}

func TestSourceControlData_GetLogin(t *testing.T) {
	login := "testuser"
	tests := []struct {
		name     string
		login    *string
		expected string
	}{
		{"With login", &login, "testuser"},
		{"Nil login", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SourceControlData{Login: tt.login}
			if got := s.GetLogin(); got != tt.expected {
				t.Errorf("GetLogin() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSourceControlData_GetInstallationID(t *testing.T) {
	installID := "12345"
	tests := []struct {
		name           string
		installationID *string
		expected       string
	}{
		{"With installation ID", &installID, "12345"},
		{"Nil installation ID", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SourceControlData{InstallationID: tt.installationID}
			if got := s.GetInstallationID(); got != tt.expected {
				t.Errorf("GetInstallationID() = %v, want %v", got, tt.expected)
			}
		})
	}
}
