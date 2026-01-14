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

func setupQueueTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

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

func setupQueueTestService(db *gorm.DB) (*QueueService, *SiteService, *serverrepos.Repository) {
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

	queueService := NewQueueService(
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

	siteService := NewSiteService(
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
	siteService.SetServerRepository(serverRepo)

	return queueService, siteService, serverRepo
}

func queueStrPtr(s string) *string {
	return &s
}

func TestQueueService_Create(t *testing.T) {
	db := setupQueueTestDB(t)
	queueService, siteService, _ := setupQueueTestService(db)

	// Create a test server
	server := &servermodels.Server{
		TeamID:   "team123",
		UserID:   "user123",
		Name:     "test-server",
		Provider: "digitalocean",
		Username: queueStrPtr("testuser"),
	}
	err := db.Create(server).Error
	require.NoError(t, err)

	// Create a site
	siteReq := &dto.CreateSiteRequest{
		Type:       enums.SiteTypeLaravel,
		Address:    "queue-test.com",
		PhpVersion: "8.2",
	}
	site, err := siteService.Create(context.Background(), server.ID, "user123", "testuser", siteReq)
	require.NoError(t, err)

	// Create a queue
	queueReq := &dto.CreateQueueRequest{
		QueueConnection:       "redis",
		Queue:                 "default",
		RestSecondsOnEmpty:    3,
		MaxSecondsPerJob:      60,
		FailedJobDelaySeconds: 10,
	}

	queue, err := queueService.Create(context.Background(), site.ID, server.ID, "user123", queueReq)

	require.NoError(t, err)
	assert.NotNil(t, queue)
	assert.Equal(t, site.ID, queue.SiteID)
	assert.Equal(t, "redis", queue.QueueConnection)
	assert.Equal(t, "default", queue.QueueName)
	assert.Equal(t, 1, queue.NumProcs)
	assert.True(t, queue.AutoStart)
	assert.True(t, queue.AutoRestart)
}

func TestQueueService_List(t *testing.T) {
	db := setupQueueTestDB(t)
	queueService, siteService, _ := setupQueueTestService(db)

	// Create a test server
	server := &servermodels.Server{
		TeamID:   "team123",
		UserID:   "user123",
		Name:     "test-server",
		Provider: "digitalocean",
		Username: queueStrPtr("testuser"),
	}
	err := db.Create(server).Error
	require.NoError(t, err)

	// Create a site
	siteReq := &dto.CreateSiteRequest{
		Type:       enums.SiteTypeLaravel,
		Address:    "list-queue-test.com",
		PhpVersion: "8.2",
	}
	site, err := siteService.Create(context.Background(), server.ID, "user123", "testuser", siteReq)
	require.NoError(t, err)

	// Create multiple queues
	for i := 0; i < 3; i++ {
		queueReq := &dto.CreateQueueRequest{
			QueueConnection:       "redis",
			Queue:                 "queue" + string(rune('A'+i)),
			RestSecondsOnEmpty:    3,
			MaxSecondsPerJob:      60,
			FailedJobDelaySeconds: 10,
		}
		_, err := queueService.Create(context.Background(), site.ID, server.ID, "user123", queueReq)
		require.NoError(t, err)
	}

	// List queues
	queues, err := queueService.List(context.Background(), site.ID, server.ID)

	require.NoError(t, err)
	assert.Len(t, queues, 3)
}

func TestQueueService_Delete(t *testing.T) {
	db := setupQueueTestDB(t)
	queueService, siteService, _ := setupQueueTestService(db)

	// Create a test server
	server := &servermodels.Server{
		TeamID:   "team123",
		UserID:   "user123",
		Name:     "test-server",
		Provider: "digitalocean",
		Username: queueStrPtr("testuser"),
	}
	err := db.Create(server).Error
	require.NoError(t, err)

	// Create a site
	siteReq := &dto.CreateSiteRequest{
		Type:       enums.SiteTypeLaravel,
		Address:    "delete-queue-test.com",
		PhpVersion: "8.2",
	}
	site, err := siteService.Create(context.Background(), server.ID, "user123", "testuser", siteReq)
	require.NoError(t, err)

	// Create a queue
	queueReq := &dto.CreateQueueRequest{
		QueueConnection:       "redis",
		Queue:                 "default",
		RestSecondsOnEmpty:    3,
		MaxSecondsPerJob:      60,
		FailedJobDelaySeconds: 10,
	}
	queue, err := queueService.Create(context.Background(), site.ID, server.ID, "user123", queueReq)
	require.NoError(t, err)

	// Delete the queue
	err = queueService.Delete(context.Background(), queue.ID, site.ID, server.ID)

	require.NoError(t, err)

	// Verify queue is marked for deletion
	var found models.Queue
	err = db.First(&found, "id = ?", queue.ID).Error
	require.NoError(t, err)
	assert.NotNil(t, found.UninstallationRequestedAt)
}

func TestQueueService_EnableAutoRestart(t *testing.T) {
	db := setupQueueTestDB(t)
	queueService, siteService, _ := setupQueueTestService(db)

	// Create a test server
	server := &servermodels.Server{
		TeamID:   "team123",
		UserID:   "user123",
		Name:     "test-server",
		Provider: "digitalocean",
		Username: queueStrPtr("testuser"),
	}
	err := db.Create(server).Error
	require.NoError(t, err)

	// Create a site
	siteReq := &dto.CreateSiteRequest{
		Type:       enums.SiteTypeLaravel,
		Address:    "auto-restart-test.com",
		PhpVersion: "8.2",
	}
	site, err := siteService.Create(context.Background(), server.ID, "user123", "testuser", siteReq)
	require.NoError(t, err)

	// Enable auto-restart
	err = queueService.EnableAutoRestart(context.Background(), site.ID, server.ID)

	require.NoError(t, err)

	// Verify site has auto-restart enabled
	var found models.Site
	err = db.First(&found, "id = ?", site.ID).Error
	require.NoError(t, err)
	assert.True(t, found.AutoRestartQueue)
}

func TestQueueService_DisableAutoRestart(t *testing.T) {
	db := setupQueueTestDB(t)
	queueService, siteService, _ := setupQueueTestService(db)

	// Create a test server
	server := &servermodels.Server{
		TeamID:   "team123",
		UserID:   "user123",
		Name:     "test-server",
		Provider: "digitalocean",
		Username: queueStrPtr("testuser"),
	}
	err := db.Create(server).Error
	require.NoError(t, err)

	// Create a site with auto-restart enabled
	siteReq := &dto.CreateSiteRequest{
		Type:       enums.SiteTypeLaravel,
		Address:    "disable-restart-test.com",
		PhpVersion: "8.2",
	}
	site, err := siteService.Create(context.Background(), server.ID, "user123", "testuser", siteReq)
	require.NoError(t, err)

	// Enable then disable auto-restart
	err = queueService.EnableAutoRestart(context.Background(), site.ID, server.ID)
	require.NoError(t, err)
	err = queueService.DisableAutoRestart(context.Background(), site.ID, server.ID)

	require.NoError(t, err)

	// Verify site has auto-restart disabled
	var found models.Site
	err = db.First(&found, "id = ?", site.ID).Error
	require.NoError(t, err)
	assert.False(t, found.AutoRestartQueue)
}

func TestQueueService_SyncStatus(t *testing.T) {
	db := setupQueueTestDB(t)
	queueService, siteService, _ := setupQueueTestService(db)

	// Create a test server
	server := &servermodels.Server{
		TeamID:   "team123",
		UserID:   "user123",
		Name:     "test-server",
		Provider: "digitalocean",
		Username: queueStrPtr("testuser"),
	}
	err := db.Create(server).Error
	require.NoError(t, err)

	// Create a site
	siteReq := &dto.CreateSiteRequest{
		Type:       enums.SiteTypeLaravel,
		Address:    "sync-test.com",
		PhpVersion: "8.2",
	}
	site, err := siteService.Create(context.Background(), server.ID, "user123", "testuser", siteReq)
	require.NoError(t, err)

	// Create some queues
	for i := 0; i < 2; i++ {
		queueReq := &dto.CreateQueueRequest{
			QueueConnection:       "redis",
			Queue:                 "sync-queue" + string(rune('A'+i)),
			RestSecondsOnEmpty:    3,
			MaxSecondsPerJob:      60,
			FailedJobDelaySeconds: 10,
		}
		_, err := queueService.Create(context.Background(), site.ID, server.ID, "user123", queueReq)
		require.NoError(t, err)
	}

	// Sync status
	err = queueService.SyncStatus(context.Background(), site.ID, server.ID, "user123")

	require.NoError(t, err)

	// Verify last status check was updated for all queues
	queues, err := queueService.List(context.Background(), site.ID, server.ID)
	require.NoError(t, err)
	assert.Len(t, queues, 2)
	for _, q := range queues {
		assert.NotNil(t, q.LastStatusCheck)
	}
}

func TestQueueService_SyncStatus_NoQueues(t *testing.T) {
	db := setupQueueTestDB(t)
	queueService, siteService, _ := setupQueueTestService(db)

	// Create a test server
	server := &servermodels.Server{
		TeamID:   "team123",
		UserID:   "user123",
		Name:     "test-server",
		Provider: "digitalocean",
		Username: queueStrPtr("testuser"),
	}
	err := db.Create(server).Error
	require.NoError(t, err)

	// Create a site without any queues
	siteReq := &dto.CreateSiteRequest{
		Type:       enums.SiteTypeLaravel,
		Address:    "no-queues-test.com",
		PhpVersion: "8.2",
	}
	site, err := siteService.Create(context.Background(), server.ID, "user123", "testuser", siteReq)
	require.NoError(t, err)

	// Sync status (should succeed without error)
	err = queueService.SyncStatus(context.Background(), site.ID, server.ID, "user123")

	require.NoError(t, err)
}

func TestQueueService_SyncStatus_SiteNotFound(t *testing.T) {
	db := setupQueueTestDB(t)
	queueService, _, _ := setupQueueTestService(db)

	// Sync status for non-existent site
	err := queueService.SyncStatus(context.Background(), "nonexistent", "serverid", "user123")

	assert.Error(t, err)
}

func TestQueueService_Create_WithCustomOptions(t *testing.T) {
	db := setupQueueTestDB(t)
	queueService, siteService, _ := setupQueueTestService(db)

	// Create a test server
	server := &servermodels.Server{
		TeamID:   "team123",
		UserID:   "user123",
		Name:     "test-server",
		Provider: "digitalocean",
		Username: queueStrPtr("testuser"),
	}
	err := db.Create(server).Error
	require.NoError(t, err)

	// Create a site
	siteReq := &dto.CreateSiteRequest{
		Type:       enums.SiteTypeLaravel,
		Address:    "custom-queue-test.com",
		PhpVersion: "8.2",
	}
	site, err := siteService.Create(context.Background(), server.ID, "user123", "testuser", siteReq)
	require.NoError(t, err)

	// Create a queue with custom options
	maxTries := 5
	maxMemory := 256
	numProcs := 4
	queueReq := &dto.CreateQueueRequest{
		QueueConnection:       "database",
		Queue:                 "high",
		RestSecondsOnEmpty:    5,
		MaxSecondsPerJob:      120,
		FailedJobDelaySeconds: 30,
		MaxTries:              &maxTries,
		MaxMemory:             &maxMemory,
		NumProcs:              &numProcs,
		RunOnMaintenance:      true,
		RunWithListen:         true,
	}

	queue, err := queueService.Create(context.Background(), site.ID, server.ID, "user123", queueReq)

	require.NoError(t, err)
	assert.NotNil(t, queue)
	assert.Equal(t, "database", queue.QueueConnection)
	assert.Equal(t, "high", queue.QueueName)
	assert.Equal(t, 5, *queue.MaxTries)
	assert.Equal(t, 256, *queue.MaxMemory)
	assert.Equal(t, 4, queue.NumProcs)
	assert.True(t, queue.RunOnMaintenance)
	assert.True(t, queue.RunWithListen)
}
