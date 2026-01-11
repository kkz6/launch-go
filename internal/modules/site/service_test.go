package site

import (
	"context"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/modules/site/services"
	"github.com/kkz6/launch-go/internal/websocket"
)

func setupTestService(t *testing.T) (*services.SiteService, *services.DeploymentService, *repositories.SiteRepository, *repositories.DeploymentRepository, *repositories.QueueRepository, *repositories.CertificateRepository, *repositories.CommandRepository, *repositories.RedirectRepository, *repositories.ReleaseRepository, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.Site{}, &models.Deployment{}, &models.Certificate{}, &models.Queue{}, &models.Command{}, &models.Redirect{}, &models.Release{})
	require.NoError(t, err)

	siteRepo := repositories.NewSiteRepository(db)
	deploymentRepo := repositories.NewDeploymentRepository(db)
	certificateRepo := repositories.NewCertificateRepository(db)
	queueRepo := repositories.NewQueueRepository(db)
	commandRepo := repositories.NewCommandRepository(db)
	redirectRepo := repositories.NewRedirectRepository(db)
	releaseRepo := repositories.NewReleaseRepository(db)

	logger := zerolog.New(os.Stdout)
	ws := websocket.NewHub()

	siteService := services.NewSiteService(
		siteRepo,
		deploymentRepo,
		certificateRepo,
		queueRepo,
		commandRepo,
		redirectRepo,
		releaseRepo,
		nil, // No queue client for testing
		ws,
		&logger,
	)

	deploymentService := services.NewDeploymentService(
		siteRepo,
		deploymentRepo,
		certificateRepo,
		queueRepo,
		commandRepo,
		redirectRepo,
		releaseRepo,
		nil, // No queue client for testing
		ws,
		&logger,
	)

	// Wire services together
	siteService.SetDeploymentService(deploymentService)

	return siteService, deploymentService, siteRepo, deploymentRepo, queueRepo, certificateRepo, commandRepo, redirectRepo, releaseRepo, db
}

func createServiceTestSite(t *testing.T, siteRepo *repositories.SiteRepository, serverID, address string) *models.Site {
	site := &models.Site{
		ServerID:               serverID,
		UserID:                 "01ARZ3NDEKTSV4RRFFQ69G5FAU",
		Address:                address,
		Type:                   enums.SiteTypeLaravel,
		User:                   "deploy",
		Path:                   "/home/deploy/" + address,
		ZeroDowntimeDeployment: true,
	}
	err := siteRepo.Create(context.Background(), site)
	require.NoError(t, err)

	return site
}

func TestNewService(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	siteRepo := repositories.NewSiteRepository(db)
	deploymentRepo := repositories.NewDeploymentRepository(db)
	certificateRepo := repositories.NewCertificateRepository(db)
	queueRepo := repositories.NewQueueRepository(db)
	commandRepo := repositories.NewCommandRepository(db)
	redirectRepo := repositories.NewRedirectRepository(db)
	releaseRepo := repositories.NewReleaseRepository(db)

	logger := zerolog.New(os.Stdout)
	ws := websocket.NewHub()

	service := services.NewSiteService(
		siteRepo,
		deploymentRepo,
		certificateRepo,
		queueRepo,
		commandRepo,
		redirectRepo,
		releaseRepo,
		nil,
		ws,
		&logger,
	)

	assert.NotNil(t, service)
}

func TestService_List(t *testing.T) {
	siteService, _, siteRepo, _, _, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	createServiceTestSite(t, siteRepo, serverID, "site1.com")
	createServiceTestSite(t, siteRepo, serverID, "site2.com")
	createServiceTestSite(t, siteRepo, "other_server", "site3.com")

	sites, err := siteService.List(ctx, serverID)
	require.NoError(t, err)

	assert.Len(t, sites, 2)
}

func TestService_Create(t *testing.T) {
	siteService, _, siteRepo, _, _, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	userID := "01ARZ3NDEKTSV4RRFFQ69G5FAU"
	username := "deploy"

	t.Run("creates site successfully", func(t *testing.T) {
		req := &dto.CreateSiteRequest{
			Address:                "example.com",
			Type:                   "laravel",
			PhpVersion:             "8.3",
			ZeroDowntimeDeployment: true,
		}

		site, err := siteService.Create(ctx, serverID, userID, username, req)
		require.NoError(t, err)

		assert.NotEmpty(t, site.ID)
		assert.Equal(t, "example.com", site.Address)
		assert.Equal(t, enums.SiteTypeLaravel, site.Type)
		assert.Equal(t, "/home/deploy/example.com", site.Path)
	})

	t.Run("creates wordpress site with root web folder", func(t *testing.T) {
		req := &dto.CreateSiteRequest{
			Address:    "wordpress.com",
			Type:       "wordpress",
			PhpVersion: "8.2",
		}

		site, err := siteService.Create(ctx, serverID, userID, username, req)
		require.NoError(t, err)

		assert.Equal(t, enums.SiteTypeWordpress, site.Type)
		assert.Equal(t, "/", site.WebFolder)
	})

	t.Run("returns error for duplicate address", func(t *testing.T) {
		// Create first site
		err := siteRepo.Create(ctx, &models.Site{
			ServerID: serverID,
			UserID:   userID,
			Address:  "duplicate.com",
		})
		require.NoError(t, err)

		req := &dto.CreateSiteRequest{
			Address:    "duplicate.com",
			Type:       "laravel",
			PhpVersion: "8.3",
		}

		_, err = siteService.Create(ctx, serverID, userID, username, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("returns error for invalid type", func(t *testing.T) {
		req := &dto.CreateSiteRequest{
			Address:    "invalid-type.com",
			Type:       "invalid",
			PhpVersion: "8.3",
		}

		_, err := siteService.Create(ctx, serverID, userID, username, req)
		assert.Error(t, err)
	})

	t.Run("sets aliases when provided", func(t *testing.T) {
		req := &dto.CreateSiteRequest{
			Address:    "with-aliases.com",
			Type:       "laravel",
			PhpVersion: "8.3",
			Aliases:    []string{"www.with-aliases.com"},
		}

		site, err := siteService.Create(ctx, serverID, userID, username, req)
		require.NoError(t, err)

		aliases := site.GetAliases()
		assert.Contains(t, aliases, "www.with-aliases.com")
	})
}

func TestService_FindByID(t *testing.T) {
	siteService, _, siteRepo, _, _, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("returns site when found", func(t *testing.T) {
		created := createServiceTestSite(t, siteRepo, serverID, "findbyid.com")

		found, err := siteService.FindByID(ctx, created.ID, serverID)
		require.NoError(t, err)

		assert.Equal(t, created.ID, found.ID)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		_, err := siteService.FindByID(ctx, "non_existent", serverID)

		assert.ErrorIs(t, err, repositories.ErrSiteNotFound)
	})

	t.Run("returns error for wrong server", func(t *testing.T) {
		created := createServiceTestSite(t, siteRepo, serverID, "wrongserver.com")

		_, err := siteService.FindByID(ctx, created.ID, "different_server")

		assert.ErrorIs(t, err, repositories.ErrSiteNotFound)
	})
}

func TestService_Update(t *testing.T) {
	siteService, _, siteRepo, _, _, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	userID := "01ARZ3NDEKTSV4RRFFQ69G5FAU"

	t.Run("updates site successfully", func(t *testing.T) {
		site := createServiceTestSite(t, siteRepo, serverID, "update.com")

		phpVersion := "8.4"
		req := &dto.UpdateSiteRequest{
			PhpVersion: &phpVersion,
		}

		updated, err := siteService.Update(ctx, site.ID, serverID, userID, req)
		require.NoError(t, err)

		assert.Equal(t, "8.4", updated.PhpVersion)
	})

	t.Run("updates web folder", func(t *testing.T) {
		site := createServiceTestSite(t, siteRepo, serverID, "webfolder.com")

		webFolder := "public_html"
		req := &dto.UpdateSiteRequest{
			WebFolder: &webFolder,
		}

		updated, err := siteService.Update(ctx, site.ID, serverID, userID, req)
		require.NoError(t, err)

		assert.Equal(t, "public_html", updated.WebFolder)
	})

	t.Run("updates hooks", func(t *testing.T) {
		site := createServiceTestSite(t, siteRepo, serverID, "hooks.com")

		hookBefore := "echo before"
		hookAfter := "echo after"
		req := &dto.UpdateSiteRequest{
			HookBeforeUpdatingRepository: &hookBefore,
			HookAfterMakingCurrent:       &hookAfter,
		}

		updated, err := siteService.Update(ctx, site.ID, serverID, userID, req)
		require.NoError(t, err)

		assert.Equal(t, "echo before", updated.HookBeforeUpdatingRepository)
		assert.Equal(t, "echo after", updated.HookAfterMakingCurrent)
	})

	t.Run("updates shared directories from multiline", func(t *testing.T) {
		site := createServiceTestSite(t, siteRepo, serverID, "shared.com")

		sharedDirs := "storage\nvendor"
		req := &dto.UpdateSiteRequest{
			SharedDirectories: &sharedDirs,
		}

		updated, err := siteService.Update(ctx, site.ID, serverID, userID, req)
		require.NoError(t, err)

		dirs := updated.GetSharedDirectories()
		assert.Contains(t, dirs, "storage")
		assert.Contains(t, dirs, "vendor")
	})

	t.Run("returns error when not found", func(t *testing.T) {
		phpVersion := "8.4"
		req := &dto.UpdateSiteRequest{
			PhpVersion: &phpVersion,
		}

		_, err := siteService.Update(ctx, "non_existent", serverID, userID, req)
		assert.ErrorIs(t, err, repositories.ErrSiteNotFound)
	})
}

func TestService_Delete(t *testing.T) {
	siteService, _, siteRepo, _, _, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("marks site as uninstalling", func(t *testing.T) {
		site := createServiceTestSite(t, siteRepo, serverID, "delete.com")

		err := siteService.Delete(ctx, site.ID, serverID)
		require.NoError(t, err)

		found, err := siteRepo.FindByID(ctx, site.ID)
		require.NoError(t, err)

		assert.NotNil(t, found.UninstallationRequestedAt)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		err := siteService.Delete(ctx, "non_existent", serverID)
		assert.ErrorIs(t, err, repositories.ErrSiteNotFound)
	})
}

func TestService_Deploy(t *testing.T) {
	_, deploymentService, siteRepo, _, _, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	userID := "01ARZ3NDEKTSV4RRFFQ69G5FAU"

	t.Run("creates deployment successfully", func(t *testing.T) {
		site := createServiceTestSite(t, siteRepo, serverID, "deploy.com")

		deployment, err := deploymentService.Deploy(ctx, site.ID, serverID, userID)
		require.NoError(t, err)

		assert.NotEmpty(t, deployment.ID)
		assert.Equal(t, site.ID, deployment.SiteID)
		assert.Equal(t, enums.DeploymentStatusPending, deployment.Status)
	})

	t.Run("returns error for pending deployment without queue", func(t *testing.T) {
		site := createServiceTestSite(t, siteRepo, serverID, "pending-deploy.com")

		// Create first deployment
		_, err := deploymentService.Deploy(ctx, site.ID, serverID, userID)
		require.NoError(t, err)

		// Try to create second deployment
		_, err = deploymentService.Deploy(ctx, site.ID, serverID, userID)
		assert.ErrorIs(t, err, services.ErrPendingDeployment)
	})

	t.Run("queues deployment when queue enabled", func(t *testing.T) {
		site := createServiceTestSite(t, siteRepo, serverID, "queue-deploy.com")
		site.QueueDeployments = true
		siteRepo.Update(ctx, site)

		// Create first deployment
		_, err := deploymentService.Deploy(ctx, site.ID, serverID, userID)
		require.NoError(t, err)

		// Create second deployment - should be queued
		deployment, err := deploymentService.Deploy(ctx, site.ID, serverID, userID)
		require.NoError(t, err)

		assert.Equal(t, enums.DeploymentStatusQueued, deployment.Status)
	})

	t.Run("returns error when site not found", func(t *testing.T) {
		_, err := deploymentService.Deploy(ctx, "non_existent", serverID, userID)
		assert.ErrorIs(t, err, repositories.ErrSiteNotFound)
	})
}

func TestService_Rollback(t *testing.T) {
	_, deploymentService, siteRepo, deploymentRepo, _, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	userID := "01ARZ3NDEKTSV4RRFFQ69G5FAU"

	t.Run("creates rollback deployment", func(t *testing.T) {
		site := createServiceTestSite(t, siteRepo, serverID, "rollback.com")

		// Create finished deployments
		gitHash := "abc123def456"
		targetDeployment := &models.Deployment{
			SiteID:  site.ID,
			UserID:  &userID,
			Status:  enums.DeploymentStatusFinished,
			GitHash: &gitHash,
		}
		deploymentRepo.Create(ctx, targetDeployment)

		latestDeployment := &models.Deployment{
			SiteID: site.ID,
			UserID: &userID,
			Status: enums.DeploymentStatusFinished,
		}
		deploymentRepo.Create(ctx, latestDeployment)

		deployment, err := deploymentService.Rollback(ctx, site.ID, serverID, targetDeployment.ID, userID)
		require.NoError(t, err)

		assert.NotEmpty(t, deployment.ID)
		assert.Equal(t, enums.DeploymentStatusPending, deployment.Status)
		assert.True(t, deployment.IsRollback())
	})

	t.Run("returns error for non-zero-downtime site", func(t *testing.T) {
		site := createServiceTestSite(t, siteRepo, serverID, "no-rollback.com")
		site.ZeroDowntimeDeployment = false
		siteRepo.Update(ctx, site)

		deployment := &models.Deployment{
			SiteID: site.ID,
			Status: enums.DeploymentStatusFinished,
		}
		deploymentRepo.Create(ctx, deployment)

		_, err := deploymentService.Rollback(ctx, site.ID, serverID, deployment.ID, userID)
		assert.ErrorIs(t, err, services.ErrRollbackNotSupported)
	})

	t.Run("returns error for non-finished deployment", func(t *testing.T) {
		site := createServiceTestSite(t, siteRepo, serverID, "rollback-pending.com")

		deployment := &models.Deployment{
			SiteID: site.ID,
			Status: enums.DeploymentStatusPending,
		}
		deploymentRepo.Create(ctx, deployment)

		_, err := deploymentService.Rollback(ctx, site.ID, serverID, deployment.ID, userID)
		assert.ErrorIs(t, err, services.ErrInvalidRollbackTarget)
	})

	t.Run("returns error when deployment belongs to different site", func(t *testing.T) {
		site1 := createServiceTestSite(t, siteRepo, serverID, "site1-rollback.com")
		site2 := createServiceTestSite(t, siteRepo, serverID, "site2-rollback.com")

		deployment := &models.Deployment{
			SiteID: site2.ID,
			Status: enums.DeploymentStatusFinished,
		}
		deploymentRepo.Create(ctx, deployment)

		_, err := deploymentService.Rollback(ctx, site1.ID, serverID, deployment.ID, userID)
		assert.ErrorIs(t, err, services.ErrDeploymentNotBelongToSite)
	})

	t.Run("returns error with pending deployment", func(t *testing.T) {
		site := createServiceTestSite(t, siteRepo, serverID, "rollback-active.com")

		targetDeployment := &models.Deployment{
			SiteID: site.ID,
			Status: enums.DeploymentStatusFinished,
		}
		deploymentRepo.Create(ctx, targetDeployment)

		// Create active deployment
		activeDeployment := &models.Deployment{
			SiteID: site.ID,
			Status: enums.DeploymentStatusInstalling,
		}
		deploymentRepo.Create(ctx, activeDeployment)

		_, err := deploymentService.Rollback(ctx, site.ID, serverID, targetDeployment.ID, userID)
		assert.ErrorIs(t, err, services.ErrPendingDeployment)
	})
}

func TestService_ListDeployments(t *testing.T) {
	_, deploymentService, siteRepo, deploymentRepo, _, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site := createServiceTestSite(t, siteRepo, serverID, "deployments.com")
	deploymentRepo.Create(ctx, &models.Deployment{SiteID: site.ID, Status: enums.DeploymentStatusFinished})
	deploymentRepo.Create(ctx, &models.Deployment{SiteID: site.ID, Status: enums.DeploymentStatusPending})

	deployments, err := deploymentService.List(ctx, site.ID, serverID)
	require.NoError(t, err)

	assert.Len(t, deployments, 2)
}

func TestService_FindDeployment(t *testing.T) {
	_, deploymentService, siteRepo, deploymentRepo, _, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site := createServiceTestSite(t, siteRepo, serverID, "find-deploy.com")
	deployment := &models.Deployment{SiteID: site.ID, Status: enums.DeploymentStatusPending}
	deploymentRepo.Create(ctx, deployment)

	t.Run("finds deployment", func(t *testing.T) {
		found, err := deploymentService.FindByID(ctx, deployment.ID, site.ID, serverID)
		require.NoError(t, err)

		assert.Equal(t, deployment.ID, found.ID)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		_, err := deploymentService.FindByID(ctx, "non_existent", site.ID, serverID)
		assert.ErrorIs(t, err, repositories.ErrDeploymentNotFound)
	})
}

func TestService_ProcessNextQueuedDeployment(t *testing.T) {
	_, deploymentService, siteRepo, deploymentRepo, _, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("processes next queued deployment", func(t *testing.T) {
		site := createServiceTestSite(t, siteRepo, serverID, "process-queue.com")

		// Create queued deployments
		deploymentRepo.Create(ctx, &models.Deployment{SiteID: site.ID, Status: enums.DeploymentStatusQueued})
		deploymentRepo.Create(ctx, &models.Deployment{SiteID: site.ID, Status: enums.DeploymentStatusQueued})

		deployment, err := deploymentService.ProcessNextQueued(ctx, site.ID)
		require.NoError(t, err)

		assert.NotNil(t, deployment)
		assert.Equal(t, enums.DeploymentStatusPending, deployment.Status)
	})

	t.Run("returns nil when no queued deployments", func(t *testing.T) {
		site := createServiceTestSite(t, siteRepo, serverID, "no-queue.com")

		deployment, err := deploymentService.ProcessNextQueued(ctx, site.ID)
		require.NoError(t, err)

		assert.Nil(t, deployment)
	})

	t.Run("returns nil when active deployment exists", func(t *testing.T) {
		site := createServiceTestSite(t, siteRepo, serverID, "active-queue.com")

		deploymentRepo.Create(ctx, &models.Deployment{SiteID: site.ID, Status: enums.DeploymentStatusInstalling})
		deploymentRepo.Create(ctx, &models.Deployment{SiteID: site.ID, Status: enums.DeploymentStatusQueued})

		deployment, err := deploymentService.ProcessNextQueued(ctx, site.ID)
		require.NoError(t, err)

		assert.Nil(t, deployment)
	})
}

func TestService_GetQueuedDeploymentsCount(t *testing.T) {
	_, deploymentService, siteRepo, deploymentRepo, _, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site := createServiceTestSite(t, siteRepo, serverID, "count-queue.com")
	deploymentRepo.Create(ctx, &models.Deployment{SiteID: site.ID, Status: enums.DeploymentStatusQueued})
	deploymentRepo.Create(ctx, &models.Deployment{SiteID: site.ID, Status: enums.DeploymentStatusQueued})

	count, err := deploymentService.GetQueuedCount(ctx, site.ID)
	require.NoError(t, err)

	assert.Equal(t, int64(2), count)
}

func TestService_CancelQueuedDeployments(t *testing.T) {
	_, deploymentService, siteRepo, deploymentRepo, _, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site := createServiceTestSite(t, siteRepo, serverID, "cancel-queue.com")
	deploymentRepo.Create(ctx, &models.Deployment{SiteID: site.ID, Status: enums.DeploymentStatusQueued})
	deploymentRepo.Create(ctx, &models.Deployment{SiteID: site.ID, Status: enums.DeploymentStatusQueued})

	count, err := deploymentService.CancelQueued(ctx, site.ID, serverID)
	require.NoError(t, err)

	assert.Equal(t, int64(2), count)

	queuedCount, _ := deploymentService.GetQueuedCount(ctx, site.ID)
	assert.Equal(t, int64(0), queuedCount)
}

func TestService_EnableAutoDeployment(t *testing.T) {
	_, deploymentService, siteRepo, _, _, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("enables auto deployment when source control connected", func(t *testing.T) {
		site := createServiceTestSite(t, siteRepo, serverID, "auto-deploy.com")
		scID := "01ARZ3NDEKTSV4RRFFQ69G5FAW"
		repoID := "01ARZ3NDEKTSV4RRFFQ69G5FAX"
		site.SourceControlID = &scID
		site.SourceControlRepositoriesID = &repoID
		siteRepo.Update(ctx, site)

		err := deploymentService.EnableAutoDeployment(ctx, site.ID, serverID)
		require.NoError(t, err)

		found, _ := siteRepo.FindByID(ctx, site.ID)
		assert.True(t, found.AutoDeployment)
	})

	t.Run("returns error when source control not connected", func(t *testing.T) {
		site := createServiceTestSite(t, siteRepo, serverID, "no-sc.com")

		err := deploymentService.EnableAutoDeployment(ctx, site.ID, serverID)
		assert.ErrorIs(t, err, services.ErrSourceControlNotConnected)
	})
}

func TestService_DisableAutoDeployment(t *testing.T) {
	_, deploymentService, siteRepo, _, _, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site := createServiceTestSite(t, siteRepo, serverID, "disable-auto.com")
	site.AutoDeployment = true
	siteRepo.Update(ctx, site)

	err := deploymentService.DisableAutoDeployment(ctx, site.ID, serverID)
	require.NoError(t, err)

	found, _ := siteRepo.FindByID(ctx, site.ID)
	assert.False(t, found.AutoDeployment)
}

func TestService_RegenerateDeployToken(t *testing.T) {
	siteService, _, siteRepo, _, _, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site := createServiceTestSite(t, siteRepo, serverID, "regen-token.com")
	oldToken := site.DeployToken

	err := siteService.RegenerateDeployToken(ctx, site.ID, serverID)
	require.NoError(t, err)

	found, _ := siteRepo.FindByID(ctx, site.ID)
	assert.NotEqual(t, oldToken, found.DeployToken)
}

func TestService_GetDeletionSummary(t *testing.T) {
	siteService, _, siteRepo, _, queueRepo, _, _, _, _, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site := createServiceTestSite(t, siteRepo, serverID, "summary.com")
	queueRepo.Create(ctx, &models.Queue{SiteID: site.ID, ServerID: serverID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU"})
	queueRepo.Create(ctx, &models.Queue{SiteID: site.ID, ServerID: serverID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU"})

	summary, err := siteService.GetDeletionSummary(ctx, site.ID, serverID)
	require.NoError(t, err)

	assert.Equal(t, 2, summary.Queues)
}

func TestService_BroadcastDeploymentProgress(t *testing.T) {
	_, deploymentService, _, _, _, _, _, _, _, _ := setupTestService(t)

	// Should not panic
	deploymentService.BroadcastProgress("site_id", "deployment_id", "installing", "Installing dependencies")
}
