package site

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
)

func setupTestRepository(t *testing.T) (*repositories.SiteRepository, *repositories.DeploymentRepository, *repositories.CertificateRepository, *repositories.QueueRepository, *repositories.CommandRepository, *repositories.RedirectRepository, *repositories.ReleaseRepository, *gorm.DB) {
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

	return siteRepo, deploymentRepo, certificateRepo, queueRepo, commandRepo, redirectRepo, releaseRepo, db
}

func createTestSite(t *testing.T, siteRepo *repositories.SiteRepository, serverID, address string) *models.Site {
	site := &models.Site{
		ServerID: serverID,
		UserID:   "01ARZ3NDEKTSV4RRFFQ69G5FAU",
		Address:  address,
		Type:     enums.SiteTypeLaravel,
		User:     "deploy",
		Path:     "/home/deploy/" + address,
	}
	err := siteRepo.Create(context.Background(), site)
	require.NoError(t, err)

	return site
}

func createTestDeployment(t *testing.T, deploymentRepo *repositories.DeploymentRepository, siteID string, status enums.DeploymentStatus) *models.Deployment {
	userID := "01ARZ3NDEKTSV4RRFFQ69G5FAU"
	deployment := &models.Deployment{
		SiteID: siteID,
		UserID: &userID,
		Status: status,
	}
	err := deploymentRepo.Create(context.Background(), deployment)
	require.NoError(t, err)

	return deployment
}

func TestNewRepository(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	repo := repositories.NewSiteRepository(db)

	assert.NotNil(t, repo)
}

// Site repository tests

func TestRepository_Create(t *testing.T) {
	siteRepo, _, _, _, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := &models.Site{
		ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		UserID:   "01ARZ3NDEKTSV4RRFFQ69G5FAU",
		Address:  "example.com",
	}

	err := siteRepo.Create(ctx, site)
	require.NoError(t, err)

	assert.NotEmpty(t, site.ID)
	assert.NotZero(t, site.CreatedAt)
	assert.NotZero(t, site.UpdatedAt)
}

func TestRepository_FindByID(t *testing.T) {
	siteRepo, _, _, _, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	t.Run("finds existing site", func(t *testing.T) {
		created := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

		found, err := siteRepo.FindByID(ctx, created.ID)
		require.NoError(t, err)

		assert.Equal(t, created.ID, found.ID)
		assert.Equal(t, created.Address, found.Address)
	})

	t.Run("returns error for non-existent site", func(t *testing.T) {
		_, err := siteRepo.FindByID(ctx, "non_existent_id")

		assert.ErrorIs(t, err, repositories.ErrSiteNotFound)
	})
}

func TestRepository_FindByIDWithDeployments(t *testing.T) {
	siteRepo, deploymentRepo, _, _, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	createTestDeployment(t, deploymentRepo, site.ID, enums.DeploymentStatusFinished)
	createTestDeployment(t, deploymentRepo, site.ID, enums.DeploymentStatusPending)

	found, err := siteRepo.FindByIDWithDeployments(ctx, site.ID)
	require.NoError(t, err)

	assert.Equal(t, site.ID, found.ID)
	assert.Len(t, found.Deployments, 2)
}

func TestRepository_FindByIDAndServer(t *testing.T) {
	siteRepo, _, _, _, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	t.Run("finds site by ID and server", func(t *testing.T) {
		serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
		created := createTestSite(t, siteRepo, serverID, "example.com")

		found, err := siteRepo.FindByIDAndServer(ctx, created.ID, serverID)
		require.NoError(t, err)

		assert.Equal(t, created.ID, found.ID)
	})

	t.Run("returns error for wrong server", func(t *testing.T) {
		created := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example2.com")

		_, err := siteRepo.FindByIDAndServer(ctx, created.ID, "different_server")

		assert.ErrorIs(t, err, repositories.ErrSiteNotFound)
	})
}

func TestRepository_FindByServer(t *testing.T) {
	siteRepo, _, _, _, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	createTestSite(t, siteRepo, serverID, "site1.com")
	createTestSite(t, siteRepo, serverID, "site2.com")
	createTestSite(t, siteRepo, "other_server", "site3.com")

	sites, err := siteRepo.FindByServer(ctx, serverID)
	require.NoError(t, err)

	assert.Len(t, sites, 2)
}

func TestRepository_FindByServerWithLatestDeployment(t *testing.T) {
	siteRepo, deploymentRepo, _, _, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	site := createTestSite(t, siteRepo, serverID, "example.com")
	createTestDeployment(t, deploymentRepo, site.ID, enums.DeploymentStatusFinished)

	sites, err := siteRepo.FindByServerWithLatestDeployment(ctx, serverID)
	require.NoError(t, err)

	assert.Len(t, sites, 1)
	assert.NotNil(t, sites[0].LatestDeployment)
}

func TestRepository_FindByAddress(t *testing.T) {
	siteRepo, _, _, _, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	t.Run("finds site by address and server", func(t *testing.T) {
		serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
		created := createTestSite(t, siteRepo, serverID, "unique.example.com")

		found, err := siteRepo.FindByAddress(ctx, "unique.example.com", serverID)
		require.NoError(t, err)

		assert.Equal(t, created.ID, found.ID)
	})

	t.Run("returns error for non-existent address", func(t *testing.T) {
		_, err := siteRepo.FindByAddress(ctx, "nonexistent.com", "server_id")

		assert.ErrorIs(t, err, repositories.ErrSiteNotFound)
	})
}

func TestRepository_FindByRepositoryAndBranch(t *testing.T) {
	siteRepo, _, _, _, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site1 := createTestSite(t, siteRepo, serverID, "site1.com")
	site1.RepositoryBranch = "main"
	site1.AutoDeployment = true
	siteRepo.Update(ctx, site1)

	site2 := createTestSite(t, siteRepo, serverID, "site2.com")
	site2.RepositoryBranch = "main"
	site2.AutoDeployment = false
	siteRepo.Update(ctx, site2)

	sites, err := siteRepo.FindByRepositoryAndBranch(ctx, "repo", "main")
	require.NoError(t, err)

	assert.Len(t, sites, 1)
}

func TestRepository_Update(t *testing.T) {
	siteRepo, _, _, _, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	now := time.Now()
	site.InstalledAt = &now

	err := siteRepo.Update(ctx, site)
	require.NoError(t, err)

	found, err := siteRepo.FindByID(ctx, site.ID)
	require.NoError(t, err)

	assert.NotNil(t, found.InstalledAt)
}

func TestRepository_UpdateFields(t *testing.T) {
	siteRepo, _, _, _, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	err := siteRepo.UpdateFields(ctx, site.ID, map[string]interface{}{
		"auto_deployment": true,
	})
	require.NoError(t, err)

	found, err := siteRepo.FindByID(ctx, site.ID)
	require.NoError(t, err)

	assert.True(t, found.AutoDeployment)
}

func TestRepository_Delete(t *testing.T) {
	siteRepo, _, _, _, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "to_delete.com")

	err := siteRepo.Delete(ctx, site.ID)
	require.NoError(t, err)

	_, err = siteRepo.FindByID(ctx, site.ID)
	assert.ErrorIs(t, err, repositories.ErrSiteNotFound)
}

// Deployment repository tests

func TestRepository_CreateDeployment(t *testing.T) {
	siteRepo, deploymentRepo, _, _, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	deployment := &models.Deployment{
		SiteID: site.ID,
		Status: enums.DeploymentStatusPending,
	}

	err := deploymentRepo.Create(ctx, deployment)
	require.NoError(t, err)

	assert.NotEmpty(t, deployment.ID)
	assert.NotZero(t, deployment.CreatedAt)
}

func TestRepository_FindDeploymentByID(t *testing.T) {
	siteRepo, deploymentRepo, _, _, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	t.Run("finds existing deployment", func(t *testing.T) {
		created := createTestDeployment(t, deploymentRepo, site.ID, enums.DeploymentStatusPending)

		found, err := deploymentRepo.FindByID(ctx, created.ID)
		require.NoError(t, err)

		assert.Equal(t, created.ID, found.ID)
	})

	t.Run("returns error for non-existent deployment", func(t *testing.T) {
		_, err := deploymentRepo.FindByID(ctx, "non_existent_id")

		assert.ErrorIs(t, err, repositories.ErrDeploymentNotFound)
	})
}

func TestRepository_FindDeploymentByIDAndSite(t *testing.T) {
	siteRepo, deploymentRepo, _, _, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	deployment := createTestDeployment(t, deploymentRepo, site.ID, enums.DeploymentStatusPending)

	t.Run("finds deployment by ID and site", func(t *testing.T) {
		found, err := deploymentRepo.FindByIDAndSite(ctx, deployment.ID, site.ID)
		require.NoError(t, err)

		assert.Equal(t, deployment.ID, found.ID)
	})

	t.Run("returns error for wrong site", func(t *testing.T) {
		_, err := deploymentRepo.FindByIDAndSite(ctx, deployment.ID, "different_site")

		assert.ErrorIs(t, err, repositories.ErrDeploymentNotFound)
	})
}

func TestRepository_FindDeploymentsBySite(t *testing.T) {
	siteRepo, deploymentRepo, _, _, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	createTestDeployment(t, deploymentRepo, site.ID, enums.DeploymentStatusPending)
	createTestDeployment(t, deploymentRepo, site.ID, enums.DeploymentStatusFinished)

	deployments, err := deploymentRepo.FindBySite(ctx, site.ID)
	require.NoError(t, err)

	assert.Len(t, deployments, 2)
}

func TestRepository_FindLatestDeploymentBySite(t *testing.T) {
	siteRepo, deploymentRepo, _, _, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	t.Run("returns latest deployment", func(t *testing.T) {
		site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
		createTestDeployment(t, deploymentRepo, site.ID, enums.DeploymentStatusFinished)
		time.Sleep(10 * time.Millisecond)
		latest := createTestDeployment(t, deploymentRepo, site.ID, enums.DeploymentStatusPending)

		found, err := deploymentRepo.FindLatestBySite(ctx, site.ID)
		require.NoError(t, err)

		assert.Equal(t, latest.ID, found.ID)
	})

	t.Run("returns nil for site without deployments", func(t *testing.T) {
		site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "empty.com")

		found, err := deploymentRepo.FindLatestBySite(ctx, site.ID)
		require.NoError(t, err)

		assert.Nil(t, found)
	})
}

func TestRepository_FindActiveDeploymentBySite(t *testing.T) {
	siteRepo, deploymentRepo, _, _, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	t.Run("finds pending deployment", func(t *testing.T) {
		site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "site1.com")
		active := createTestDeployment(t, deploymentRepo, site.ID, enums.DeploymentStatusPending)

		found, err := deploymentRepo.FindActiveBySite(ctx, site.ID)
		require.NoError(t, err)

		assert.Equal(t, active.ID, found.ID)
	})

	t.Run("finds installing deployment", func(t *testing.T) {
		site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "site2.com")
		active := createTestDeployment(t, deploymentRepo, site.ID, enums.DeploymentStatusInstalling)

		found, err := deploymentRepo.FindActiveBySite(ctx, site.ID)
		require.NoError(t, err)

		assert.Equal(t, active.ID, found.ID)
	})

	t.Run("returns nil for finished deployments", func(t *testing.T) {
		site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "site3.com")
		createTestDeployment(t, deploymentRepo, site.ID, enums.DeploymentStatusFinished)

		found, err := deploymentRepo.FindActiveBySite(ctx, site.ID)
		require.NoError(t, err)

		assert.Nil(t, found)
	})
}

func TestRepository_FindQueuedDeploymentsBySite(t *testing.T) {
	siteRepo, deploymentRepo, _, _, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	createTestDeployment(t, deploymentRepo, site.ID, enums.DeploymentStatusQueued)
	createTestDeployment(t, deploymentRepo, site.ID, enums.DeploymentStatusQueued)
	createTestDeployment(t, deploymentRepo, site.ID, enums.DeploymentStatusFinished)

	deployments, err := deploymentRepo.FindQueuedBySite(ctx, site.ID)
	require.NoError(t, err)

	assert.Len(t, deployments, 2)
}

func TestRepository_CountQueuedDeploymentsBySite(t *testing.T) {
	siteRepo, deploymentRepo, _, _, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	createTestDeployment(t, deploymentRepo, site.ID, enums.DeploymentStatusQueued)
	createTestDeployment(t, deploymentRepo, site.ID, enums.DeploymentStatusQueued)

	count, err := deploymentRepo.CountQueuedBySite(ctx, site.ID)
	require.NoError(t, err)

	assert.Equal(t, int64(2), count)
}

func TestRepository_UpdateDeployment(t *testing.T) {
	siteRepo, deploymentRepo, _, _, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	deployment := createTestDeployment(t, deploymentRepo, site.ID, enums.DeploymentStatusPending)

	deployment.Status = enums.DeploymentStatusFinished
	err := deploymentRepo.Update(ctx, deployment)
	require.NoError(t, err)

	found, err := deploymentRepo.FindByID(ctx, deployment.ID)
	require.NoError(t, err)

	assert.Equal(t, enums.DeploymentStatusFinished, found.Status)
}

func TestRepository_UpdateDeploymentStatus(t *testing.T) {
	siteRepo, deploymentRepo, _, _, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	deployment := createTestDeployment(t, deploymentRepo, site.ID, enums.DeploymentStatusPending)

	err := deploymentRepo.UpdateStatus(ctx, deployment.ID, enums.DeploymentStatusFailed)
	require.NoError(t, err)

	found, err := deploymentRepo.FindByID(ctx, deployment.ID)
	require.NoError(t, err)

	assert.Equal(t, enums.DeploymentStatusFailed, found.Status)
}

func TestRepository_CancelQueuedDeployments(t *testing.T) {
	siteRepo, deploymentRepo, _, _, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	createTestDeployment(t, deploymentRepo, site.ID, enums.DeploymentStatusQueued)
	createTestDeployment(t, deploymentRepo, site.ID, enums.DeploymentStatusQueued)
	createTestDeployment(t, deploymentRepo, site.ID, enums.DeploymentStatusPending)

	count, err := deploymentRepo.CancelQueued(ctx, site.ID)
	require.NoError(t, err)

	assert.Equal(t, int64(2), count)

	queuedCount, err := deploymentRepo.CountQueuedBySite(ctx, site.ID)
	require.NoError(t, err)

	assert.Equal(t, int64(0), queuedCount)
}

// Certificate repository tests

func TestRepository_CreateCertificate(t *testing.T) {
	siteRepo, _, certificateRepo, _, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	cert := &models.Certificate{
		SiteID: site.ID,
		Type:   enums.CertificateTypeAuto,
	}

	err := certificateRepo.Create(ctx, cert)
	require.NoError(t, err)

	assert.NotEmpty(t, cert.ID)
}

func TestRepository_FindCertificateByID(t *testing.T) {
	siteRepo, _, certificateRepo, _, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	t.Run("finds existing certificate", func(t *testing.T) {
		cert := &models.Certificate{SiteID: site.ID, Type: enums.CertificateTypeAuto}
		certificateRepo.Create(ctx, cert)

		found, err := certificateRepo.FindByID(ctx, cert.ID)
		require.NoError(t, err)

		assert.Equal(t, cert.ID, found.ID)
	})

	t.Run("returns error for non-existent certificate", func(t *testing.T) {
		_, err := certificateRepo.FindByID(ctx, "non_existent_id")

		assert.ErrorIs(t, err, repositories.ErrCertificateNotFound)
	})
}

func TestRepository_FindCertificatesBySite(t *testing.T) {
	siteRepo, _, certificateRepo, _, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	certificateRepo.Create(ctx, &models.Certificate{SiteID: site.ID, Type: enums.CertificateTypeAuto})
	certificateRepo.Create(ctx, &models.Certificate{SiteID: site.ID, Type: enums.CertificateTypeCustom})

	certs, err := certificateRepo.FindBySite(ctx, site.ID)
	require.NoError(t, err)

	assert.Len(t, certs, 2)
}

func TestRepository_FindActiveCertificateBySite(t *testing.T) {
	siteRepo, _, certificateRepo, _, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	t.Run("finds active certificate", func(t *testing.T) {
		cert := &models.Certificate{SiteID: site.ID, Type: enums.CertificateTypeAuto, IsActive: true}
		certificateRepo.Create(ctx, cert)

		found, err := certificateRepo.FindActiveBySite(ctx, site.ID)
		require.NoError(t, err)

		assert.Equal(t, cert.ID, found.ID)
	})

	t.Run("returns nil when no active certificate", func(t *testing.T) {
		site2 := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "site2.com")

		found, err := certificateRepo.FindActiveBySite(ctx, site2.ID)
		require.NoError(t, err)

		assert.Nil(t, found)
	})
}

func TestRepository_UpdateCertificate(t *testing.T) {
	siteRepo, _, certificateRepo, _, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	cert := &models.Certificate{SiteID: site.ID, Type: enums.CertificateTypeAuto}
	certificateRepo.Create(ctx, cert)

	cert.IsActive = true
	err := certificateRepo.Update(ctx, cert)
	require.NoError(t, err)

	found, err := certificateRepo.FindByID(ctx, cert.ID)
	require.NoError(t, err)

	assert.True(t, found.IsActive)
}

func TestRepository_DeleteCertificate(t *testing.T) {
	siteRepo, _, certificateRepo, _, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	cert := &models.Certificate{SiteID: site.ID, Type: enums.CertificateTypeAuto}
	certificateRepo.Create(ctx, cert)

	err := certificateRepo.Delete(ctx, cert.ID)
	require.NoError(t, err)

	_, err = certificateRepo.FindByID(ctx, cert.ID)
	assert.ErrorIs(t, err, repositories.ErrCertificateNotFound)
}

func TestRepository_DeactivateAllCertificates(t *testing.T) {
	siteRepo, _, certificateRepo, _, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	cert1 := &models.Certificate{SiteID: site.ID, Type: enums.CertificateTypeAuto, IsActive: true}
	cert2 := &models.Certificate{SiteID: site.ID, Type: enums.CertificateTypeCustom, IsActive: true}
	certificateRepo.Create(ctx, cert1)
	certificateRepo.Create(ctx, cert2)

	err := certificateRepo.DeactivateAll(ctx, site.ID)
	require.NoError(t, err)

	certs, err := certificateRepo.FindBySite(ctx, site.ID)
	require.NoError(t, err)

	for _, cert := range certs {
		assert.False(t, cert.IsActive)
	}
}

// Queue repository tests

func TestRepository_CreateQueue(t *testing.T) {
	siteRepo, _, _, queueRepo, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	queue := &models.Queue{
		SiteID:   site.ID,
		ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		UserID:   "01ARZ3NDEKTSV4RRFFQ69G5FAU",
	}

	err := queueRepo.Create(ctx, queue)
	require.NoError(t, err)

	assert.NotEmpty(t, queue.ID)
}

func TestRepository_FindQueueByID(t *testing.T) {
	siteRepo, _, _, queueRepo, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	t.Run("finds existing queue", func(t *testing.T) {
		queue := &models.Queue{SiteID: site.ID, ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU"}
		queueRepo.Create(ctx, queue)

		found, err := queueRepo.FindByID(ctx, queue.ID)
		require.NoError(t, err)

		assert.Equal(t, queue.ID, found.ID)
	})

	t.Run("returns error for non-existent queue", func(t *testing.T) {
		_, err := queueRepo.FindByID(ctx, "non_existent_id")

		assert.ErrorIs(t, err, repositories.ErrQueueNotFound)
	})
}

func TestRepository_FindQueueByIDAndSite(t *testing.T) {
	siteRepo, _, _, queueRepo, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	queue := &models.Queue{SiteID: site.ID, ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU"}
	queueRepo.Create(ctx, queue)

	t.Run("finds queue by ID and site", func(t *testing.T) {
		found, err := queueRepo.FindByIDAndSite(ctx, queue.ID, site.ID)
		require.NoError(t, err)

		assert.Equal(t, queue.ID, found.ID)
	})

	t.Run("returns error for wrong site", func(t *testing.T) {
		_, err := queueRepo.FindByIDAndSite(ctx, queue.ID, "different_site")

		assert.ErrorIs(t, err, repositories.ErrQueueNotFound)
	})
}

func TestRepository_FindQueuesBySite(t *testing.T) {
	siteRepo, _, _, queueRepo, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	queueRepo.Create(ctx, &models.Queue{SiteID: site.ID, ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU"})
	queueRepo.Create(ctx, &models.Queue{SiteID: site.ID, ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU"})

	queues, err := queueRepo.FindBySite(ctx, site.ID)
	require.NoError(t, err)

	assert.Len(t, queues, 2)
}

func TestRepository_FindQueuesByServer(t *testing.T) {
	siteRepo, _, _, queueRepo, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	site := createTestSite(t, siteRepo, serverID, "example.com")
	queueRepo.Create(ctx, &models.Queue{SiteID: site.ID, ServerID: serverID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU"})
	queueRepo.Create(ctx, &models.Queue{SiteID: site.ID, ServerID: serverID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU"})

	queues, err := queueRepo.FindByServer(ctx, serverID)
	require.NoError(t, err)

	assert.Len(t, queues, 2)
}

func TestRepository_CountQueuesBySite(t *testing.T) {
	siteRepo, _, _, queueRepo, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	queueRepo.Create(ctx, &models.Queue{SiteID: site.ID, ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU"})
	queueRepo.Create(ctx, &models.Queue{SiteID: site.ID, ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU"})

	count, err := queueRepo.CountBySite(ctx, site.ID)
	require.NoError(t, err)

	assert.Equal(t, int64(2), count)
}

func TestRepository_UpdateQueue(t *testing.T) {
	siteRepo, _, _, queueRepo, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	queue := &models.Queue{SiteID: site.ID, ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU"}
	queueRepo.Create(ctx, queue)

	now := time.Now()
	queue.InstalledAt = &now
	err := queueRepo.Update(ctx, queue)
	require.NoError(t, err)

	found, err := queueRepo.FindByID(ctx, queue.ID)
	require.NoError(t, err)

	assert.NotNil(t, found.InstalledAt)
}

func TestRepository_DeleteQueue(t *testing.T) {
	siteRepo, _, _, queueRepo, _, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	queue := &models.Queue{SiteID: site.ID, ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU"}
	queueRepo.Create(ctx, queue)

	err := queueRepo.Delete(ctx, queue.ID)
	require.NoError(t, err)

	_, err = queueRepo.FindByID(ctx, queue.ID)
	assert.ErrorIs(t, err, repositories.ErrQueueNotFound)
}

// Command repository tests

func TestRepository_CreateCommand(t *testing.T) {
	siteRepo, _, _, _, commandRepo, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	cmd := &models.Command{
		SiteID:  site.ID,
		UserID:  "01ARZ3NDEKTSV4RRFFQ69G5FAU",
		Command: "php artisan migrate",
	}

	err := commandRepo.Create(ctx, cmd)
	require.NoError(t, err)

	assert.NotEmpty(t, cmd.ID)
}

func TestRepository_FindCommandByID(t *testing.T) {
	siteRepo, _, _, _, commandRepo, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	t.Run("finds existing command", func(t *testing.T) {
		cmd := &models.Command{SiteID: site.ID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU", Command: "test"}
		commandRepo.Create(ctx, cmd)

		found, err := commandRepo.FindByID(ctx, cmd.ID)
		require.NoError(t, err)

		assert.Equal(t, cmd.ID, found.ID)
	})

	t.Run("returns error for non-existent command", func(t *testing.T) {
		_, err := commandRepo.FindByID(ctx, "non_existent_id")

		assert.ErrorIs(t, err, repositories.ErrCommandNotFound)
	})
}

func TestRepository_FindCommandsBySite(t *testing.T) {
	siteRepo, _, _, _, commandRepo, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	commandRepo.Create(ctx, &models.Command{SiteID: site.ID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU", Command: "cmd1"})
	commandRepo.Create(ctx, &models.Command{SiteID: site.ID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU", Command: "cmd2"})

	cmds, err := commandRepo.FindBySite(ctx, site.ID)
	require.NoError(t, err)

	assert.Len(t, cmds, 2)
}

func TestRepository_UpdateCommand(t *testing.T) {
	siteRepo, _, _, _, commandRepo, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	cmd := &models.Command{SiteID: site.ID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU", Command: "test"}
	commandRepo.Create(ctx, cmd)

	output := "success"
	exitCode := 0
	cmd.Output = &output
	cmd.ExitCode = &exitCode
	cmd.Status = enums.CommandStatusFinished
	err := commandRepo.Update(ctx, cmd)
	require.NoError(t, err)

	found, err := commandRepo.FindByID(ctx, cmd.ID)
	require.NoError(t, err)

	assert.Equal(t, enums.CommandStatusFinished, found.Status)
	assert.Equal(t, "success", *found.Output)
}

func TestRepository_DeleteCommand(t *testing.T) {
	siteRepo, _, _, _, commandRepo, _, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	cmd := &models.Command{SiteID: site.ID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU", Command: "test"}
	commandRepo.Create(ctx, cmd)

	err := commandRepo.Delete(ctx, cmd.ID)
	require.NoError(t, err)

	_, err = commandRepo.FindByID(ctx, cmd.ID)
	assert.ErrorIs(t, err, repositories.ErrCommandNotFound)
}

// Redirect repository tests

func TestRepository_CreateRedirect(t *testing.T) {
	siteRepo, _, _, _, _, redirectRepo, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	redirect := &models.Redirect{
		SiteID: site.ID,
		UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU",
		Mode:   enums.RedirectModePermanent,
		From:   "/old",
		To:     "/new",
	}

	err := redirectRepo.Create(ctx, redirect)
	require.NoError(t, err)

	assert.NotEmpty(t, redirect.ID)
}

func TestRepository_FindRedirectByID(t *testing.T) {
	siteRepo, _, _, _, _, redirectRepo, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	t.Run("finds existing redirect", func(t *testing.T) {
		redirect := &models.Redirect{SiteID: site.ID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU", Mode: enums.RedirectModePermanent, From: "/a", To: "/b"}
		redirectRepo.Create(ctx, redirect)

		found, err := redirectRepo.FindByID(ctx, redirect.ID)
		require.NoError(t, err)

		assert.Equal(t, redirect.ID, found.ID)
	})

	t.Run("returns error for non-existent redirect", func(t *testing.T) {
		_, err := redirectRepo.FindByID(ctx, "non_existent_id")

		assert.ErrorIs(t, err, repositories.ErrRedirectNotFound)
	})
}

func TestRepository_FindRedirectsBySite(t *testing.T) {
	siteRepo, _, _, _, _, redirectRepo, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	redirectRepo.Create(ctx, &models.Redirect{SiteID: site.ID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU", Mode: enums.RedirectModePermanent, From: "/a", To: "/b"})
	redirectRepo.Create(ctx, &models.Redirect{SiteID: site.ID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU", Mode: enums.RedirectModeTemporary, From: "/c", To: "/d"})

	redirects, err := redirectRepo.FindBySite(ctx, site.ID)
	require.NoError(t, err)

	assert.Len(t, redirects, 2)
}

func TestRepository_UpdateRedirect(t *testing.T) {
	siteRepo, _, _, _, _, redirectRepo, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	redirect := &models.Redirect{SiteID: site.ID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU", Mode: enums.RedirectModePermanent, From: "/a", To: "/b", Status: "pending"}
	redirectRepo.Create(ctx, redirect)

	redirect.Status = "active"
	err := redirectRepo.Update(ctx, redirect)
	require.NoError(t, err)

	found, err := redirectRepo.FindByID(ctx, redirect.ID)
	require.NoError(t, err)

	assert.Equal(t, "active", found.Status)
}

func TestRepository_DeleteRedirect(t *testing.T) {
	siteRepo, _, _, _, _, redirectRepo, _, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	redirect := &models.Redirect{SiteID: site.ID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU", Mode: enums.RedirectModePermanent, From: "/a", To: "/b"}
	redirectRepo.Create(ctx, redirect)

	err := redirectRepo.Delete(ctx, redirect.ID)
	require.NoError(t, err)

	_, err = redirectRepo.FindByID(ctx, redirect.ID)
	assert.ErrorIs(t, err, repositories.ErrRedirectNotFound)
}

// Release repository tests

func TestRepository_CreateRelease(t *testing.T) {
	siteRepo, _, _, _, _, _, releaseRepo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	release := &models.Release{
		SiteID: site.ID,
		Path:   "/home/deploy/example.com/releases/20240101120000",
	}

	err := releaseRepo.Create(ctx, release)
	require.NoError(t, err)

	assert.NotEmpty(t, release.ID)
}

func TestRepository_FindReleaseByID(t *testing.T) {
	siteRepo, _, _, _, _, _, releaseRepo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	t.Run("finds existing release", func(t *testing.T) {
		release := &models.Release{SiteID: site.ID, Path: "/path/to/release"}
		releaseRepo.Create(ctx, release)

		found, err := releaseRepo.FindByID(ctx, release.ID)
		require.NoError(t, err)

		assert.Equal(t, release.ID, found.ID)
	})

	t.Run("returns error for non-existent release", func(t *testing.T) {
		_, err := releaseRepo.FindByID(ctx, "non_existent_id")

		assert.ErrorIs(t, err, repositories.ErrReleaseNotFound)
	})
}

func TestRepository_FindReleasesBySite(t *testing.T) {
	siteRepo, _, _, _, _, _, releaseRepo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	releaseRepo.Create(ctx, &models.Release{SiteID: site.ID, Path: "/path1"})
	releaseRepo.Create(ctx, &models.Release{SiteID: site.ID, Path: "/path2"})

	releases, err := releaseRepo.FindBySite(ctx, site.ID)
	require.NoError(t, err)

	assert.Len(t, releases, 2)
}

func TestRepository_CountReleasesBySite(t *testing.T) {
	siteRepo, _, _, _, _, _, releaseRepo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	releaseRepo.Create(ctx, &models.Release{SiteID: site.ID, Path: "/path1"})
	releaseRepo.Create(ctx, &models.Release{SiteID: site.ID, Path: "/path2"})
	releaseRepo.Create(ctx, &models.Release{SiteID: site.ID, Path: "/path3"})

	count, err := releaseRepo.CountBySite(ctx, site.ID)
	require.NoError(t, err)

	assert.Equal(t, int64(3), count)
}

func TestRepository_DeleteRelease(t *testing.T) {
	siteRepo, _, _, _, _, _, releaseRepo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	release := &models.Release{SiteID: site.ID, Path: "/path"}
	releaseRepo.Create(ctx, release)

	err := releaseRepo.Delete(ctx, release.ID)
	require.NoError(t, err)

	_, err = releaseRepo.FindByID(ctx, release.ID)
	assert.ErrorIs(t, err, repositories.ErrReleaseNotFound)
}

func TestRepository_DeleteOldReleases(t *testing.T) {
	siteRepo, _, _, _, _, _, releaseRepo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, siteRepo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	// Create 5 releases
	for i := 0; i < 5; i++ {
		releaseRepo.Create(ctx, &models.Release{SiteID: site.ID, Path: "/path" + string(rune('0'+i))})
		time.Sleep(10 * time.Millisecond)
	}

	// Keep only 3
	err := releaseRepo.DeleteOld(ctx, site.ID, 3)
	require.NoError(t, err)

	count, err := releaseRepo.CountBySite(ctx, site.ID)
	require.NoError(t, err)

	assert.Equal(t, int64(3), count)
}
