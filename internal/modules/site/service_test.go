package site

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/websocket"
)

func setupTestService(t *testing.T) (*Service, *Repository, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&Site{}, &Deployment{}, &Certificate{}, &Queue{}, &Command{}, &Redirect{}, &Release{})
	require.NoError(t, err)

	repo := NewRepository(db)
	logger := zerolog.New(os.Stdout)
	ws := websocket.NewHub()

	// Service without queue client for testing
	service := &Service{
		repo:   repo,
		queue:  nil, // No queue for testing
		ws:     ws,
		logger: &logger,
	}

	return service, repo, db
}

func createServiceTestSite(t *testing.T, repo *Repository, serverID, address string) *Site {
	site := &Site{
		ServerID:               serverID,
		UserID:                 "01ARZ3NDEKTSV4RRFFQ69G5FAU",
		Address:                address,
		Type:                   SiteTypeLaravel,
		User:                   "deploy",
		Path:                   "/home/deploy/" + address,
		ZeroDowntimeDeployment: true,
	}
	err := repo.Create(context.Background(), site)
	require.NoError(t, err)

	return site
}

func TestNewService(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	repo := NewRepository(db)
	logger := zerolog.New(os.Stdout)
	ws := websocket.NewHub()

	service := NewService(repo, nil, ws, &logger)

	assert.NotNil(t, service)
	assert.Equal(t, repo, service.repo)
}

func TestService_List(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	createServiceTestSite(t, repo, serverID, "site1.com")
	createServiceTestSite(t, repo, serverID, "site2.com")
	createServiceTestSite(t, repo, "other_server", "site3.com")

	sites, err := service.List(ctx, serverID)
	require.NoError(t, err)

	assert.Len(t, sites, 2)
}

func TestService_Create(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	userID := "01ARZ3NDEKTSV4RRFFQ69G5FAU"
	username := "deploy"

	t.Run("creates site successfully", func(t *testing.T) {
		req := &CreateSiteRequest{
			Address:                "example.com",
			Type:                   "laravel",
			PhpVersion:             "8.3",
			ZeroDowntimeDeployment: true,
		}

		site, err := service.Create(ctx, serverID, userID, username, req)
		require.NoError(t, err)

		assert.NotEmpty(t, site.ID)
		assert.Equal(t, "example.com", site.Address)
		assert.Equal(t, SiteTypeLaravel, site.Type)
		assert.Equal(t, "/home/deploy/example.com", site.Path)
	})

	t.Run("creates wordpress site with root web folder", func(t *testing.T) {
		req := &CreateSiteRequest{
			Address:    "wordpress.com",
			Type:       "wordpress",
			PhpVersion: "8.2",
		}

		site, err := service.Create(ctx, serverID, userID, username, req)
		require.NoError(t, err)

		assert.Equal(t, SiteTypeWordpress, site.Type)
		assert.Equal(t, "/", site.WebFolder)
	})

	t.Run("returns error for duplicate address", func(t *testing.T) {
		// Create first site
		err := repo.Create(ctx, &Site{
			ServerID: serverID,
			UserID:   userID,
			Address:  "duplicate.com",
		})
		require.NoError(t, err)

		req := &CreateSiteRequest{
			Address:    "duplicate.com",
			Type:       "laravel",
			PhpVersion: "8.3",
		}

		_, err = service.Create(ctx, serverID, userID, username, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("returns error for invalid type", func(t *testing.T) {
		req := &CreateSiteRequest{
			Address:    "invalid-type.com",
			Type:       "invalid",
			PhpVersion: "8.3",
		}

		_, err := service.Create(ctx, serverID, userID, username, req)
		assert.Error(t, err)
	})

	t.Run("sets aliases when provided", func(t *testing.T) {
		req := &CreateSiteRequest{
			Address:    "with-aliases.com",
			Type:       "laravel",
			PhpVersion: "8.3",
			Aliases:    []string{"www.with-aliases.com"},
		}

		site, err := service.Create(ctx, serverID, userID, username, req)
		require.NoError(t, err)

		aliases := site.GetAliases()
		assert.Contains(t, aliases, "www.with-aliases.com")
	})
}

func TestService_FindByID(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("returns site when found", func(t *testing.T) {
		created := createServiceTestSite(t, repo, serverID, "findbyid.com")

		found, err := service.FindByID(ctx, created.ID, serverID)
		require.NoError(t, err)

		assert.Equal(t, created.ID, found.ID)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		_, err := service.FindByID(ctx, "non_existent", serverID)

		assert.ErrorIs(t, err, ErrSiteNotFound)
	})

	t.Run("returns error for wrong server", func(t *testing.T) {
		created := createServiceTestSite(t, repo, serverID, "wrongserver.com")

		_, err := service.FindByID(ctx, created.ID, "different_server")

		assert.ErrorIs(t, err, ErrSiteNotFound)
	})
}

func TestService_Update(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	userID := "01ARZ3NDEKTSV4RRFFQ69G5FAU"

	t.Run("updates site successfully", func(t *testing.T) {
		site := createServiceTestSite(t, repo, serverID, "update.com")

		phpVersion := "8.4"
		req := &UpdateSiteRequest{
			PhpVersion: &phpVersion,
		}

		updated, err := service.Update(ctx, site.ID, serverID, userID, req)
		require.NoError(t, err)

		assert.Equal(t, "8.4", updated.PhpVersion)
	})

	t.Run("updates web folder", func(t *testing.T) {
		site := createServiceTestSite(t, repo, serverID, "webfolder.com")

		webFolder := "public_html"
		req := &UpdateSiteRequest{
			WebFolder: &webFolder,
		}

		updated, err := service.Update(ctx, site.ID, serverID, userID, req)
		require.NoError(t, err)

		assert.Equal(t, "public_html", updated.WebFolder)
	})

	t.Run("updates hooks", func(t *testing.T) {
		site := createServiceTestSite(t, repo, serverID, "hooks.com")

		hookBefore := "echo before"
		hookAfter := "echo after"
		req := &UpdateSiteRequest{
			HookBeforeUpdatingRepository: &hookBefore,
			HookAfterMakingCurrent:       &hookAfter,
		}

		updated, err := service.Update(ctx, site.ID, serverID, userID, req)
		require.NoError(t, err)

		assert.Equal(t, "echo before", updated.HookBeforeUpdatingRepository)
		assert.Equal(t, "echo after", updated.HookAfterMakingCurrent)
	})

	t.Run("updates shared directories from multiline", func(t *testing.T) {
		site := createServiceTestSite(t, repo, serverID, "shared.com")

		sharedDirs := "storage\nvendor"
		req := &UpdateSiteRequest{
			SharedDirectories: &sharedDirs,
		}

		updated, err := service.Update(ctx, site.ID, serverID, userID, req)
		require.NoError(t, err)

		dirs := updated.GetSharedDirectories()
		assert.Contains(t, dirs, "storage")
		assert.Contains(t, dirs, "vendor")
	})

	t.Run("returns error when not found", func(t *testing.T) {
		phpVersion := "8.4"
		req := &UpdateSiteRequest{
			PhpVersion: &phpVersion,
		}

		_, err := service.Update(ctx, "non_existent", serverID, userID, req)
		assert.ErrorIs(t, err, ErrSiteNotFound)
	})
}

func TestService_Delete(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("marks site as uninstalling", func(t *testing.T) {
		site := createServiceTestSite(t, repo, serverID, "delete.com")

		err := service.Delete(ctx, site.ID, serverID)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, site.ID)
		require.NoError(t, err)

		assert.NotNil(t, found.UninstallationRequestedAt)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		err := service.Delete(ctx, "non_existent", serverID)
		assert.ErrorIs(t, err, ErrSiteNotFound)
	})
}

func TestService_Deploy(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	userID := "01ARZ3NDEKTSV4RRFFQ69G5FAU"

	t.Run("creates deployment successfully", func(t *testing.T) {
		site := createServiceTestSite(t, repo, serverID, "deploy.com")

		deployment, err := service.Deploy(ctx, site.ID, serverID, userID)
		require.NoError(t, err)

		assert.NotEmpty(t, deployment.ID)
		assert.Equal(t, site.ID, deployment.SiteID)
		assert.Equal(t, DeploymentStatusPending, deployment.Status)
	})

	t.Run("returns error for pending deployment without queue", func(t *testing.T) {
		site := createServiceTestSite(t, repo, serverID, "pending-deploy.com")

		// Create first deployment
		_, err := service.Deploy(ctx, site.ID, serverID, userID)
		require.NoError(t, err)

		// Try to create second deployment
		_, err = service.Deploy(ctx, site.ID, serverID, userID)
		assert.ErrorIs(t, err, ErrPendingDeployment)
	})

	t.Run("queues deployment when queue enabled", func(t *testing.T) {
		site := createServiceTestSite(t, repo, serverID, "queue-deploy.com")
		site.QueueDeployments = true
		repo.Update(ctx, site)

		// Create first deployment
		_, err := service.Deploy(ctx, site.ID, serverID, userID)
		require.NoError(t, err)

		// Create second deployment - should be queued
		deployment, err := service.Deploy(ctx, site.ID, serverID, userID)
		require.NoError(t, err)

		assert.Equal(t, DeploymentStatusQueued, deployment.Status)
	})

	t.Run("returns error when site not found", func(t *testing.T) {
		_, err := service.Deploy(ctx, "non_existent", serverID, userID)
		assert.ErrorIs(t, err, ErrSiteNotFound)
	})
}

func TestService_Rollback(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	userID := "01ARZ3NDEKTSV4RRFFQ69G5FAU"

	t.Run("creates rollback deployment", func(t *testing.T) {
		site := createServiceTestSite(t, repo, serverID, "rollback.com")

		// Create finished deployments
		gitHash := "abc123def456"
		targetDeployment := &Deployment{
			SiteID:  site.ID,
			UserID:  &userID,
			Status:  DeploymentStatusFinished,
			GitHash: &gitHash,
		}
		repo.CreateDeployment(ctx, targetDeployment)

		latestDeployment := &Deployment{
			SiteID: site.ID,
			UserID: &userID,
			Status: DeploymentStatusFinished,
		}
		repo.CreateDeployment(ctx, latestDeployment)

		deployment, err := service.Rollback(ctx, site.ID, serverID, targetDeployment.ID, userID)
		require.NoError(t, err)

		assert.NotEmpty(t, deployment.ID)
		assert.Equal(t, DeploymentStatusPending, deployment.Status)
		assert.True(t, deployment.IsRollback())
	})

	t.Run("returns error for non-zero-downtime site", func(t *testing.T) {
		site := createServiceTestSite(t, repo, serverID, "no-rollback.com")
		site.ZeroDowntimeDeployment = false
		repo.Update(ctx, site)

		deployment := &Deployment{
			SiteID: site.ID,
			Status: DeploymentStatusFinished,
		}
		repo.CreateDeployment(ctx, deployment)

		_, err := service.Rollback(ctx, site.ID, serverID, deployment.ID, userID)
		assert.ErrorIs(t, err, ErrRollbackNotSupported)
	})

	t.Run("returns error for non-finished deployment", func(t *testing.T) {
		site := createServiceTestSite(t, repo, serverID, "rollback-pending.com")

		deployment := &Deployment{
			SiteID: site.ID,
			Status: DeploymentStatusPending,
		}
		repo.CreateDeployment(ctx, deployment)

		_, err := service.Rollback(ctx, site.ID, serverID, deployment.ID, userID)
		assert.ErrorIs(t, err, ErrInvalidRollbackTarget)
	})

	t.Run("returns error when deployment belongs to different site", func(t *testing.T) {
		site1 := createServiceTestSite(t, repo, serverID, "site1-rollback.com")
		site2 := createServiceTestSite(t, repo, serverID, "site2-rollback.com")

		deployment := &Deployment{
			SiteID: site2.ID,
			Status: DeploymentStatusFinished,
		}
		repo.CreateDeployment(ctx, deployment)

		_, err := service.Rollback(ctx, site1.ID, serverID, deployment.ID, userID)
		assert.ErrorIs(t, err, ErrDeploymentNotBelongToSite)
	})

	t.Run("returns error with pending deployment", func(t *testing.T) {
		site := createServiceTestSite(t, repo, serverID, "rollback-active.com")

		targetDeployment := &Deployment{
			SiteID: site.ID,
			Status: DeploymentStatusFinished,
		}
		repo.CreateDeployment(ctx, targetDeployment)

		// Create active deployment
		activeDeployment := &Deployment{
			SiteID: site.ID,
			Status: DeploymentStatusInstalling,
		}
		repo.CreateDeployment(ctx, activeDeployment)

		_, err := service.Rollback(ctx, site.ID, serverID, targetDeployment.ID, userID)
		assert.ErrorIs(t, err, ErrPendingDeployment)
	})
}

func TestService_ListDeployments(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site := createServiceTestSite(t, repo, serverID, "deployments.com")
	repo.CreateDeployment(ctx, &Deployment{SiteID: site.ID, Status: DeploymentStatusFinished})
	repo.CreateDeployment(ctx, &Deployment{SiteID: site.ID, Status: DeploymentStatusPending})

	deployments, err := service.ListDeployments(ctx, site.ID, serverID)
	require.NoError(t, err)

	assert.Len(t, deployments, 2)
}

func TestService_FindDeployment(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site := createServiceTestSite(t, repo, serverID, "find-deploy.com")
	deployment := &Deployment{SiteID: site.ID, Status: DeploymentStatusPending}
	repo.CreateDeployment(ctx, deployment)

	t.Run("finds deployment", func(t *testing.T) {
		found, err := service.FindDeployment(ctx, deployment.ID, site.ID, serverID)
		require.NoError(t, err)

		assert.Equal(t, deployment.ID, found.ID)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		_, err := service.FindDeployment(ctx, "non_existent", site.ID, serverID)
		assert.ErrorIs(t, err, ErrDeploymentNotFound)
	})
}

func TestService_ProcessNextQueuedDeployment(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("processes next queued deployment", func(t *testing.T) {
		site := createServiceTestSite(t, repo, serverID, "process-queue.com")

		// Create queued deployments
		repo.CreateDeployment(ctx, &Deployment{SiteID: site.ID, Status: DeploymentStatusQueued})
		repo.CreateDeployment(ctx, &Deployment{SiteID: site.ID, Status: DeploymentStatusQueued})

		deployment, err := service.ProcessNextQueuedDeployment(ctx, site.ID)
		require.NoError(t, err)

		assert.NotNil(t, deployment)
		assert.Equal(t, DeploymentStatusPending, deployment.Status)
	})

	t.Run("returns nil when no queued deployments", func(t *testing.T) {
		site := createServiceTestSite(t, repo, serverID, "no-queue.com")

		deployment, err := service.ProcessNextQueuedDeployment(ctx, site.ID)
		require.NoError(t, err)

		assert.Nil(t, deployment)
	})

	t.Run("returns nil when active deployment exists", func(t *testing.T) {
		site := createServiceTestSite(t, repo, serverID, "active-queue.com")

		repo.CreateDeployment(ctx, &Deployment{SiteID: site.ID, Status: DeploymentStatusInstalling})
		repo.CreateDeployment(ctx, &Deployment{SiteID: site.ID, Status: DeploymentStatusQueued})

		deployment, err := service.ProcessNextQueuedDeployment(ctx, site.ID)
		require.NoError(t, err)

		assert.Nil(t, deployment)
	})
}

func TestService_GetQueuedDeploymentsCount(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site := createServiceTestSite(t, repo, serverID, "count-queue.com")
	repo.CreateDeployment(ctx, &Deployment{SiteID: site.ID, Status: DeploymentStatusQueued})
	repo.CreateDeployment(ctx, &Deployment{SiteID: site.ID, Status: DeploymentStatusQueued})

	count, err := service.GetQueuedDeploymentsCount(ctx, site.ID)
	require.NoError(t, err)

	assert.Equal(t, int64(2), count)
}

func TestService_CancelQueuedDeployments(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site := createServiceTestSite(t, repo, serverID, "cancel-queue.com")
	repo.CreateDeployment(ctx, &Deployment{SiteID: site.ID, Status: DeploymentStatusQueued})
	repo.CreateDeployment(ctx, &Deployment{SiteID: site.ID, Status: DeploymentStatusQueued})

	count, err := service.CancelQueuedDeployments(ctx, site.ID, serverID)
	require.NoError(t, err)

	assert.Equal(t, int64(2), count)

	queuedCount, _ := service.GetQueuedDeploymentsCount(ctx, site.ID)
	assert.Equal(t, int64(0), queuedCount)
}

func TestService_EnableAutoDeployment(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("enables auto deployment when source control connected", func(t *testing.T) {
		site := createServiceTestSite(t, repo, serverID, "auto-deploy.com")
		scID := "01ARZ3NDEKTSV4RRFFQ69G5FAW"
		repoID := "01ARZ3NDEKTSV4RRFFQ69G5FAX"
		site.SourceControlID = &scID
		site.SourceControlRepositoriesID = &repoID
		repo.Update(ctx, site)

		err := service.EnableAutoDeployment(ctx, site.ID, serverID)
		require.NoError(t, err)

		found, _ := repo.FindByID(ctx, site.ID)
		assert.True(t, found.AutoDeployment)
	})

	t.Run("returns error when source control not connected", func(t *testing.T) {
		site := createServiceTestSite(t, repo, serverID, "no-sc.com")

		err := service.EnableAutoDeployment(ctx, site.ID, serverID)
		assert.ErrorIs(t, err, ErrSourceControlNotConnected)
	})
}

func TestService_DisableAutoDeployment(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site := createServiceTestSite(t, repo, serverID, "disable-auto.com")
	site.AutoDeployment = true
	repo.Update(ctx, site)

	err := service.DisableAutoDeployment(ctx, site.ID, serverID)
	require.NoError(t, err)

	found, _ := repo.FindByID(ctx, site.ID)
	assert.False(t, found.AutoDeployment)
}

func TestService_EnableAutoRestartQueue(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site := createServiceTestSite(t, repo, serverID, "enable-restart.com")

	err := service.EnableAutoRestartQueue(ctx, site.ID, serverID)
	require.NoError(t, err)

	found, _ := repo.FindByID(ctx, site.ID)
	assert.True(t, found.AutoRestartQueue)
}

func TestService_DisableAutoRestartQueue(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site := createServiceTestSite(t, repo, serverID, "disable-restart.com")
	site.AutoRestartQueue = true
	repo.Update(ctx, site)

	err := service.DisableAutoRestartQueue(ctx, site.ID, serverID)
	require.NoError(t, err)

	found, _ := repo.FindByID(ctx, site.ID)
	assert.False(t, found.AutoRestartQueue)
}

func TestService_RegenerateDeployToken(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site := createServiceTestSite(t, repo, serverID, "regen-token.com")
	oldToken := site.DeployToken

	err := service.RegenerateDeployToken(ctx, site.ID, serverID)
	require.NoError(t, err)

	found, _ := repo.FindByID(ctx, site.ID)
	assert.NotEqual(t, oldToken, found.DeployToken)
}

func TestService_GetDeletionSummary(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site := createServiceTestSite(t, repo, serverID, "summary.com")
	repo.CreateQueue(ctx, &Queue{SiteID: site.ID, ServerID: serverID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU"})
	repo.CreateQueue(ctx, &Queue{SiteID: site.ID, ServerID: serverID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU"})

	summary, err := service.GetDeletionSummary(ctx, site.ID, serverID)
	require.NoError(t, err)

	assert.Equal(t, 2, summary.Queues)
}

func TestService_UpdateSSL(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	userID := "01ARZ3NDEKTSV4RRFFQ69G5FAU"

	t.Run("updates TLS setting", func(t *testing.T) {
		site := createServiceTestSite(t, repo, serverID, "ssl.com")

		req := &UpdateSSLRequest{
			TlsSetting: "off",
		}

		err := service.UpdateSSL(ctx, site.ID, serverID, userID, req)
		require.NoError(t, err)

		found, _ := repo.FindByID(ctx, site.ID)
		assert.Equal(t, TlsSettingOff, found.TlsSetting)
	})

	t.Run("creates custom certificate", func(t *testing.T) {
		site := createServiceTestSite(t, repo, serverID, "custom-ssl.com")

		privateKey := "-----BEGIN PRIVATE KEY-----\n..."
		cert := "-----BEGIN CERTIFICATE-----\n..."
		req := &UpdateSSLRequest{
			TlsSetting:  "custom",
			PrivateKey:  &privateKey,
			Certificate: &cert,
		}

		err := service.UpdateSSL(ctx, site.ID, serverID, userID, req)
		require.NoError(t, err)

		certs, _ := repo.FindCertificatesBySite(ctx, site.ID)
		assert.Len(t, certs, 1)
		assert.Equal(t, CertificateTypeCustom, certs[0].Type)
		assert.True(t, certs[0].IsActive)
	})

	t.Run("returns error for invalid TLS setting", func(t *testing.T) {
		site := createServiceTestSite(t, repo, serverID, "invalid-ssl.com")

		req := &UpdateSSLRequest{
			TlsSetting: "invalid",
		}

		err := service.UpdateSSL(ctx, site.ID, serverID, userID, req)
		assert.Error(t, err)
	})
}

func TestService_ListCertificates(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site := createServiceTestSite(t, repo, serverID, "certs.com")
	repo.CreateCertificate(ctx, &Certificate{SiteID: site.ID, Type: CertificateTypeAuto})
	repo.CreateCertificate(ctx, &Certificate{SiteID: site.ID, Type: CertificateTypeCustom})

	certs, err := service.ListCertificates(ctx, site.ID, serverID)
	require.NoError(t, err)

	assert.Len(t, certs, 2)
}

func TestService_CreateQueue(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	userID := "01ARZ3NDEKTSV4RRFFQ69G5FAU"

	t.Run("creates queue successfully", func(t *testing.T) {
		site := createServiceTestSite(t, repo, serverID, "queue.com")

		req := &CreateQueueRequest{
			QueueConnection:       "redis",
			Queue:                 "default",
			RestSecondsOnEmpty:    5,
			MaxSecondsPerJob:      60,
			FailedJobDelaySeconds: 3,
		}

		queue, err := service.CreateQueue(ctx, site.ID, serverID, userID, req)
		require.NoError(t, err)

		assert.NotEmpty(t, queue.ID)
		assert.Equal(t, "redis", queue.QueueConnection)
		assert.Equal(t, "default", queue.QueueName)
	})

	t.Run("uses site user when not specified", func(t *testing.T) {
		site := createServiceTestSite(t, repo, serverID, "queue-user.com")

		req := &CreateQueueRequest{
			QueueConnection:       "database",
			Queue:                 "default",
			RestSecondsOnEmpty:    5,
			MaxSecondsPerJob:      60,
			FailedJobDelaySeconds: 3,
		}

		queue, err := service.CreateQueue(ctx, site.ID, serverID, userID, req)
		require.NoError(t, err)

		assert.Equal(t, site.User, queue.User)
	})
}

func TestService_ListQueues(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site := createServiceTestSite(t, repo, serverID, "list-queues.com")
	repo.CreateQueue(ctx, &Queue{SiteID: site.ID, ServerID: serverID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU"})
	repo.CreateQueue(ctx, &Queue{SiteID: site.ID, ServerID: serverID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU"})

	queues, err := service.ListQueues(ctx, site.ID, serverID)
	require.NoError(t, err)

	assert.Len(t, queues, 2)
}

func TestService_DeleteQueue(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site := createServiceTestSite(t, repo, serverID, "delete-queue.com")
	queue := &Queue{SiteID: site.ID, ServerID: serverID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU"}
	repo.CreateQueue(ctx, queue)

	err := service.DeleteQueue(ctx, queue.ID, site.ID, serverID)
	require.NoError(t, err)

	found, _ := repo.FindQueueByID(ctx, queue.ID)
	assert.NotNil(t, found.UninstallationRequestedAt)
}

func TestService_CreateCommand(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	userID := "01ARZ3NDEKTSV4RRFFQ69G5FAU"

	t.Run("creates command successfully", func(t *testing.T) {
		site := createServiceTestSite(t, repo, serverID, "command.com")
		now := time.Now()
		site.InstalledAt = &now
		repo.Update(ctx, site)

		req := &CreateCommandRequest{
			Command: "php artisan migrate",
		}

		cmd, err := service.CreateCommand(ctx, site.ID, serverID, userID, req)
		require.NoError(t, err)

		assert.NotEmpty(t, cmd.ID)
		assert.Equal(t, "php artisan migrate", cmd.Command)
		assert.Equal(t, CommandStatusPending, cmd.Status)
	})

	t.Run("returns error when site not installed", func(t *testing.T) {
		site := createServiceTestSite(t, repo, serverID, "not-installed.com")

		req := &CreateCommandRequest{
			Command: "php artisan migrate",
		}

		_, err := service.CreateCommand(ctx, site.ID, serverID, userID, req)
		assert.ErrorIs(t, err, ErrSiteNotInstalled)
	})
}

func TestService_ListCommands(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site := createServiceTestSite(t, repo, serverID, "list-cmds.com")
	repo.CreateCommand(ctx, &Command{SiteID: site.ID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU", Command: "cmd1"})
	repo.CreateCommand(ctx, &Command{SiteID: site.ID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU", Command: "cmd2"})

	cmds, err := service.ListCommands(ctx, site.ID, serverID)
	require.NoError(t, err)

	assert.Len(t, cmds, 2)
}

func TestService_CreateRedirect(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	userID := "01ARZ3NDEKTSV4RRFFQ69G5FAU"

	site := createServiceTestSite(t, repo, serverID, "redirect.com")

	req := &CreateRedirectRequest{
		From: "/old",
		To:   "/new",
		Mode: 1,
	}

	redirect, err := service.CreateRedirect(ctx, site.ID, serverID, userID, req)
	require.NoError(t, err)

	assert.NotEmpty(t, redirect.ID)
	assert.Equal(t, "/old", redirect.From)
	assert.Equal(t, "/new", redirect.To)
	assert.Equal(t, RedirectModePermanent, redirect.Mode)
}

func TestService_ListRedirects(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site := createServiceTestSite(t, repo, serverID, "list-redirects.com")
	repo.CreateRedirect(ctx, &Redirect{SiteID: site.ID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU", Mode: RedirectModePermanent, From: "/a", To: "/b"})
	repo.CreateRedirect(ctx, &Redirect{SiteID: site.ID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU", Mode: RedirectModeTemporary, From: "/c", To: "/d"})

	redirects, err := service.ListRedirects(ctx, site.ID, serverID)
	require.NoError(t, err)

	assert.Len(t, redirects, 2)
}

func TestService_DeleteRedirect(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site := createServiceTestSite(t, repo, serverID, "delete-redirect.com")
	redirect := &Redirect{SiteID: site.ID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU", Mode: RedirectModePermanent, From: "/a", To: "/b"}
	repo.CreateRedirect(ctx, redirect)

	err := service.DeleteRedirect(ctx, redirect.ID, site.ID, serverID)
	require.NoError(t, err)

	_, err = repo.FindRedirectByID(ctx, redirect.ID)
	assert.ErrorIs(t, err, ErrRedirectNotFound)
}

func TestService_BroadcastDeploymentProgress(t *testing.T) {
	service, _, _ := setupTestService(t)

	// Should not panic
	service.BroadcastDeploymentProgress("site_id", "deployment_id", "installing", "Installing dependencies")
}

func TestNormalizeLineEndings(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "unix line endings unchanged",
			input:    "line1\nline2\nline3",
			expected: "line1\nline2\nline3",
		},
		{
			name:     "windows line endings converted",
			input:    "line1\r\nline2\r\nline3",
			expected: "line1\nline2\nline3",
		},
		{
			name:     "old mac line endings converted",
			input:    "line1\rline2\rline3",
			expected: "line1\nline2\nline3",
		},
		{
			name:     "mixed line endings",
			input:    "line1\r\nline2\nline3\rline4",
			expected: "line1\nline2\nline3\nline4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeLineEndings(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseMultilineToSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "single line",
			input:    "storage",
			expected: []string{"storage"},
		},
		{
			name:     "multiple lines",
			input:    "storage\nvendor\nnode_modules",
			expected: []string{"storage", "vendor", "node_modules"},
		},
		{
			name:     "lines with whitespace",
			input:    "  storage  \n  vendor  ",
			expected: []string{"storage", "vendor"},
		},
		{
			name:     "empty lines ignored",
			input:    "storage\n\nvendor\n\n",
			expected: []string{"storage", "vendor"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseMultilineToSlice(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
