package git

import (
	"testing"
	"time"
)

func TestToSourceControlResponse(t *testing.T) {
	now := time.Now()
	connectedAt := now.Add(-1 * time.Hour)
	lastSyncedAt := now.Add(-30 * time.Minute)

	sc := &SourceControl{
		ID:                      "test-id-123",
		Provider:                GitProviderGitHub,
		Login:                   strPtr("testuser"),
		Name:                    strPtr("Test User"),
		Type:                    strPtr("user"),
		AvatarURL:               strPtr("https://example.com/avatar.png"),
		HTMLURL:                 strPtr("https://github.com/testuser"),
		InstallationID:          strPtr("12345"),
		RepositorySelection:     strPtr("selected"),
		HasMultipleRepositories: true,
		RepositoryCount:         5,
		ConnectedAt:             &connectedAt,
		LastSyncedAt:            &lastSyncedAt,
		CreatedAt:               now,
		Repositories: []SourceControlRepository{
			{
				ID:       "repo-1",
				Name:     "test-repo",
				FullName: "testuser/test-repo",
				Public:   true,
			},
		},
	}

	resp := ToSourceControlResponse(sc)

	if resp.ID != "test-id-123" {
		t.Errorf("ID = %v, want test-id-123", resp.ID)
	}
	if resp.Provider != "github" {
		t.Errorf("Provider = %v, want github", resp.Provider)
	}
	if resp.ProviderLabel != "GitHub" {
		t.Errorf("ProviderLabel = %v, want GitHub", resp.ProviderLabel)
	}
	if resp.Login != "testuser" {
		t.Errorf("Login = %v, want testuser", resp.Login)
	}
	if resp.Name != "Test User" {
		t.Errorf("Name = %v, want Test User", resp.Name)
	}
	if resp.Type != "user" {
		t.Errorf("Type = %v, want user", resp.Type)
	}
	if resp.HasMultipleRepositories != true {
		t.Error("HasMultipleRepositories should be true")
	}
	if resp.RepositoryCount != 5 {
		t.Errorf("RepositoryCount = %d, want 5", resp.RepositoryCount)
	}
	if resp.ConnectedAt == nil {
		t.Error("ConnectedAt should not be nil")
	}
	if resp.LastSyncedAt == nil {
		t.Error("LastSyncedAt should not be nil")
	}
	if len(resp.Repositories) != 1 {
		t.Errorf("Repositories length = %d, want 1", len(resp.Repositories))
	}
}

func TestToSourceControlResponse_NilValues(t *testing.T) {
	sc := &SourceControl{
		ID:        "test-id",
		Provider:  GitProviderGitHub,
		CreatedAt: time.Now(),
	}

	resp := ToSourceControlResponse(sc)

	if resp.ID != "test-id" {
		t.Errorf("ID = %v, want test-id", resp.ID)
	}
	if resp.Login != "" {
		t.Errorf("Login = %v, want empty string", resp.Login)
	}
	if resp.Name != "" {
		t.Errorf("Name = %v, want empty string", resp.Name)
	}
	if resp.ConnectedAt != nil {
		t.Error("ConnectedAt should be nil")
	}
	if resp.LastSyncedAt != nil {
		t.Error("LastSyncedAt should be nil")
	}
	if resp.Repositories != nil {
		t.Error("Repositories should be nil")
	}
}

func TestToRepositoryResponse(t *testing.T) {
	repo := &SourceControlRepository{
		ID:            "repo-123",
		Name:          "test-repo",
		FullName:      "user/test-repo",
		Public:        true,
		DefaultBranch: strPtr("main"),
		HTMLURL:       strPtr("https://github.com/user/test-repo"),
		SSHURL:        strPtr("git@github.com:user/test-repo.git"),
		AdditionalData: JSONMap{
			"id":       123,
			"language": "Go",
		},
	}

	resp := ToRepositoryResponse(repo)

	if resp.ID != "repo-123" {
		t.Errorf("ID = %v, want repo-123", resp.ID)
	}
	if resp.Name != "test-repo" {
		t.Errorf("Name = %v, want test-repo", resp.Name)
	}
	if resp.FullName != "user/test-repo" {
		t.Errorf("FullName = %v, want user/test-repo", resp.FullName)
	}
	if !resp.Public {
		t.Error("Public should be true")
	}
	if resp.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %v, want main", resp.DefaultBranch)
	}
	if resp.HTMLURL != "https://github.com/user/test-repo" {
		t.Errorf("HTMLURL = %v, want https://github.com/user/test-repo", resp.HTMLURL)
	}
	if resp.AdditionalData == nil {
		t.Error("AdditionalData should not be nil")
	}
}

func TestAppInstallationData_IsOrganization(t *testing.T) {
	tests := []struct {
		name        string
		accountType string
		expected    bool
	}{
		{"organization lowercase", "organization", true},
		{"Organization mixed case", "Organization", true},
		{"user", "user", false},
		{"User uppercase", "User", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &AppInstallationData{AccountType: tt.accountType}
			if got := a.IsOrganization(); got != tt.expected {
				t.Errorf("AppInstallationData.IsOrganization() = %v, want %v", got, tt.expected)
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
		{"user lowercase", "user", true},
		{"User mixed case", "User", true},
		{"organization", "organization", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &AppInstallationData{AccountType: tt.accountType}
			if got := a.IsUser(); got != tt.expected {
				t.Errorf("AppInstallationData.IsUser() = %v, want %v", got, tt.expected)
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
		{"not suspended", "", false},
		{"suspended", "2024-01-01T00:00:00Z", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &AppInstallationData{SuspendedAt: tt.suspendedAt}
			if got := a.IsSuspended(); got != tt.expected {
				t.Errorf("AppInstallationData.IsSuspended() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAppInstallationData_ToMap(t *testing.T) {
	a := &AppInstallationData{
		ID:               "123",
		AccountID:        "456",
		AccountLogin:     "testuser",
		AccountType:      "user",
		AccountAvatarURL: "https://example.com/avatar.png",
		Permissions:      map[string]interface{}{"contents": "read"},
		AppSlug:          "test-app",
		AppID:            789,
	}

	m := a.ToMap()

	if m["id"] != "123" {
		t.Errorf("id = %v, want 123", m["id"])
	}
	if m["account_login"] != "testuser" {
		t.Errorf("account_login = %v, want testuser", m["account_login"])
	}
	if m["app_id"] != int64(789) {
		t.Errorf("app_id = %v, want 789", m["app_id"])
	}
}

func TestCommitData_ToMap(t *testing.T) {
	c := &CommitData{
		CommitID: "abc123def",
		SHA:      "abc123d",
		Name:     "Test User",
		Email:    "test@example.com",
		Message:  "Test commit",
		URL:      "https://github.com/commit/abc123def",
		Branch:   "main",
	}

	m := c.ToMap()

	if m["commit_id"] != "abc123def" {
		t.Errorf("commit_id = %v, want abc123def", m["commit_id"])
	}
	if m["branch"] != "main" {
		t.Errorf("branch = %v, want main", m["branch"])
	}

	commitData, ok := m["commit_data"].(map[string]interface{})
	if !ok {
		t.Fatal("commit_data should be a map")
	}
	if commitData["sha"] != "abc123d" {
		t.Errorf("sha = %v, want abc123d", commitData["sha"])
	}
	if commitData["name"] != "Test User" {
		t.Errorf("name = %v, want Test User", commitData["name"])
	}
}

func TestCommitDataFromGitHubPayload(t *testing.T) {
	tests := []struct {
		name     string
		payload  map[string]interface{}
		expected *CommitData
	}{
		{
			name: "valid payload",
			payload: map[string]interface{}{
				"ref": "refs/heads/main",
				"head_commit": map[string]interface{}{
					"id":      "abc123def456789",
					"message": "Test commit",
					"url":     "https://github.com/commit/abc123def456789",
					"author": map[string]interface{}{
						"name":  "Test User",
						"email": "test@example.com",
					},
				},
			},
			expected: &CommitData{
				CommitID: "abc123def456789",
				SHA:      "abc123d",
				Name:     "Test User",
				Email:    "test@example.com",
				Message:  "Test commit",
				URL:      "https://github.com/commit/abc123def456789",
				Branch:   "main",
			},
		},
		{
			name:     "missing head_commit",
			payload:  map[string]interface{}{},
			expected: nil,
		},
		{
			name: "missing commit id",
			payload: map[string]interface{}{
				"head_commit": map[string]interface{}{},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CommitDataFromGitHubPayload(tt.payload)
			if tt.expected == nil {
				if got != nil {
					t.Errorf("Expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("Expected non-nil CommitData")
			}
			if got.CommitID != tt.expected.CommitID {
				t.Errorf("CommitID = %v, want %v", got.CommitID, tt.expected.CommitID)
			}
			if got.SHA != tt.expected.SHA {
				t.Errorf("SHA = %v, want %v", got.SHA, tt.expected.SHA)
			}
			if got.Branch != tt.expected.Branch {
				t.Errorf("Branch = %v, want %v", got.Branch, tt.expected.Branch)
			}
		})
	}
}

func TestCommitDataFromGitLabPayload(t *testing.T) {
	tests := []struct {
		name     string
		payload  map[string]interface{}
		expected *CommitData
	}{
		{
			name: "valid payload",
			payload: map[string]interface{}{
				"ref": "refs/heads/develop",
				"commits": []interface{}{
					map[string]interface{}{
						"id":    "abc123def456789",
						"title": "Test commit",
						"url":   "https://gitlab.com/commit/abc123def456789",
						"author": map[string]interface{}{
							"name":  "Test User",
							"email": "test@example.com",
						},
					},
				},
			},
			expected: &CommitData{
				CommitID: "abc123def456789",
				SHA:      "abc123d",
				Branch:   "develop",
			},
		},
		{
			name:     "empty commits",
			payload:  map[string]interface{}{"commits": []interface{}{}},
			expected: nil,
		},
		{
			name:     "missing commits",
			payload:  map[string]interface{}{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CommitDataFromGitLabPayload(tt.payload)
			if tt.expected == nil {
				if got != nil {
					t.Errorf("Expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("Expected non-nil CommitData")
			}
			if got.CommitID != tt.expected.CommitID {
				t.Errorf("CommitID = %v, want %v", got.CommitID, tt.expected.CommitID)
			}
			if got.Branch != tt.expected.Branch {
				t.Errorf("Branch = %v, want %v", got.Branch, tt.expected.Branch)
			}
		})
	}
}

func TestCommitDataFromBitbucketPayload(t *testing.T) {
	tests := []struct {
		name     string
		payload  map[string]interface{}
		expected *CommitData
	}{
		{
			name: "valid payload",
			payload: map[string]interface{}{
				"push": map[string]interface{}{
					"changes": []interface{}{
						map[string]interface{}{
							"new": map[string]interface{}{
								"name": "feature-branch",
							},
							"commits": []interface{}{
								map[string]interface{}{
									"hash":    "abc123def456789",
									"message": "Test commit\n",
									"author": map[string]interface{}{
										"raw": "Test User <test@example.com>",
									},
									"links": map[string]interface{}{
										"html": map[string]interface{}{
											"href": "https://bitbucket.org/commit/abc123def456789",
										},
									},
								},
							},
						},
					},
				},
			},
			expected: &CommitData{
				CommitID: "abc123def456789",
				SHA:      "abc123d",
				Name:     "Test User",
				Email:    "test@example.com",
				Branch:   "feature-branch",
			},
		},
		{
			name:     "missing push",
			payload:  map[string]interface{}{},
			expected: nil,
		},
		{
			name: "empty changes",
			payload: map[string]interface{}{
				"push": map[string]interface{}{
					"changes": []interface{}{},
				},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CommitDataFromBitbucketPayload(tt.payload)
			if tt.expected == nil {
				if got != nil {
					t.Errorf("Expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("Expected non-nil CommitData")
			}
			if got.CommitID != tt.expected.CommitID {
				t.Errorf("CommitID = %v, want %v", got.CommitID, tt.expected.CommitID)
			}
			if got.Branch != tt.expected.Branch {
				t.Errorf("Branch = %v, want %v", got.Branch, tt.expected.Branch)
			}
		})
	}
}

func TestParseCommitter(t *testing.T) {
	tests := []struct {
		name          string
		raw           string
		expectedName  string
		expectedEmail string
	}{
		{"valid format", "John Doe <john@example.com>", "John Doe", "john@example.com"},
		{"with spaces", "  John Doe  <john@example.com>", "John Doe", "john@example.com"},
		{"invalid format", "John Doe", "", ""},
		{"empty string", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, email := parseCommitter(tt.raw)
			if name != tt.expectedName {
				t.Errorf("name = %v, want %v", name, tt.expectedName)
			}
			if email != tt.expectedEmail {
				t.Errorf("email = %v, want %v", email, tt.expectedEmail)
			}
		})
	}
}

func TestRepositoryDataFromAPIResponse(t *testing.T) {
	data := map[string]interface{}{
		"id":             float64(123),
		"name":           "test-repo",
		"full_name":      "user/test-repo",
		"private":        false,
		"default_branch": "main",
		"html_url":       "https://github.com/user/test-repo",
		"ssh_url":        "git@github.com:user/test-repo.git",
		"description":    "A test repository",
		"language":       "Go",
	}

	result := RepositoryDataFromAPIResponse(data)

	if result.Name != "test-repo" {
		t.Errorf("Name = %v, want test-repo", result.Name)
	}
	if result.FullName != "user/test-repo" {
		t.Errorf("FullName = %v, want user/test-repo", result.FullName)
	}
	if !result.IsPublic {
		t.Error("IsPublic should be true for non-private repo")
	}
	if result.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %v, want main", result.DefaultBranch)
	}
}

func TestRepositoryDataFromAPIResponse_DefaultBranch(t *testing.T) {
	data := map[string]interface{}{
		"name":      "test-repo",
		"full_name": "user/test-repo",
	}

	result := RepositoryDataFromAPIResponse(data)

	if result.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %v, want main (default)", result.DefaultBranch)
	}
}

func TestRepositoryData_ToMap(t *testing.T) {
	rd := &RepositoryData{
		Name:          "test-repo",
		FullName:      "user/test-repo",
		IsPublic:      true,
		DefaultBranch: "main",
		HTMLURL:       "https://github.com/user/test-repo",
		SSHURL:        "git@github.com:user/test-repo.git",
		AdditionalData: map[string]interface{}{
			"id": 123,
		},
	}

	m := rd.ToMap()

	if m["name"] != "test-repo" {
		t.Errorf("name = %v, want test-repo", m["name"])
	}
	if m["public"] != true {
		t.Errorf("public = %v, want true", m["public"])
	}
}

func TestInstallationSummaryFromSourceControl(t *testing.T) {
	lastSynced := time.Now()

	sc := &SourceControl{
		ID:                      "sc-123",
		Provider:                GitProviderGitHub,
		ProviderID:              strPtr("prov-456"),
		Login:                   strPtr("testuser"),
		Name:                    strPtr("Test User"),
		Type:                    strPtr("user"),
		AvatarURL:               strPtr("https://example.com/avatar.png"),
		HTMLURL:                 strPtr("https://github.com/testuser"),
		InstallationID:          strPtr("inst-789"),
		RepositorySelection:     strPtr("all"),
		HasMultipleRepositories: false,
		RepositoryCount:         10,
		LastSyncedAt:            &lastSynced,
	}

	summary := InstallationSummaryFromSourceControl(sc)

	if summary.ID != "sc-123" {
		t.Errorf("ID = %v, want sc-123", summary.ID)
	}
	if summary.Provider != "github" {
		t.Errorf("Provider = %v, want github", summary.Provider)
	}
	if summary.ProviderID != "prov-456" {
		t.Errorf("ProviderID = %v, want prov-456", summary.ProviderID)
	}
	if summary.Login != "testuser" {
		t.Errorf("Login = %v, want testuser", summary.Login)
	}
	if summary.RepositoryCount != 10 {
		t.Errorf("RepositoryCount = %d, want 10", summary.RepositoryCount)
	}
	if summary.LastSyncedAt == "" {
		t.Error("LastSyncedAt should not be empty")
	}
}

func TestInstallationSummaryFromSourceControl_NilValues(t *testing.T) {
	sc := &SourceControl{
		ID:       "sc-123",
		Provider: GitProviderGitLab,
	}

	summary := InstallationSummaryFromSourceControl(sc)

	if summary.ID != "sc-123" {
		t.Errorf("ID = %v, want sc-123", summary.ID)
	}
	if summary.Login != "" {
		t.Errorf("Login = %v, want empty string", summary.Login)
	}
	if summary.LastSyncedAt != "" {
		t.Errorf("LastSyncedAt = %v, want empty string", summary.LastSyncedAt)
	}
}
