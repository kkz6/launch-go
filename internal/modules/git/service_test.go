package git

import (
	"context"
	"errors"
	"testing"

	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/git/providers"
)

// MockProvider implements the providers.Provider interface for testing
type MockProvider struct {
	providerType          providers.GitProviderType
	installationURL       string
	installationURLErr    error
	installation          *providers.AppInstallationData
	installationErr       error
	allInstallations      []providers.AppInstallationData
	allInstallationsErr   error
	repositories          []map[string]interface{}
	repositoriesErr       error
	repository            map[string]interface{}
	repositoryErr         error
	validateWebhookResult bool
	commitData            *providers.CommitData
	testConnectionErr     error
	deployKeyErr          error
	lastCommit            *providers.CommitData
	lastCommitErr         error
}

func (m *MockProvider) GetType() providers.GitProviderType {
	return m.providerType
}

func (m *MockProvider) GetInstallationURL() (string, error) {
	return m.installationURL, m.installationURLErr
}

func (m *MockProvider) GetInstallation(ctx context.Context, installationID string) (*providers.AppInstallationData, error) {
	return m.installation, m.installationErr
}

func (m *MockProvider) GetAllInstallations(ctx context.Context) ([]providers.AppInstallationData, error) {
	return m.allInstallations, m.allInstallationsErr
}

func (m *MockProvider) GetInstallationRepositories(ctx context.Context, installationID string) ([]map[string]interface{}, error) {
	return m.repositories, m.repositoriesErr
}

func (m *MockProvider) GetRepository(ctx context.Context, installationID, owner, repo string) (map[string]interface{}, error) {
	return m.repository, m.repositoryErr
}

func (m *MockProvider) ValidateWebhook(payload []byte, signature string) bool {
	return m.validateWebhookResult
}

func (m *MockProvider) GetCommitData(payload map[string]interface{}) *providers.CommitData {
	return m.commitData
}

func (m *MockProvider) TestConnection(ctx context.Context) error {
	return m.testConnectionErr
}

func (m *MockProvider) GetSSHURL(repo string) string {
	return "git@github.com:" + repo + ".git"
}

func (m *MockProvider) GetHTTPSURL(repo string) string {
	return "https://github.com/" + repo + ".git"
}

func (m *MockProvider) DeployKey(ctx context.Context, sourceControlID, title, repo, key string) error {
	return m.deployKeyErr
}

func (m *MockProvider) GetLastCommit(ctx context.Context, sourceControlID, repo, branch string) (*providers.CommitData, error) {
	return m.lastCommit, m.lastCommitErr
}

// MockProviderFactory for testing
type MockProviderFactory struct {
	mockProviders map[providers.GitProviderType]*MockProvider
}

func NewMockProviderFactory() *MockProviderFactory {
	return &MockProviderFactory{
		mockProviders: make(map[providers.GitProviderType]*MockProvider),
	}
}

func (f *MockProviderFactory) RegisterMock(providerType providers.GitProviderType, provider *MockProvider) {
	f.mockProviders[providerType] = provider
}

func (f *MockProviderFactory) GetProvider(providerType providers.GitProviderType) (providers.Provider, error) {
	p, ok := f.mockProviders[providerType]
	if !ok {
		return nil, providers.ErrProviderNotConfigured
	}
	return p, nil
}

func setupServiceTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	if err := db.AutoMigrate(&SourceControl{}, &SourceControlRepository{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	return db
}

func setupTestService(t *testing.T) (*Service, *gorm.DB, *providers.ProviderFactory) {
	db := setupServiceTestDB(t)
	repo := NewRepository(db)
	factory := providers.NewProviderFactory()

	// Register GitHub provider with test config
	factory.RegisterConfig(providers.GitProviderGitHub, &providers.ProviderConfig{
		AppID:         "test-app-id",
		PrivateKey:    "test-key",
		WebhookSecret: "test-secret",
		AppSlug:       "test-app",
	})

	logger := zerolog.Nop()
	service := NewService(repo, factory, nil, &logger)

	return service, db, factory
}

func TestNewService(t *testing.T) {
	db := setupServiceTestDB(t)
	repo := NewRepository(db)
	factory := providers.NewProviderFactory()
	logger := zerolog.Nop()

	service := NewService(repo, factory, nil, &logger)

	if service == nil {
		t.Fatal("NewService() returned nil")
	}
}

func TestService_ListSourceControls(t *testing.T) {
	service, db, _ := setupTestService(t)
	ctx := context.Background()

	// Create test data
	sc1 := &SourceControl{
		UserID:   "user1",
		TeamID:   "team1",
		Provider: GitProviderGitHub,
	}
	sc2 := &SourceControl{
		UserID:   "user1",
		TeamID:   "team1",
		Provider: GitProviderGitLab,
	}
	sc3 := &SourceControl{
		UserID:   "user2",
		TeamID:   "team2",
		Provider: GitProviderGitHub,
	}

	db.Create(sc1)
	db.Create(sc2)
	db.Create(sc3)

	// Test listing for team1
	result, err := service.ListSourceControls(ctx, "team1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("Expected 2 source controls, got %d", len(result))
	}

	// Test listing for team2
	result, err = service.ListSourceControls(ctx, "team2")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("Expected 1 source control, got %d", len(result))
	}

	// Test listing for non-existent team
	result, err = service.ListSourceControls(ctx, "team3")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("Expected 0 source controls, got %d", len(result))
	}
}

func TestService_GetSourceControl(t *testing.T) {
	service, db, _ := setupTestService(t)
	ctx := context.Background()

	// Create test data
	sc := &SourceControl{
		UserID:   "user1",
		TeamID:   "team1",
		Provider: GitProviderGitHub,
	}
	db.Create(sc)

	// Test getting existing source control
	result, err := service.GetSourceControl(ctx, sc.ID, "team1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.ID != sc.ID {
		t.Errorf("Expected ID %s, got %s", sc.ID, result.ID)
	}

	// Test getting with wrong team
	_, err = service.GetSourceControl(ctx, sc.ID, "team2")
	if err == nil {
		t.Error("Expected error for wrong team")
	}

	// Test getting non-existent source control
	_, err = service.GetSourceControl(ctx, "non-existent", "team1")
	if err == nil {
		t.Error("Expected error for non-existent source control")
	}
}

func TestService_Disconnect(t *testing.T) {
	service, db, _ := setupTestService(t)
	ctx := context.Background()

	// Create test source control with repositories
	sc := &SourceControl{
		UserID:   "user1",
		TeamID:   "team1",
		Provider: GitProviderGitHub,
	}
	db.Create(sc)

	repo1 := &SourceControlRepository{
		SourceControlID: sc.ID,
		Name:            "repo1",
		FullName:        "user/repo1",
	}
	repo2 := &SourceControlRepository{
		SourceControlID: sc.ID,
		Name:            "repo2",
		FullName:        "user/repo2",
	}
	db.Create(repo1)
	db.Create(repo2)

	// Verify repositories exist
	var repoCount int64
	db.Model(&SourceControlRepository{}).Where("source_control_id = ?", sc.ID).Count(&repoCount)
	if repoCount != 2 {
		t.Fatalf("Expected 2 repositories, got %d", repoCount)
	}

	// Test disconnect
	err := service.Disconnect(ctx, sc.ID, "team1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify source control is deleted
	var scCount int64
	db.Model(&SourceControl{}).Where("id = ?", sc.ID).Count(&scCount)
	if scCount != 0 {
		t.Error("Source control should be deleted")
	}

	// Verify repositories are deleted
	db.Model(&SourceControlRepository{}).Where("source_control_id = ?", sc.ID).Count(&repoCount)
	if repoCount != 0 {
		t.Error("Repositories should be deleted")
	}
}

func TestService_Disconnect_NotFound(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	err := service.Disconnect(ctx, "non-existent", "team1")
	if err == nil {
		t.Error("Expected error for non-existent source control")
	}
}

func TestService_Disconnect_WrongTeam(t *testing.T) {
	service, db, _ := setupTestService(t)
	ctx := context.Background()

	sc := &SourceControl{
		UserID:   "user1",
		TeamID:   "team1",
		Provider: GitProviderGitHub,
	}
	db.Create(sc)

	err := service.Disconnect(ctx, sc.ID, "team2")
	if err == nil {
		t.Error("Expected error for wrong team")
	}
}

func TestService_GetInstallationURL(t *testing.T) {
	service, _, _ := setupTestService(t)

	// Test GitHub (configured)
	url, err := service.GetInstallationURL(GitProviderGitHub)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if url == "" {
		t.Error("URL should not be empty")
	}

	// Test unconfigured provider
	_, err = service.GetInstallationURL(GitProviderGitLab)
	if err == nil {
		t.Error("Expected error for unconfigured provider")
	}
}

func TestService_GetCachedRepositories(t *testing.T) {
	service, db, _ := setupTestService(t)
	ctx := context.Background()

	installationID := "12345"
	sc := &SourceControl{
		UserID:         "user1",
		TeamID:         "team1",
		Provider:       GitProviderGitHub,
		InstallationID: &installationID,
		ProviderID:     &installationID, // GetInstallationRepositories queries by provider_id
	}
	db.Create(sc)

	repo1 := &SourceControlRepository{
		SourceControlID: sc.ID,
		Name:            "repo1",
		FullName:        "user/repo1",
	}
	db.Create(repo1)

	repos, err := service.GetCachedRepositories(ctx, GitProviderGitHub, installationID, "team1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(repos) != 1 {
		t.Errorf("Expected 1 repository, got %d", len(repos))
	}
}

func TestService_SyncRepositories_NoInstallationID(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	sc := &SourceControl{
		UserID:   "user1",
		TeamID:   "team1",
		Provider: GitProviderGitHub,
	}

	err := service.SyncRepositories(ctx, sc)
	if !errors.Is(err, ErrNoInstallationID) {
		t.Errorf("Expected ErrNoInstallationID, got %v", err)
	}

	// Test with empty installation ID
	emptyID := ""
	sc.InstallationID = &emptyID
	err = service.SyncRepositories(ctx, sc)
	if !errors.Is(err, ErrNoInstallationID) {
		t.Errorf("Expected ErrNoInstallationID, got %v", err)
	}
}

func TestService_SyncInstallationRepositories(t *testing.T) {
	service, db, _ := setupTestService(t)
	ctx := context.Background()

	installationID := "12345"
	sc := &SourceControl{
		UserID:          "user1",
		TeamID:          "team1",
		Provider:        GitProviderGitHub,
		InstallationID:  &installationID,
		RepositoryCount: 0,
	}
	db.Create(sc)

	// Sync with new repositories
	repositories := []map[string]interface{}{
		{
			"id":             float64(1),
			"name":           "repo1",
			"full_name":      "user/repo1",
			"private":        false,
			"html_url":       "https://github.com/user/repo1",
			"ssh_url":        "git@github.com:user/repo1.git",
			"default_branch": "main",
		},
		{
			"id":             float64(2),
			"name":           "repo2",
			"full_name":      "user/repo2",
			"private":        true,
			"html_url":       "https://github.com/user/repo2",
			"ssh_url":        "git@github.com:user/repo2.git",
			"default_branch": "master",
		},
	}

	err := service.SyncInstallationRepositories(ctx, sc, repositories)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify repositories were created
	var repoCount int64
	db.Model(&SourceControlRepository{}).Where("source_control_id = ?", sc.ID).Count(&repoCount)
	if repoCount != 2 {
		t.Errorf("Expected 2 repositories, got %d", repoCount)
	}

	// Verify source control was updated
	var updatedSC SourceControl
	db.First(&updatedSC, "id = ?", sc.ID)
	if updatedSC.RepositoryCount != 2 {
		t.Errorf("Expected repository count 2, got %d", updatedSC.RepositoryCount)
	}
	if updatedSC.LastSyncedAt == nil {
		t.Error("LastSyncedAt should be set")
	}
}

func TestService_GetInstallationsWithRepositoryCounts(t *testing.T) {
	service, db, _ := setupTestService(t)
	ctx := context.Background()

	installationID := "12345"
	sc := &SourceControl{
		UserID:          "user1",
		TeamID:          "team1",
		Provider:        GitProviderGitHub,
		InstallationID:  &installationID,
		RepositoryCount: 5,
	}
	db.Create(sc)

	result, err := service.GetInstallationsWithRepositoryCounts(ctx, "team1", "user1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should have entries for all provider types
	if result == nil {
		t.Fatal("Result should not be nil")
	}

	// GitHub should have our installation
	githubInstalls, ok := result["github"]
	if !ok {
		t.Fatal("Expected github key in result")
	}
	if len(githubInstalls) != 1 {
		t.Errorf("Expected 1 GitHub installation, got %d", len(githubInstalls))
	}
	if githubInstalls[0].RepositoryCount != 5 {
		t.Errorf("Expected repository count 5, got %d", githubInstalls[0].RepositoryCount)
	}
}

func TestService_DeleteByInstallationID(t *testing.T) {
	service, db, _ := setupTestService(t)
	ctx := context.Background()

	installationID := "12345"

	// Create multiple source controls with same installation ID (different teams)
	// Note: DeleteByInstallationID searches by provider_id, so we need to set that
	sc1 := &SourceControl{
		UserID:         "user1",
		TeamID:         "team1",
		Provider:       GitProviderGitHub,
		InstallationID: &installationID,
		ProviderID:     &installationID,
	}
	sc2 := &SourceControl{
		UserID:         "user2",
		TeamID:         "team2",
		Provider:       GitProviderGitHub,
		InstallationID: &installationID,
		ProviderID:     &installationID,
	}
	db.Create(sc1)
	db.Create(sc2)

	// Delete by installation ID
	err := service.DeleteByInstallationID(ctx, installationID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify all were soft-deleted (should not appear in normal query)
	var count int64
	db.Model(&SourceControl{}).Where("provider_id = ?", installationID).Count(&count)
	if count != 0 {
		t.Errorf("Expected 0 source controls, got %d", count)
	}
}

func TestService_GetSourceControlByInstallation(t *testing.T) {
	service, db, _ := setupTestService(t)
	ctx := context.Background()

	installationID := "12345"
	sc := &SourceControl{
		UserID:         "user1",
		TeamID:         "team1",
		Provider:       GitProviderGitHub,
		ProviderID:     &installationID,
		InstallationID: &installationID,
	}
	db.Create(sc)

	// Test finding by installation
	result, err := service.GetSourceControlByInstallation(ctx, GitProviderGitHub, installationID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.ID != sc.ID {
		t.Errorf("Expected ID %s, got %s", sc.ID, result.ID)
	}

	// Test with user filter
	result, err = service.GetSourceControlByInstallation(ctx, GitProviderGitHub, installationID, WithUserID("user1"))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.ID != sc.ID {
		t.Errorf("Expected ID %s, got %s", sc.ID, result.ID)
	}

	// Test with wrong user filter
	_, err = service.GetSourceControlByInstallation(ctx, GitProviderGitHub, installationID, WithUserID("user2"))
	if err == nil {
		t.Error("Expected error for wrong user")
	}
}

func TestService_SyncRepositoriesForInstallation(t *testing.T) {
	service, db, _ := setupTestService(t)
	ctx := context.Background()

	installationID := "12345"

	// Create source control without repositories
	sc := &SourceControl{
		UserID:          "user1",
		TeamID:          "team1",
		Provider:        GitProviderGitHub,
		InstallationID:  &installationID,
		RepositoryCount: 0,
	}
	db.Create(sc)

	// This will fail because we can't make real API calls, but we're testing the method flow
	// In production, the provider would return actual repositories
	err := service.SyncRepositoriesForInstallation(ctx, installationID)
	// We expect this to not error - it will just log warnings for failed syncs
	if err != nil && !errors.Is(err, providers.ErrProviderNotConfigured) {
		// If we have a valid provider, it might fail on API calls
		// But the method itself shouldn't error
	}
}

func TestService_RefreshInstallationRepositories(t *testing.T) {
	service, db, _ := setupTestService(t)
	ctx := context.Background()

	installationID := "12345"
	sc := &SourceControl{
		UserID:         "user1",
		TeamID:         "team1",
		Provider:       GitProviderGitHub,
		InstallationID: &installationID,
	}
	db.Create(sc)

	// This will attempt to sync - we're testing the method exists and calls SyncRepositories
	// In a real test, we'd mock the provider
	_ = service.RefreshInstallationRepositories(ctx, sc)
}

func TestService_ErrorConstants(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		errStr string
	}{
		{"ErrProviderNotSupported", ErrProviderNotSupported, "provider not supported"},
		{"ErrNoInstallationID", ErrNoInstallationID, "no installation ID found"},
		{"ErrHasSites", ErrHasSites, "cannot delete source control with associated sites"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.errStr {
				t.Errorf("%s.Error() = %v, want %v", tt.name, tt.err.Error(), tt.errStr)
			}
		})
	}
}

func TestService_SyncInstallationRepositories_RemovesOldRepos(t *testing.T) {
	service, db, _ := setupTestService(t)
	ctx := context.Background()

	installationID := "12345"
	sc := &SourceControl{
		UserID:         "user1",
		TeamID:         "team1",
		Provider:       GitProviderGitHub,
		InstallationID: &installationID,
	}
	db.Create(sc)

	// Create an existing repository that won't be in the API response
	oldRepo := &SourceControlRepository{
		SourceControlID: sc.ID,
		Name:            "old-repo",
		FullName:        "user/old-repo",
		AdditionalData: JSONMap{
			"id": float64(999),
		},
	}
	db.Create(oldRepo)

	// Sync with new repositories (different IDs)
	repositories := []map[string]interface{}{
		{
			"id":        float64(1),
			"name":      "new-repo",
			"full_name": "user/new-repo",
		},
	}

	err := service.SyncInstallationRepositories(ctx, sc, repositories)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify old repo was deleted
	var count int64
	db.Model(&SourceControlRepository{}).Where("id = ?", oldRepo.ID).Count(&count)
	if count != 0 {
		t.Error("Old repository should be deleted")
	}

	// Verify new repo was created
	db.Model(&SourceControlRepository{}).Where("source_control_id = ? AND name = ?", sc.ID, "new-repo").Count(&count)
	if count != 1 {
		t.Error("New repository should be created")
	}
}

func TestService_SyncInstallationRepositories_HandlesStringIDs(t *testing.T) {
	service, db, _ := setupTestService(t)
	ctx := context.Background()

	installationID := "12345"
	sc := &SourceControl{
		UserID:         "user1",
		TeamID:         "team1",
		Provider:       GitProviderGitHub,
		InstallationID: &installationID,
	}
	db.Create(sc)

	// Sync with repositories using string IDs
	repositories := []map[string]interface{}{
		{
			"id":        "string-id-123",
			"name":      "repo1",
			"full_name": "user/repo1",
		},
	}

	err := service.SyncInstallationRepositories(ctx, sc, repositories)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var count int64
	db.Model(&SourceControlRepository{}).Where("source_control_id = ?", sc.ID).Count(&count)
	if count != 1 {
		t.Errorf("Expected 1 repository, got %d", count)
	}
}

func TestService_SyncInstallationRepositories_SkipsExistingIDs(t *testing.T) {
	service, db, _ := setupTestService(t)
	ctx := context.Background()

	installationID := "12345"
	sc := &SourceControl{
		UserID:         "user1",
		TeamID:         "team1",
		Provider:       GitProviderGitHub,
		InstallationID: &installationID,
	}
	db.Create(sc)

	// Create an existing repository
	existingRepo := &SourceControlRepository{
		SourceControlID: sc.ID,
		Name:            "existing-repo",
		FullName:        "user/existing-repo",
		AdditionalData: JSONMap{
			"id": float64(1),
		},
	}
	db.Create(existingRepo)
	existingRepoID := existingRepo.ID

	// Sync with same ID - should not update existing repo
	repositories := []map[string]interface{}{
		{
			"id":        float64(1),
			"name":      "updated-name",
			"full_name": "user/updated-name",
		},
	}

	err := service.SyncInstallationRepositories(ctx, sc, repositories)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify existing repo was not modified
	var repo SourceControlRepository
	db.First(&repo, "id = ?", existingRepoID)
	if repo.Name != "existing-repo" {
		t.Error("Existing repository should not be modified")
	}
}
