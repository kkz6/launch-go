package git

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestRepository(t *testing.T) (*Repository, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	if err := db.AutoMigrate(&SourceControl{}, &SourceControlRepository{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	repo := NewRepository(db)
	return repo, db
}

func createTestSourceControl(t *testing.T, db *gorm.DB, provider GitProviderType, teamID, userID string) *SourceControl {
	installationID := "test-installation"
	sc := &SourceControl{
		TeamID:         teamID,
		UserID:         userID,
		Provider:       provider,
		InstallationID: &installationID,
		ProviderID:     &installationID,
		Login:          strPtr("testuser"),
	}
	if err := db.Create(sc).Error; err != nil {
		t.Fatalf("failed to create source control: %v", err)
	}
	return sc
}

func TestNewRepository(t *testing.T) {
	repo, _ := setupTestRepository(t)
	if repo == nil {
		t.Error("NewRepository should return non-nil repository")
	}
}

func TestRepository_Create(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	sc := &SourceControl{
		TeamID:   "team-123",
		UserID:   "user-123",
		Provider: GitProviderGitHub,
	}

	err := repo.Create(ctx, sc)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if sc.ID == "" {
		t.Error("Create() should set ID")
	}
}

func TestRepository_Update(t *testing.T) {
	repo, db := setupTestRepository(t)
	ctx := context.Background()

	sc := createTestSourceControl(t, db, GitProviderGitHub, "team-123", "user-123")

	newLogin := "updated-user"
	sc.Login = &newLogin

	err := repo.Update(ctx, sc)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify update
	var updated SourceControl
	db.First(&updated, "id = ?", sc.ID)
	if *updated.Login != "updated-user" {
		t.Errorf("Update() login = %v, want updated-user", *updated.Login)
	}
}

func TestRepository_UpdateFields(t *testing.T) {
	repo, db := setupTestRepository(t)
	ctx := context.Background()

	sc := createTestSourceControl(t, db, GitProviderGitHub, "team-123", "user-123")

	err := repo.UpdateFields(ctx, sc.ID, map[string]interface{}{
		"repository_count": 10,
	})
	if err != nil {
		t.Fatalf("UpdateFields() error = %v", err)
	}

	var updated SourceControl
	db.First(&updated, "id = ?", sc.ID)
	if updated.RepositoryCount != 10 {
		t.Errorf("UpdateFields() repository_count = %d, want 10", updated.RepositoryCount)
	}
}

func TestRepository_Delete(t *testing.T) {
	repo, db := setupTestRepository(t)
	ctx := context.Background()

	sc := createTestSourceControl(t, db, GitProviderGitHub, "team-123", "user-123")

	err := repo.Delete(ctx, sc.ID)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify soft delete
	var count int64
	db.Unscoped().Model(&SourceControl{}).Where("id = ?", sc.ID).Count(&count)
	if count != 1 {
		t.Error("Delete() should soft delete, not hard delete")
	}

	var deleted SourceControl
	db.Unscoped().First(&deleted, "id = ?", sc.ID)
	if !deleted.DeletedAt.Valid {
		t.Error("Delete() should set deleted_at")
	}
}

func TestRepository_FindByID(t *testing.T) {
	repo, db := setupTestRepository(t)
	ctx := context.Background()

	sc := createTestSourceControl(t, db, GitProviderGitHub, "team-123", "user-123")

	found, err := repo.FindByID(ctx, sc.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if found.ID != sc.ID {
		t.Errorf("FindByID() ID = %v, want %v", found.ID, sc.ID)
	}
}

func TestRepository_FindByID_NotFound(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	_, err := repo.FindByID(ctx, "nonexistent")
	if err != ErrSourceControlNotFound {
		t.Errorf("FindByID() error = %v, want ErrSourceControlNotFound", err)
	}
}

func TestRepository_FindByIDAndTeam(t *testing.T) {
	repo, db := setupTestRepository(t)
	ctx := context.Background()

	sc := createTestSourceControl(t, db, GitProviderGitHub, "team-123", "user-123")

	found, err := repo.FindByIDAndTeam(ctx, sc.ID, "team-123")
	if err != nil {
		t.Fatalf("FindByIDAndTeam() error = %v", err)
	}

	if found.TeamID != "team-123" {
		t.Errorf("FindByIDAndTeam() TeamID = %v, want team-123", found.TeamID)
	}
}

func TestRepository_FindByIDAndTeam_WrongTeam(t *testing.T) {
	repo, db := setupTestRepository(t)
	ctx := context.Background()

	sc := createTestSourceControl(t, db, GitProviderGitHub, "team-123", "user-123")

	_, err := repo.FindByIDAndTeam(ctx, sc.ID, "other-team")
	if err != ErrSourceControlNotFound {
		t.Errorf("FindByIDAndTeam() error = %v, want ErrSourceControlNotFound", err)
	}
}

func TestRepository_FindAllByTeam(t *testing.T) {
	repo, db := setupTestRepository(t)
	ctx := context.Background()

	createTestSourceControl(t, db, GitProviderGitHub, "team-123", "user-123")
	createTestSourceControl(t, db, GitProviderGitLab, "team-123", "user-123")
	createTestSourceControl(t, db, GitProviderGitHub, "team-456", "user-123")

	found, err := repo.FindAllByTeam(ctx, "team-123")
	if err != nil {
		t.Fatalf("FindAllByTeam() error = %v", err)
	}

	if len(found) != 2 {
		t.Errorf("FindAllByTeam() returned %d results, want 2", len(found))
	}
}

func TestRepository_FindAllByUser(t *testing.T) {
	repo, db := setupTestRepository(t)
	ctx := context.Background()

	createTestSourceControl(t, db, GitProviderGitHub, "team-123", "user-123")
	createTestSourceControl(t, db, GitProviderGitHub, "team-456", "user-123")
	createTestSourceControl(t, db, GitProviderGitHub, "team-789", "user-456")

	found, err := repo.FindAllByUser(ctx, "user-123")
	if err != nil {
		t.Fatalf("FindAllByUser() error = %v", err)
	}

	if len(found) != 2 {
		t.Errorf("FindAllByUser() returned %d results, want 2", len(found))
	}
}

func TestRepository_FindByProvider(t *testing.T) {
	repo, db := setupTestRepository(t)
	ctx := context.Background()

	createTestSourceControl(t, db, GitProviderGitHub, "team-123", "user-123")
	createTestSourceControl(t, db, GitProviderGitHub, "team-456", "user-123")
	createTestSourceControl(t, db, GitProviderGitLab, "team-789", "user-123")

	found, err := repo.FindByProvider(ctx, GitProviderGitHub)
	if err != nil {
		t.Fatalf("FindByProvider() error = %v", err)
	}

	if len(found) != 2 {
		t.Errorf("FindByProvider() returned %d results, want 2", len(found))
	}
}

func TestRepository_FindByTeamAndProvider(t *testing.T) {
	repo, db := setupTestRepository(t)
	ctx := context.Background()

	createTestSourceControl(t, db, GitProviderGitHub, "team-123", "user-123")
	createTestSourceControl(t, db, GitProviderGitLab, "team-123", "user-123")
	createTestSourceControl(t, db, GitProviderGitHub, "team-456", "user-123")

	found, err := repo.FindByTeamAndProvider(ctx, "team-123", GitProviderGitHub)
	if err != nil {
		t.Fatalf("FindByTeamAndProvider() error = %v", err)
	}

	if len(found) != 1 {
		t.Errorf("FindByTeamAndProvider() returned %d results, want 1", len(found))
	}
}

func TestRepository_FindByProviderAndInstallationAndTeam(t *testing.T) {
	repo, db := setupTestRepository(t)
	ctx := context.Background()

	sc := createTestSourceControl(t, db, GitProviderGitHub, "team-123", "user-123")

	found, err := repo.FindByProviderAndInstallationAndTeam(ctx, GitProviderGitHub, *sc.ProviderID, "team-123")
	if err != nil {
		t.Fatalf("FindByProviderAndInstallationAndTeam() error = %v", err)
	}

	if found.ID != sc.ID {
		t.Errorf("FindByProviderAndInstallationAndTeam() ID = %v, want %v", found.ID, sc.ID)
	}
}

func TestRepository_FirstOrCreateByProviderAndInstallationAndTeam_Create(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	sc, created, err := repo.FirstOrCreateByProviderAndInstallationAndTeam(
		ctx,
		GitProviderGitHub,
		"new-installation",
		"team-123",
		map[string]interface{}{
			"user_id": "user-123",
		},
	)

	if err != nil {
		t.Fatalf("FirstOrCreateByProviderAndInstallationAndTeam() error = %v", err)
	}
	if !created {
		t.Error("FirstOrCreateByProviderAndInstallationAndTeam() should have created new record")
	}
	if sc.ID == "" {
		t.Error("FirstOrCreateByProviderAndInstallationAndTeam() should set ID")
	}
}

func TestRepository_FirstOrCreateByProviderAndInstallationAndTeam_Find(t *testing.T) {
	repo, db := setupTestRepository(t)
	ctx := context.Background()

	existing := createTestSourceControl(t, db, GitProviderGitHub, "team-123", "user-123")

	sc, created, err := repo.FirstOrCreateByProviderAndInstallationAndTeam(
		ctx,
		GitProviderGitHub,
		*existing.ProviderID,
		"team-123",
		map[string]interface{}{},
	)

	if err != nil {
		t.Fatalf("FirstOrCreateByProviderAndInstallationAndTeam() error = %v", err)
	}
	if created {
		t.Error("FirstOrCreateByProviderAndInstallationAndTeam() should have found existing record")
	}
	if sc.ID != existing.ID {
		t.Errorf("FirstOrCreateByProviderAndInstallationAndTeam() ID = %v, want %v", sc.ID, existing.ID)
	}
}

func TestRepository_FindByInstallationID(t *testing.T) {
	repo, db := setupTestRepository(t)
	ctx := context.Background()

	sc := createTestSourceControl(t, db, GitProviderGitHub, "team-123", "user-123")

	found, err := repo.FindByInstallationID(ctx, *sc.ProviderID)
	if err != nil {
		t.Fatalf("FindByInstallationID() error = %v", err)
	}

	if len(found) != 1 {
		t.Errorf("FindByInstallationID() returned %d results, want 1", len(found))
	}
}

func TestRepository_DeleteByInstallationID(t *testing.T) {
	repo, db := setupTestRepository(t)
	ctx := context.Background()

	sc := createTestSourceControl(t, db, GitProviderGitHub, "team-123", "user-123")

	// Add a repository to the source control
	scRepo := &SourceControlRepository{
		SourceControlID: sc.ID,
		Name:            "test-repo",
		FullName:        "user/test-repo",
	}
	db.Create(scRepo)

	count, err := repo.DeleteByInstallationID(ctx, *sc.ProviderID)
	if err != nil {
		t.Fatalf("DeleteByInstallationID() error = %v", err)
	}

	if count != 1 {
		t.Errorf("DeleteByInstallationID() count = %d, want 1", count)
	}

	// Verify repositories are also deleted
	var repoCount int64
	db.Model(&SourceControlRepository{}).Where("source_control_id = ?", sc.ID).Count(&repoCount)
	if repoCount != 0 {
		t.Error("DeleteByInstallationID() should delete associated repositories")
	}
}

func TestRepository_GetInstallations(t *testing.T) {
	repo, db := setupTestRepository(t)
	ctx := context.Background()

	createTestSourceControl(t, db, GitProviderGitHub, "team-123", "user-123")
	createTestSourceControl(t, db, GitProviderGitHub, "team-456", "user-456")

	// Test with user filter
	found, err := repo.GetInstallations(ctx, GitProviderGitHub, WithUserID("user-123"))
	if err != nil {
		t.Fatalf("GetInstallations() error = %v", err)
	}
	if len(found) != 1 {
		t.Errorf("GetInstallations() with user filter returned %d results, want 1", len(found))
	}
}

func TestRepository_GetFirstInstallation(t *testing.T) {
	repo, db := setupTestRepository(t)
	ctx := context.Background()

	sc := createTestSourceControl(t, db, GitProviderGitHub, "team-123", "user-123")

	found, err := repo.GetFirstInstallation(ctx, GitProviderGitHub, WithUserID("user-123"))
	if err != nil {
		t.Fatalf("GetFirstInstallation() error = %v", err)
	}

	if found.ID != sc.ID {
		t.Errorf("GetFirstInstallation() ID = %v, want %v", found.ID, sc.ID)
	}
}

func TestRepository_GetFirstInstallation_NotFound(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	_, err := repo.GetFirstInstallation(ctx, GitProviderGitHub, WithUserID("nonexistent"))
	if err != ErrSourceControlNotFound {
		t.Errorf("GetFirstInstallation() error = %v, want ErrSourceControlNotFound", err)
	}
}

func TestRepository_CreateRepository(t *testing.T) {
	repo, db := setupTestRepository(t)
	ctx := context.Background()

	sc := createTestSourceControl(t, db, GitProviderGitHub, "team-123", "user-123")

	scRepo := &SourceControlRepository{
		SourceControlID: sc.ID,
		Name:            "test-repo",
		FullName:        "user/test-repo",
		Public:          true,
	}

	err := repo.CreateRepository(ctx, scRepo)
	if err != nil {
		t.Fatalf("CreateRepository() error = %v", err)
	}

	if scRepo.ID == "" {
		t.Error("CreateRepository() should set ID")
	}
}

func TestRepository_FindRepositoryByID(t *testing.T) {
	repo, db := setupTestRepository(t)
	ctx := context.Background()

	sc := createTestSourceControl(t, db, GitProviderGitHub, "team-123", "user-123")

	scRepo := &SourceControlRepository{
		SourceControlID: sc.ID,
		Name:            "test-repo",
		FullName:        "user/test-repo",
	}
	db.Create(scRepo)

	found, err := repo.FindRepositoryByID(ctx, scRepo.ID)
	if err != nil {
		t.Fatalf("FindRepositoryByID() error = %v", err)
	}

	if found.ID != scRepo.ID {
		t.Errorf("FindRepositoryByID() ID = %v, want %v", found.ID, scRepo.ID)
	}
}

func TestRepository_FindRepositoryByFullName(t *testing.T) {
	repo, db := setupTestRepository(t)
	ctx := context.Background()

	sc := createTestSourceControl(t, db, GitProviderGitHub, "team-123", "user-123")

	scRepo := &SourceControlRepository{
		SourceControlID: sc.ID,
		Name:            "test-repo",
		FullName:        "user/test-repo",
	}
	db.Create(scRepo)

	found, err := repo.FindRepositoryByFullName(ctx, "user/test-repo")
	if err != nil {
		t.Fatalf("FindRepositoryByFullName() error = %v", err)
	}

	if found.FullName != "user/test-repo" {
		t.Errorf("FindRepositoryByFullName() FullName = %v, want user/test-repo", found.FullName)
	}
}

func TestRepository_FindRepositoriesBySourceControlID(t *testing.T) {
	repo, db := setupTestRepository(t)
	ctx := context.Background()

	sc := createTestSourceControl(t, db, GitProviderGitHub, "team-123", "user-123")

	db.Create(&SourceControlRepository{SourceControlID: sc.ID, Name: "repo1", FullName: "user/repo1"})
	db.Create(&SourceControlRepository{SourceControlID: sc.ID, Name: "repo2", FullName: "user/repo2"})

	found, err := repo.FindRepositoriesBySourceControlID(ctx, sc.ID)
	if err != nil {
		t.Fatalf("FindRepositoriesBySourceControlID() error = %v", err)
	}

	if len(found) != 2 {
		t.Errorf("FindRepositoriesBySourceControlID() returned %d results, want 2", len(found))
	}
}

func TestRepository_UpsertRepository_Create(t *testing.T) {
	repo, db := setupTestRepository(t)
	ctx := context.Background()

	sc := createTestSourceControl(t, db, GitProviderGitHub, "team-123", "user-123")

	data := &RepositoryData{
		Name:          "new-repo",
		FullName:      "user/new-repo",
		IsPublic:      true,
		DefaultBranch: "main",
		HTMLURL:       "https://github.com/user/new-repo",
		SSHURL:        "git@github.com:user/new-repo.git",
	}

	result, err := repo.UpsertRepository(ctx, sc.ID, data)
	if err != nil {
		t.Fatalf("UpsertRepository() error = %v", err)
	}

	if result.ID == "" {
		t.Error("UpsertRepository() should create new repository")
	}
	if result.FullName != "user/new-repo" {
		t.Errorf("UpsertRepository() FullName = %v, want user/new-repo", result.FullName)
	}
}

func TestRepository_UpsertRepository_Update(t *testing.T) {
	repo, db := setupTestRepository(t)
	ctx := context.Background()

	sc := createTestSourceControl(t, db, GitProviderGitHub, "team-123", "user-123")

	existing := &SourceControlRepository{
		SourceControlID: sc.ID,
		Name:            "existing-repo",
		FullName:        "user/existing-repo",
		Public:          false,
	}
	db.Create(existing)

	data := &RepositoryData{
		Name:          "existing-repo",
		FullName:      "user/existing-repo",
		IsPublic:      true,
		DefaultBranch: "develop",
	}

	result, err := repo.UpsertRepository(ctx, sc.ID, data)
	if err != nil {
		t.Fatalf("UpsertRepository() error = %v", err)
	}

	if result.ID != existing.ID {
		t.Errorf("UpsertRepository() should update existing repository")
	}
	if !result.Public {
		t.Error("UpsertRepository() should update Public field")
	}
}

func TestRepository_CountRepositoriesBySourceControlID(t *testing.T) {
	repo, db := setupTestRepository(t)
	ctx := context.Background()

	sc := createTestSourceControl(t, db, GitProviderGitHub, "team-123", "user-123")

	db.Create(&SourceControlRepository{SourceControlID: sc.ID, Name: "repo1", FullName: "user/repo1"})
	db.Create(&SourceControlRepository{SourceControlID: sc.ID, Name: "repo2", FullName: "user/repo2"})
	db.Create(&SourceControlRepository{SourceControlID: sc.ID, Name: "repo3", FullName: "user/repo3"})

	count, err := repo.CountRepositoriesBySourceControlID(ctx, sc.ID)
	if err != nil {
		t.Fatalf("CountRepositoriesBySourceControlID() error = %v", err)
	}

	if count != 3 {
		t.Errorf("CountRepositoriesBySourceControlID() = %d, want 3", count)
	}
}

func TestRepository_DeleteRepositoriesByIDs(t *testing.T) {
	repo, db := setupTestRepository(t)
	ctx := context.Background()

	sc := createTestSourceControl(t, db, GitProviderGitHub, "team-123", "user-123")

	repo1 := &SourceControlRepository{SourceControlID: sc.ID, Name: "repo1", FullName: "user/repo1"}
	repo2 := &SourceControlRepository{SourceControlID: sc.ID, Name: "repo2", FullName: "user/repo2"}
	db.Create(repo1)
	db.Create(repo2)

	err := repo.DeleteRepositoriesByIDs(ctx, []string{repo1.ID})
	if err != nil {
		t.Fatalf("DeleteRepositoriesByIDs() error = %v", err)
	}

	var count int64
	db.Model(&SourceControlRepository{}).Where("source_control_id = ?", sc.ID).Count(&count)
	if count != 1 {
		t.Errorf("DeleteRepositoriesByIDs() should leave 1 repository, got %d", count)
	}
}

func TestRepository_DeleteRepositoriesByIDs_EmptySlice(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	err := repo.DeleteRepositoriesByIDs(ctx, []string{})
	if err != nil {
		t.Fatalf("DeleteRepositoriesByIDs() with empty slice should not error: %v", err)
	}
}

func TestInstallationQueryOptions(t *testing.T) {
	tests := []struct {
		name   string
		opts   []InstallationQueryOption
		verify func(*installationQueryOptions) bool
	}{
		{
			name: "WithUserID",
			opts: []InstallationQueryOption{WithUserID("user-123")},
			verify: func(o *installationQueryOptions) bool {
				return o.userID == "user-123"
			},
		},
		{
			name: "WithProviderID",
			opts: []InstallationQueryOption{WithProviderID("prov-456")},
			verify: func(o *installationQueryOptions) bool {
				return o.providerID == "prov-456"
			},
		},
		{
			name: "RequireInstallationID",
			opts: []InstallationQueryOption{RequireInstallationID()},
			verify: func(o *installationQueryOptions) bool {
				return o.requireInstallationID
			},
		},
		{
			name: "multiple options",
			opts: []InstallationQueryOption{
				WithUserID("user-123"),
				WithProviderID("prov-456"),
				RequireInstallationID(),
			},
			verify: func(o *installationQueryOptions) bool {
				return o.userID == "user-123" &&
					o.providerID == "prov-456" &&
					o.requireInstallationID
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &installationQueryOptions{}
			for _, opt := range tt.opts {
				opt(opts)
			}
			if !tt.verify(opts) {
				t.Errorf("Option verification failed for %s", tt.name)
			}
		})
	}
}
