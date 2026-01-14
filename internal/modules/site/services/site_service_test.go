package services

import (
	"context"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/websocket"
)

func strPtr(s string) *string {
	return &s
}

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Migrate all required models
	err = db.AutoMigrate(
		&models.Site{},
		&models.Deployment{},
		&models.Certificate{},
		&models.Queue{},
		&models.Command{},
		&models.Redirect{},
		&models.Release{},
		&servermodels.Server{},
		&servermodels.InstalledService{},
	)
	require.NoError(t, err)

	return db
}

func setupTestService(db *gorm.DB) *SiteService {
	logger := zerolog.New(os.Stdout)
	ws := websocket.NewHub()

	siteRepo := repositories.NewSiteRepository(db)
	deploymentRepo := repositories.NewDeploymentRepository(db)
	certificateRepo := repositories.NewCertificateRepository(db)
	queueRepo := repositories.NewQueueRepository(db)
	commandRepo := repositories.NewCommandRepository(db)
	redirectRepo := repositories.NewRedirectRepository(db)
	releaseRepo := repositories.NewReleaseRepository(db)
	serverRepo := serverrepos.NewRepository(db)

	service := NewSiteService(
		siteRepo,
		deploymentRepo,
		certificateRepo,
		queueRepo,
		commandRepo,
		redirectRepo,
		releaseRepo,
		nil, // queue client
		ws,
		&logger,
	)

	service.SetServerRepository(serverRepo)

	return service
}

func TestSiteService_Create_LaravelSite(t *testing.T) {
	db := setupTestDB(t)
	service := setupTestService(db)

	// Create a test server
	server := &servermodels.Server{
		TeamID:   "team123",
		UserID:   "user123",
		Name:     "test-server",
		Provider: "digitalocean",
		Username: strPtr("testuser"),
	}
	err := db.Create(server).Error
	require.NoError(t, err)

	req := &dto.CreateSiteRequest{
		Type:                   enums.SiteTypeLaravel,
		Address:                "example.com",
		ZeroDowntimeDeployment: true,
		WebFolder:              "public",
		PhpVersion:             "8.2",
	}

	site, err := service.Create(context.Background(), server.ID, "user123", "testuser", req)

	require.NoError(t, err)
	assert.NotNil(t, site)
	assert.Equal(t, "example.com", site.Address)
	assert.Equal(t, enums.SiteTypeLaravel, site.Type)
	assert.Equal(t, "/home/testuser/example.com", site.Path)
	assert.Equal(t, "public", site.WebFolder)
	assert.True(t, site.ZeroDowntimeDeployment)

	// Verify Laravel defaults were applied
	assert.Contains(t, site.SharedDirectories, "storage")
	assert.Contains(t, site.SharedFiles, ".env")
	assert.NotEmpty(t, site.WriteableDirectories)

	// Verify deploy token was generated
	assert.NotNil(t, site.DeployToken)
	assert.Len(t, *site.DeployToken, 32)
}

func TestSiteService_Create_WordPressSite(t *testing.T) {
	db := setupTestDB(t)
	service := setupTestService(db)

	// Create a test server
	server := &servermodels.Server{
		TeamID:   "team123",
		UserID:   "user123",
		Name:     "test-server",
		Provider: "digitalocean",
		Username: strPtr("testuser"),
	}
	err := db.Create(server).Error
	require.NoError(t, err)

	req := &dto.CreateSiteRequest{
		Type:    enums.SiteTypeWordpress,
		Address: "wpsite.com",
	}

	site, err := service.Create(context.Background(), server.ID, "user123", "testuser", req)

	require.NoError(t, err)
	assert.NotNil(t, site)
	assert.Equal(t, enums.SiteTypeWordpress, site.Type)

	// WordPress defaults
	assert.Equal(t, "/", site.WebFolder) // WordPress uses root as web folder
	assert.False(t, site.ZeroDowntimeDeployment) // WordPress doesn't use zero-downtime
}

func TestSiteService_Create_StaticSite(t *testing.T) {
	db := setupTestDB(t)
	service := setupTestService(db)

	// Create a test server
	server := &servermodels.Server{
		TeamID:   "team123",
		UserID:   "user123",
		Name:     "test-server",
		Provider: "digitalocean",
		Username: strPtr("testuser"),
	}
	err := db.Create(server).Error
	require.NoError(t, err)

	req := &dto.CreateSiteRequest{
		Type:                   enums.SiteTypeStatic,
		Address:                "static.com",
		ZeroDowntimeDeployment: true,
	}

	site, err := service.Create(context.Background(), server.ID, "user123", "testuser", req)

	require.NoError(t, err)
	assert.NotNil(t, site)
	assert.Equal(t, enums.SiteTypeStatic, site.Type)
	assert.Equal(t, "public", site.WebFolder) // Static sites use public folder
}

func TestSiteService_Create_DuplicateAddress(t *testing.T) {
	db := setupTestDB(t)
	service := setupTestService(db)

	// Create a test server
	server := &servermodels.Server{
		TeamID:   "team123",
		UserID:   "user123",
		Name:     "test-server",
		Provider: "digitalocean",
		Username: strPtr("testuser"),
	}
	err := db.Create(server).Error
	require.NoError(t, err)

	req := &dto.CreateSiteRequest{
		Type:    enums.SiteTypeLaravel,
		Address: "duplicate.com",
	}

	// Create first site
	_, err = service.Create(context.Background(), server.ID, "user123", "testuser", req)
	require.NoError(t, err)

	// Try to create duplicate
	_, err = service.Create(context.Background(), server.ID, "user123", "testuser", req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestSiteService_Create_InvalidSiteType(t *testing.T) {
	db := setupTestDB(t)
	service := setupTestService(db)

	// Create a test server
	server := &servermodels.Server{
		TeamID:   "team123",
		UserID:   "user123",
		Name:     "test-server",
		Provider: "digitalocean",
		Username: strPtr("testuser"),
	}
	err := db.Create(server).Error
	require.NoError(t, err)

	req := &dto.CreateSiteRequest{
		Type:    enums.SiteType("invalid"),
		Address: "test.com",
	}

	_, err = service.Create(context.Background(), server.ID, "user123", "testuser", req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid site type")
}

func TestSiteService_List(t *testing.T) {
	db := setupTestDB(t)
	service := setupTestService(db)

	// Create a test server
	server := &servermodels.Server{
		TeamID:   "team123",
		UserID:   "user123",
		Name:     "test-server",
		Provider: "digitalocean",
		Username: strPtr("testuser"),
	}
	err := db.Create(server).Error
	require.NoError(t, err)

	// Create multiple sites
	sites := []string{"site1.com", "site2.com", "site3.com"}
	for _, addr := range sites {
		req := &dto.CreateSiteRequest{
			Type:    enums.SiteTypeLaravel,
			Address: addr,
		}
		_, err := service.Create(context.Background(), server.ID, "user123", "testuser", req)
		require.NoError(t, err)
	}

	// List sites
	result, err := service.List(context.Background(), server.ID)

	require.NoError(t, err)
	assert.Len(t, result, 3)
}

func TestSiteService_FindByID(t *testing.T) {
	db := setupTestDB(t)
	service := setupTestService(db)

	// Create a test server
	server := &servermodels.Server{
		TeamID:   "team123",
		UserID:   "user123",
		Name:     "test-server",
		Provider: "digitalocean",
		Username: strPtr("testuser"),
	}
	err := db.Create(server).Error
	require.NoError(t, err)

	// Create a site
	req := &dto.CreateSiteRequest{
		Type:    enums.SiteTypeLaravel,
		Address: "findme.com",
	}
	created, err := service.Create(context.Background(), server.ID, "user123", "testuser", req)
	require.NoError(t, err)

	// Find the site
	found, err := service.FindByID(context.Background(), created.ID, server.ID)

	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, "findme.com", found.Address)
}

func TestSiteService_Update(t *testing.T) {
	db := setupTestDB(t)
	service := setupTestService(db)

	// Create a test server
	server := &servermodels.Server{
		TeamID:   "team123",
		UserID:   "user123",
		Name:     "test-server",
		Provider: "digitalocean",
		Username: strPtr("testuser"),
	}
	err := db.Create(server).Error
	require.NoError(t, err)

	// Create a site
	req := &dto.CreateSiteRequest{
		Type:       enums.SiteTypeLaravel,
		Address:    "update-test.com",
		PhpVersion: "8.1",
	}
	site, err := service.Create(context.Background(), server.ID, "user123", "testuser", req)
	require.NoError(t, err)

	// Update the site
	newPhpVersion := "8.2"
	newWebFolder := "public_html"
	updateReq := &dto.UpdateSiteRequest{
		PhpVersion: &newPhpVersion,
		WebFolder:  &newWebFolder,
	}

	updated, err := service.Update(context.Background(), site.ID, server.ID, "user123", updateReq)

	require.NoError(t, err)
	assert.Equal(t, "8.2", *updated.PhpVersion)
	assert.Equal(t, "public_html", updated.WebFolder)
}

func TestSiteService_Delete(t *testing.T) {
	db := setupTestDB(t)
	service := setupTestService(db)

	// Create a test server
	server := &servermodels.Server{
		TeamID:   "team123",
		UserID:   "user123",
		Name:     "test-server",
		Provider: "digitalocean",
		Username: strPtr("testuser"),
	}
	err := db.Create(server).Error
	require.NoError(t, err)

	// Create a site
	req := &dto.CreateSiteRequest{
		Type:    enums.SiteTypeLaravel,
		Address: "delete-me.com",
	}
	site, err := service.Create(context.Background(), server.ID, "user123", "testuser", req)
	require.NoError(t, err)

	// Delete the site
	err = service.Delete(context.Background(), site.ID, server.ID)

	require.NoError(t, err)

	// Verify site is marked for deletion (not actually deleted yet)
	var found models.Site
	err = db.First(&found, "id = ?", site.ID).Error
	require.NoError(t, err)
	assert.NotNil(t, found.UninstallationRequestedAt)
}

func TestSiteService_RegenerateDeployToken(t *testing.T) {
	db := setupTestDB(t)
	service := setupTestService(db)

	// Create a test server
	server := &servermodels.Server{
		TeamID:   "team123",
		UserID:   "user123",
		Name:     "test-server",
		Provider: "digitalocean",
		Username: strPtr("testuser"),
	}
	err := db.Create(server).Error
	require.NoError(t, err)

	// Create a site
	req := &dto.CreateSiteRequest{
		Type:    enums.SiteTypeLaravel,
		Address: "token-test.com",
	}
	site, err := service.Create(context.Background(), server.ID, "user123", "testuser", req)
	require.NoError(t, err)

	oldToken := *site.DeployToken

	// Regenerate token
	updated, err := service.RegenerateDeployToken(context.Background(), site.ID, server.ID)

	require.NoError(t, err)
	assert.NotNil(t, updated.DeployToken)
	assert.NotEqual(t, oldToken, *updated.DeployToken)
	assert.Len(t, *updated.DeployToken, 32)
}

func TestSiteService_Create_WithAliases(t *testing.T) {
	db := setupTestDB(t)
	service := setupTestService(db)

	// Create a test server
	server := &servermodels.Server{
		TeamID:   "team123",
		UserID:   "user123",
		Name:     "test-server",
		Provider: "digitalocean",
		Username: strPtr("testuser"),
	}
	err := db.Create(server).Error
	require.NoError(t, err)

	req := &dto.CreateSiteRequest{
		Type:    enums.SiteTypeLaravel,
		Address: "primary.com",
		Aliases: []string{"www.primary.com", "alias.primary.com"},
	}

	site, err := service.Create(context.Background(), server.ID, "user123", "testuser", req)

	require.NoError(t, err)
	assert.Len(t, site.Aliases, 2)
	assert.Contains(t, site.Aliases, "www.primary.com")
	assert.Contains(t, site.Aliases, "alias.primary.com")
}

func TestSiteService_Create_LaravelHooks_ZeroDowntime(t *testing.T) {
	db := setupTestDB(t)
	service := setupTestService(db)

	// Create a test server
	server := &servermodels.Server{
		TeamID:   "team123",
		UserID:   "user123",
		Name:     "test-server",
		Provider: "digitalocean",
		Username: strPtr("testuser"),
	}
	err := db.Create(server).Error
	require.NoError(t, err)

	req := &dto.CreateSiteRequest{
		Type:                   enums.SiteTypeLaravel,
		Address:                "laravel-zd.com",
		ZeroDowntimeDeployment: true,
	}

	site, err := service.Create(context.Background(), server.ID, "user123", "testuser", req)

	require.NoError(t, err)

	// For zero-downtime, hook_before_making_current should have the install script
	assert.NotNil(t, site.HookBeforeMakingCurrent)
	assert.Contains(t, *site.HookBeforeMakingCurrent, "composer install")

	// hook_before_updating_repository should be empty
	if site.HookBeforeUpdatingRepository != nil {
		assert.Empty(t, *site.HookBeforeUpdatingRepository)
	}
}

func TestSiteService_Create_LaravelHooks_NoZeroDowntime(t *testing.T) {
	db := setupTestDB(t)
	service := setupTestService(db)

	// Create a test server
	server := &servermodels.Server{
		TeamID:   "team123",
		UserID:   "user123",
		Name:     "test-server",
		Provider: "digitalocean",
		Username: strPtr("testuser"),
	}
	err := db.Create(server).Error
	require.NoError(t, err)

	req := &dto.CreateSiteRequest{
		Type:                   enums.SiteTypeLaravel,
		Address:                "laravel-no-zd.com",
		ZeroDowntimeDeployment: false,
	}

	site, err := service.Create(context.Background(), server.ID, "user123", "testuser", req)

	require.NoError(t, err)

	// For non-zero-downtime, hook_before_updating_repository should have "artisan down"
	assert.NotNil(t, site.HookBeforeUpdatingRepository)
	assert.Contains(t, *site.HookBeforeUpdatingRepository, "artisan down")

	// hook_after_updating_repository should have the install script
	assert.NotNil(t, site.HookAfterUpdatingRepository)
	assert.Contains(t, *site.HookAfterUpdatingRepository, "composer install")
}
