package git

import (
	"encoding/json"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	if err := db.AutoMigrate(&SourceControl{}, &SourceControlRepository{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	return db
}

func TestJSONMap_Value(t *testing.T) {
	tests := []struct {
		name     string
		input    JSONMap
		expected string
	}{
		{"nil map", nil, ""},
		{"empty map", JSONMap{}, "{}"},
		{"with values", JSONMap{"key": "value"}, `{"key":"value"}`},
		{"nested", JSONMap{"nested": map[string]interface{}{"key": "value"}}, `{"nested":{"key":"value"}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.input.Value()
			if err != nil {
				t.Errorf("JSONMap.Value() error = %v", err)
				return
			}

			if tt.input == nil {
				if got != nil {
					t.Errorf("JSONMap.Value() = %v, want nil", got)
				}
				return
			}

			gotBytes, ok := got.([]byte)
			if !ok {
				t.Errorf("JSONMap.Value() returned %T, want []byte", got)
				return
			}

			if string(gotBytes) != tt.expected {
				t.Errorf("JSONMap.Value() = %v, want %v", string(gotBytes), tt.expected)
			}
		})
	}
}

func TestJSONMap_Scan(t *testing.T) {
	tests := []struct {
		name      string
		input     interface{}
		expectNil bool
		expectErr bool
	}{
		{"nil value", nil, true, false},
		{"byte slice", []byte(`{"key":"value"}`), false, false},
		{"string", `{"key":"value"}`, false, false},
		{"invalid json", []byte(`invalid`), false, true},
		{"other type", 123, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m JSONMap
			err := m.Scan(tt.input)

			if (err != nil) != tt.expectErr {
				t.Errorf("JSONMap.Scan() error = %v, expectErr %v", err, tt.expectErr)
				return
			}

			if tt.expectNil && m != nil {
				t.Errorf("JSONMap.Scan() = %v, want nil", m)
			}
		})
	}
}

func TestJSONArray_Value(t *testing.T) {
	tests := []struct {
		name     string
		input    JSONArray
		expected string
	}{
		{"nil array", nil, ""},
		{"empty array", JSONArray{}, "[]"},
		{"with values", JSONArray{"a", "b"}, `["a","b"]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.input.Value()
			if err != nil {
				t.Errorf("JSONArray.Value() error = %v", err)
				return
			}

			if tt.input == nil {
				if got != nil {
					t.Errorf("JSONArray.Value() = %v, want nil", got)
				}
				return
			}

			gotBytes, ok := got.([]byte)
			if !ok {
				t.Errorf("JSONArray.Value() returned %T, want []byte", got)
				return
			}

			if string(gotBytes) != tt.expected {
				t.Errorf("JSONArray.Value() = %v, want %v", string(gotBytes), tt.expected)
			}
		})
	}
}

func TestJSONArray_Scan(t *testing.T) {
	tests := []struct {
		name      string
		input     interface{}
		expectNil bool
		expectErr bool
	}{
		{"nil value", nil, true, false},
		{"byte slice", []byte(`["a","b"]`), false, false},
		{"string", `["a","b"]`, false, false},
		{"invalid json", []byte(`invalid`), false, true},
		{"other type", 123, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var a JSONArray
			err := a.Scan(tt.input)

			if (err != nil) != tt.expectErr {
				t.Errorf("JSONArray.Scan() error = %v, expectErr %v", err, tt.expectErr)
				return
			}

			if tt.expectNil && a != nil {
				t.Errorf("JSONArray.Scan() = %v, want nil", a)
			}
		})
	}
}

func TestSourceControl_TableName(t *testing.T) {
	sc := SourceControl{}
	if got := sc.TableName(); got != "source_controls" {
		t.Errorf("SourceControl.TableName() = %v, want source_controls", got)
	}
}

func TestSourceControl_BeforeCreate(t *testing.T) {
	db := setupTestDB(t)

	sc := &SourceControl{
		UserID:   "user123",
		TeamID:   "team123",
		Provider: GitProviderGitHub,
	}

	if err := db.Create(sc).Error; err != nil {
		t.Fatalf("failed to create source control: %v", err)
	}

	if sc.ID == "" {
		t.Error("SourceControl.BeforeCreate() should have set ID")
	}

	if len(sc.ID) != 26 {
		t.Errorf("SourceControl.ID length = %d, want 26 (ULID)", len(sc.ID))
	}
}

func TestSourceControl_IsOrganization(t *testing.T) {
	tests := []struct {
		name     string
		scType   *string
		expected bool
	}{
		{"nil type", nil, false},
		{"user type", strPtr("user"), false},
		{"organization type", strPtr("organization"), true},
		{"Organization uppercase", strPtr("Organization"), true},
		{"unknown type", strPtr("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := &SourceControl{Type: tt.scType}
			if got := sc.IsOrganization(); got != tt.expected {
				t.Errorf("SourceControl.IsOrganization() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSourceControl_IsUser(t *testing.T) {
	tests := []struct {
		name     string
		scType   *string
		expected bool
	}{
		{"nil type defaults to user", nil, true},
		{"user type", strPtr("user"), true},
		{"organization type", strPtr("organization"), false},
		{"User uppercase", strPtr("User"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := &SourceControl{Type: tt.scType}
			if got := sc.IsUser(); got != tt.expected {
				t.Errorf("SourceControl.IsUser() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSourceControl_NeedsSynchronization(t *testing.T) {
	now := time.Now()
	twoHoursAgo := now.Add(-2 * time.Hour)
	thirtyMinutesAgo := now.Add(-30 * time.Minute)

	tests := []struct {
		name         string
		lastSyncedAt *time.Time
		expected     bool
	}{
		{"nil last synced", nil, true},
		{"synced 2 hours ago", &twoHoursAgo, true},
		{"synced 30 minutes ago", &thirtyMinutesAgo, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := &SourceControl{LastSyncedAt: tt.lastSyncedAt}
			if got := sc.NeedsSynchronization(); got != tt.expected {
				t.Errorf("SourceControl.NeedsSynchronization() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSourceControl_GetLogin(t *testing.T) {
	tests := []struct {
		name     string
		login    *string
		expected string
	}{
		{"nil login", nil, ""},
		{"with login", strPtr("testuser"), "testuser"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := &SourceControl{Login: tt.login}
			if got := sc.GetLogin(); got != tt.expected {
				t.Errorf("SourceControl.GetLogin() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSourceControl_GetInstallationID(t *testing.T) {
	tests := []struct {
		name           string
		installationID *string
		expected       string
	}{
		{"nil installation ID", nil, ""},
		{"with installation ID", strPtr("12345"), "12345"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := &SourceControl{InstallationID: tt.installationID}
			if got := sc.GetInstallationID(); got != tt.expected {
				t.Errorf("SourceControl.GetInstallationID() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSourceControlRepository_TableName(t *testing.T) {
	repo := SourceControlRepository{}
	if got := repo.TableName(); got != "source_control_repositories" {
		t.Errorf("SourceControlRepository.TableName() = %v, want source_control_repositories", got)
	}
}

func TestSourceControlRepository_BeforeCreate(t *testing.T) {
	db := setupTestDB(t)

	// First create a source control
	sc := &SourceControl{
		UserID:   "user123",
		TeamID:   "team123",
		Provider: GitProviderGitHub,
	}
	db.Create(sc)

	repo := &SourceControlRepository{
		SourceControlID: sc.ID,
		Name:            "test-repo",
		FullName:        "user/test-repo",
	}

	if err := db.Create(repo).Error; err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	if repo.ID == "" {
		t.Error("SourceControlRepository.BeforeCreate() should have set ID")
	}

	if len(repo.ID) != 26 {
		t.Errorf("SourceControlRepository.ID length = %d, want 26 (ULID)", len(repo.ID))
	}
}

func TestSourceControlRepository_GetDefaultBranch(t *testing.T) {
	tests := []struct {
		name          string
		defaultBranch *string
		expected      string
	}{
		{"nil default branch", nil, "main"},
		{"empty default branch", strPtr(""), "main"},
		{"with default branch", strPtr("master"), "master"},
		{"develop branch", strPtr("develop"), "develop"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &SourceControlRepository{DefaultBranch: tt.defaultBranch}
			if got := repo.GetDefaultBranch(); got != tt.expected {
				t.Errorf("SourceControlRepository.GetDefaultBranch() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSourceControlRepository_GetHTMLURL(t *testing.T) {
	tests := []struct {
		name     string
		htmlURL  *string
		expected string
	}{
		{"nil HTML URL", nil, ""},
		{"with HTML URL", strPtr("https://github.com/user/repo"), "https://github.com/user/repo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &SourceControlRepository{HTMLURL: tt.htmlURL}
			if got := repo.GetHTMLURL(); got != tt.expected {
				t.Errorf("SourceControlRepository.GetHTMLURL() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSourceControlRepository_GetSSHURL(t *testing.T) {
	tests := []struct {
		name     string
		sshURL   *string
		expected string
	}{
		{"nil SSH URL", nil, ""},
		{"with SSH URL", strPtr("git@github.com:user/repo.git"), "git@github.com:user/repo.git"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &SourceControlRepository{SSHURL: tt.sshURL}
			if got := repo.GetSSHURL(); got != tt.expected {
				t.Errorf("SourceControlRepository.GetSSHURL() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSourceControl_WithRepositories(t *testing.T) {
	db := setupTestDB(t)

	sc := &SourceControl{
		UserID:   "user123",
		TeamID:   "team123",
		Provider: GitProviderGitHub,
	}
	db.Create(sc)

	repo1 := &SourceControlRepository{
		SourceControlID: sc.ID,
		Name:            "repo1",
		FullName:        "user/repo1",
		Public:          true,
	}
	repo2 := &SourceControlRepository{
		SourceControlID: sc.ID,
		Name:            "repo2",
		FullName:        "user/repo2",
		Public:          false,
	}
	db.Create(repo1)
	db.Create(repo2)

	// Fetch with preload
	var fetchedSC SourceControl
	db.Preload("Repositories").First(&fetchedSC, "id = ?", sc.ID)

	if len(fetchedSC.Repositories) != 2 {
		t.Errorf("Expected 2 repositories, got %d", len(fetchedSC.Repositories))
	}
}

func TestJSONMap_MarshalJSON(t *testing.T) {
	m := JSONMap{
		"string": "value",
		"number": 42,
		"bool":   true,
		"nested": map[string]interface{}{
			"key": "nested_value",
		},
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Failed to marshal JSONMap: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if result["string"] != "value" {
		t.Errorf("Expected string = 'value', got %v", result["string"])
	}
}

// Helper function to create string pointer
func strPtr(s string) *string {
	return &s
}
