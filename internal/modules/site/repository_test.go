package site

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestRepository(t *testing.T) (*Repository, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&Site{}, &Deployment{}, &Certificate{}, &Queue{}, &Command{}, &Redirect{}, &Release{})
	require.NoError(t, err)

	return NewRepository(db), db
}

func createTestSite(t *testing.T, repo *Repository, serverID, address string) *Site {
	site := &Site{
		ServerID: serverID,
		UserID:   "01ARZ3NDEKTSV4RRFFQ69G5FAU",
		Address:  address,
		Type:     SiteTypeLaravel,
		User:     "deploy",
		Path:     "/home/deploy/" + address,
	}
	err := repo.Create(context.Background(), site)
	require.NoError(t, err)

	return site
}

func createTestDeployment(t *testing.T, repo *Repository, siteID string, status DeploymentStatus) *Deployment {
	userID := "01ARZ3NDEKTSV4RRFFQ69G5FAU"
	deployment := &Deployment{
		SiteID: siteID,
		UserID: &userID,
		Status: status,
	}
	err := repo.CreateDeployment(context.Background(), deployment)
	require.NoError(t, err)

	return deployment
}

func TestNewRepository(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	repo := NewRepository(db)

	assert.NotNil(t, repo)
	assert.Equal(t, db, repo.db)
}

// Site repository tests

func TestRepository_Create(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := &Site{
		ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		UserID:   "01ARZ3NDEKTSV4RRFFQ69G5FAU",
		Address:  "example.com",
	}

	err := repo.Create(ctx, site)
	require.NoError(t, err)

	assert.NotEmpty(t, site.ID)
	assert.NotZero(t, site.CreatedAt)
	assert.NotZero(t, site.UpdatedAt)
}

func TestRepository_FindByID(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	t.Run("finds existing site", func(t *testing.T) {
		created := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

		found, err := repo.FindByID(ctx, created.ID)
		require.NoError(t, err)

		assert.Equal(t, created.ID, found.ID)
		assert.Equal(t, created.Address, found.Address)
	})

	t.Run("returns error for non-existent site", func(t *testing.T) {
		_, err := repo.FindByID(ctx, "non_existent_id")

		assert.ErrorIs(t, err, ErrSiteNotFound)
	})
}

func TestRepository_FindByIDWithDeployments(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	createTestDeployment(t, repo, site.ID, DeploymentStatusFinished)
	createTestDeployment(t, repo, site.ID, DeploymentStatusPending)

	found, err := repo.FindByIDWithDeployments(ctx, site.ID)
	require.NoError(t, err)

	assert.Equal(t, site.ID, found.ID)
	assert.Len(t, found.Deployments, 2)
}

func TestRepository_FindByIDAndServer(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	t.Run("finds site by ID and server", func(t *testing.T) {
		serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
		created := createTestSite(t, repo, serverID, "example.com")

		found, err := repo.FindByIDAndServer(ctx, created.ID, serverID)
		require.NoError(t, err)

		assert.Equal(t, created.ID, found.ID)
	})

	t.Run("returns error for wrong server", func(t *testing.T) {
		created := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example2.com")

		_, err := repo.FindByIDAndServer(ctx, created.ID, "different_server")

		assert.ErrorIs(t, err, ErrSiteNotFound)
	})
}

func TestRepository_FindByServer(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	createTestSite(t, repo, serverID, "site1.com")
	createTestSite(t, repo, serverID, "site2.com")
	createTestSite(t, repo, "other_server", "site3.com")

	sites, err := repo.FindByServer(ctx, serverID)
	require.NoError(t, err)

	assert.Len(t, sites, 2)
}

func TestRepository_FindByServerWithLatestDeployment(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	site := createTestSite(t, repo, serverID, "example.com")
	createTestDeployment(t, repo, site.ID, DeploymentStatusFinished)

	sites, err := repo.FindByServerWithLatestDeployment(ctx, serverID)
	require.NoError(t, err)

	assert.Len(t, sites, 1)
	assert.NotNil(t, sites[0].LatestDeployment)
}

func TestRepository_FindByAddress(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	t.Run("finds site by address and server", func(t *testing.T) {
		serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
		created := createTestSite(t, repo, serverID, "unique.example.com")

		found, err := repo.FindByAddress(ctx, "unique.example.com", serverID)
		require.NoError(t, err)

		assert.Equal(t, created.ID, found.ID)
	})

	t.Run("returns error for non-existent address", func(t *testing.T) {
		_, err := repo.FindByAddress(ctx, "nonexistent.com", "server_id")

		assert.ErrorIs(t, err, ErrSiteNotFound)
	})
}

func TestRepository_FindByRepositoryAndBranch(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site1 := createTestSite(t, repo, serverID, "site1.com")
	site1.RepositoryBranch = "main"
	site1.AutoDeployment = true
	repo.Update(ctx, site1)

	site2 := createTestSite(t, repo, serverID, "site2.com")
	site2.RepositoryBranch = "main"
	site2.AutoDeployment = false
	repo.Update(ctx, site2)

	sites, err := repo.FindByRepositoryAndBranch(ctx, "repo", "main")
	require.NoError(t, err)

	assert.Len(t, sites, 1)
}

func TestRepository_Update(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	now := time.Now()
	site.InstalledAt = &now

	err := repo.Update(ctx, site)
	require.NoError(t, err)

	found, err := repo.FindByID(ctx, site.ID)
	require.NoError(t, err)

	assert.NotNil(t, found.InstalledAt)
}

func TestRepository_UpdateFields(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	err := repo.UpdateFields(ctx, site.ID, map[string]interface{}{
		"auto_deployment": true,
	})
	require.NoError(t, err)

	found, err := repo.FindByID(ctx, site.ID)
	require.NoError(t, err)

	assert.True(t, found.AutoDeployment)
}

func TestRepository_Delete(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "to_delete.com")

	err := repo.Delete(ctx, site.ID)
	require.NoError(t, err)

	_, err = repo.FindByID(ctx, site.ID)
	assert.ErrorIs(t, err, ErrSiteNotFound)
}

// Deployment repository tests

func TestRepository_CreateDeployment(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	deployment := &Deployment{
		SiteID: site.ID,
		Status: DeploymentStatusPending,
	}

	err := repo.CreateDeployment(ctx, deployment)
	require.NoError(t, err)

	assert.NotEmpty(t, deployment.ID)
	assert.NotZero(t, deployment.CreatedAt)
}

func TestRepository_FindDeploymentByID(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	t.Run("finds existing deployment", func(t *testing.T) {
		created := createTestDeployment(t, repo, site.ID, DeploymentStatusPending)

		found, err := repo.FindDeploymentByID(ctx, created.ID)
		require.NoError(t, err)

		assert.Equal(t, created.ID, found.ID)
	})

	t.Run("returns error for non-existent deployment", func(t *testing.T) {
		_, err := repo.FindDeploymentByID(ctx, "non_existent_id")

		assert.ErrorIs(t, err, ErrDeploymentNotFound)
	})
}

func TestRepository_FindDeploymentByIDAndSite(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	deployment := createTestDeployment(t, repo, site.ID, DeploymentStatusPending)

	t.Run("finds deployment by ID and site", func(t *testing.T) {
		found, err := repo.FindDeploymentByIDAndSite(ctx, deployment.ID, site.ID)
		require.NoError(t, err)

		assert.Equal(t, deployment.ID, found.ID)
	})

	t.Run("returns error for wrong site", func(t *testing.T) {
		_, err := repo.FindDeploymentByIDAndSite(ctx, deployment.ID, "different_site")

		assert.ErrorIs(t, err, ErrDeploymentNotFound)
	})
}

func TestRepository_FindDeploymentsBySite(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	createTestDeployment(t, repo, site.ID, DeploymentStatusPending)
	createTestDeployment(t, repo, site.ID, DeploymentStatusFinished)

	deployments, err := repo.FindDeploymentsBySite(ctx, site.ID)
	require.NoError(t, err)

	assert.Len(t, deployments, 2)
}

func TestRepository_FindLatestDeploymentBySite(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	t.Run("returns latest deployment", func(t *testing.T) {
		site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
		createTestDeployment(t, repo, site.ID, DeploymentStatusFinished)
		time.Sleep(10 * time.Millisecond)
		latest := createTestDeployment(t, repo, site.ID, DeploymentStatusPending)

		found, err := repo.FindLatestDeploymentBySite(ctx, site.ID)
		require.NoError(t, err)

		assert.Equal(t, latest.ID, found.ID)
	})

	t.Run("returns nil for site without deployments", func(t *testing.T) {
		site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "empty.com")

		found, err := repo.FindLatestDeploymentBySite(ctx, site.ID)
		require.NoError(t, err)

		assert.Nil(t, found)
	})
}

func TestRepository_FindActiveDeploymentBySite(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	t.Run("finds pending deployment", func(t *testing.T) {
		site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "site1.com")
		active := createTestDeployment(t, repo, site.ID, DeploymentStatusPending)

		found, err := repo.FindActiveDeploymentBySite(ctx, site.ID)
		require.NoError(t, err)

		assert.Equal(t, active.ID, found.ID)
	})

	t.Run("finds installing deployment", func(t *testing.T) {
		site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "site2.com")
		active := createTestDeployment(t, repo, site.ID, DeploymentStatusInstalling)

		found, err := repo.FindActiveDeploymentBySite(ctx, site.ID)
		require.NoError(t, err)

		assert.Equal(t, active.ID, found.ID)
	})

	t.Run("returns nil for finished deployments", func(t *testing.T) {
		site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "site3.com")
		createTestDeployment(t, repo, site.ID, DeploymentStatusFinished)

		found, err := repo.FindActiveDeploymentBySite(ctx, site.ID)
		require.NoError(t, err)

		assert.Nil(t, found)
	})
}

func TestRepository_FindQueuedDeploymentsBySite(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	createTestDeployment(t, repo, site.ID, DeploymentStatusQueued)
	createTestDeployment(t, repo, site.ID, DeploymentStatusQueued)
	createTestDeployment(t, repo, site.ID, DeploymentStatusFinished)

	deployments, err := repo.FindQueuedDeploymentsBySite(ctx, site.ID)
	require.NoError(t, err)

	assert.Len(t, deployments, 2)
}

func TestRepository_CountQueuedDeploymentsBySite(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	createTestDeployment(t, repo, site.ID, DeploymentStatusQueued)
	createTestDeployment(t, repo, site.ID, DeploymentStatusQueued)

	count, err := repo.CountQueuedDeploymentsBySite(ctx, site.ID)
	require.NoError(t, err)

	assert.Equal(t, int64(2), count)
}

func TestRepository_UpdateDeployment(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	deployment := createTestDeployment(t, repo, site.ID, DeploymentStatusPending)

	deployment.Status = DeploymentStatusFinished
	err := repo.UpdateDeployment(ctx, deployment)
	require.NoError(t, err)

	found, err := repo.FindDeploymentByID(ctx, deployment.ID)
	require.NoError(t, err)

	assert.Equal(t, DeploymentStatusFinished, found.Status)
}

func TestRepository_UpdateDeploymentStatus(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	deployment := createTestDeployment(t, repo, site.ID, DeploymentStatusPending)

	err := repo.UpdateDeploymentStatus(ctx, deployment.ID, DeploymentStatusFailed)
	require.NoError(t, err)

	found, err := repo.FindDeploymentByID(ctx, deployment.ID)
	require.NoError(t, err)

	assert.Equal(t, DeploymentStatusFailed, found.Status)
}

func TestRepository_CancelQueuedDeployments(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	createTestDeployment(t, repo, site.ID, DeploymentStatusQueued)
	createTestDeployment(t, repo, site.ID, DeploymentStatusQueued)
	createTestDeployment(t, repo, site.ID, DeploymentStatusPending)

	count, err := repo.CancelQueuedDeployments(ctx, site.ID)
	require.NoError(t, err)

	assert.Equal(t, int64(2), count)

	queuedCount, err := repo.CountQueuedDeploymentsBySite(ctx, site.ID)
	require.NoError(t, err)

	assert.Equal(t, int64(0), queuedCount)
}

// Certificate repository tests

func TestRepository_CreateCertificate(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	cert := &Certificate{
		SiteID: site.ID,
		Type:   CertificateTypeAuto,
	}

	err := repo.CreateCertificate(ctx, cert)
	require.NoError(t, err)

	assert.NotEmpty(t, cert.ID)
}

func TestRepository_FindCertificateByID(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	t.Run("finds existing certificate", func(t *testing.T) {
		cert := &Certificate{SiteID: site.ID, Type: CertificateTypeAuto}
		repo.CreateCertificate(ctx, cert)

		found, err := repo.FindCertificateByID(ctx, cert.ID)
		require.NoError(t, err)

		assert.Equal(t, cert.ID, found.ID)
	})

	t.Run("returns error for non-existent certificate", func(t *testing.T) {
		_, err := repo.FindCertificateByID(ctx, "non_existent_id")

		assert.ErrorIs(t, err, ErrCertificateNotFound)
	})
}

func TestRepository_FindCertificatesBySite(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	repo.CreateCertificate(ctx, &Certificate{SiteID: site.ID, Type: CertificateTypeAuto})
	repo.CreateCertificate(ctx, &Certificate{SiteID: site.ID, Type: CertificateTypeCustom})

	certs, err := repo.FindCertificatesBySite(ctx, site.ID)
	require.NoError(t, err)

	assert.Len(t, certs, 2)
}

func TestRepository_FindActiveCertificateBySite(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	t.Run("finds active certificate", func(t *testing.T) {
		cert := &Certificate{SiteID: site.ID, Type: CertificateTypeAuto, IsActive: true}
		repo.CreateCertificate(ctx, cert)

		found, err := repo.FindActiveCertificateBySite(ctx, site.ID)
		require.NoError(t, err)

		assert.Equal(t, cert.ID, found.ID)
	})

	t.Run("returns nil when no active certificate", func(t *testing.T) {
		site2 := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "site2.com")

		found, err := repo.FindActiveCertificateBySite(ctx, site2.ID)
		require.NoError(t, err)

		assert.Nil(t, found)
	})
}

func TestRepository_UpdateCertificate(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	cert := &Certificate{SiteID: site.ID, Type: CertificateTypeAuto}
	repo.CreateCertificate(ctx, cert)

	cert.IsActive = true
	err := repo.UpdateCertificate(ctx, cert)
	require.NoError(t, err)

	found, err := repo.FindCertificateByID(ctx, cert.ID)
	require.NoError(t, err)

	assert.True(t, found.IsActive)
}

func TestRepository_DeleteCertificate(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	cert := &Certificate{SiteID: site.ID, Type: CertificateTypeAuto}
	repo.CreateCertificate(ctx, cert)

	err := repo.DeleteCertificate(ctx, cert.ID)
	require.NoError(t, err)

	_, err = repo.FindCertificateByID(ctx, cert.ID)
	assert.ErrorIs(t, err, ErrCertificateNotFound)
}

func TestRepository_DeactivateAllCertificates(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	cert1 := &Certificate{SiteID: site.ID, Type: CertificateTypeAuto, IsActive: true}
	cert2 := &Certificate{SiteID: site.ID, Type: CertificateTypeCustom, IsActive: true}
	repo.CreateCertificate(ctx, cert1)
	repo.CreateCertificate(ctx, cert2)

	err := repo.DeactivateAllCertificates(ctx, site.ID)
	require.NoError(t, err)

	certs, err := repo.FindCertificatesBySite(ctx, site.ID)
	require.NoError(t, err)

	for _, cert := range certs {
		assert.False(t, cert.IsActive)
	}
}

// Queue repository tests

func TestRepository_CreateQueue(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	queue := &Queue{
		SiteID:   site.ID,
		ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		UserID:   "01ARZ3NDEKTSV4RRFFQ69G5FAU",
	}

	err := repo.CreateQueue(ctx, queue)
	require.NoError(t, err)

	assert.NotEmpty(t, queue.ID)
}

func TestRepository_FindQueueByID(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	t.Run("finds existing queue", func(t *testing.T) {
		queue := &Queue{SiteID: site.ID, ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU"}
		repo.CreateQueue(ctx, queue)

		found, err := repo.FindQueueByID(ctx, queue.ID)
		require.NoError(t, err)

		assert.Equal(t, queue.ID, found.ID)
	})

	t.Run("returns error for non-existent queue", func(t *testing.T) {
		_, err := repo.FindQueueByID(ctx, "non_existent_id")

		assert.ErrorIs(t, err, ErrQueueNotFound)
	})
}

func TestRepository_FindQueueByIDAndSite(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	queue := &Queue{SiteID: site.ID, ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU"}
	repo.CreateQueue(ctx, queue)

	t.Run("finds queue by ID and site", func(t *testing.T) {
		found, err := repo.FindQueueByIDAndSite(ctx, queue.ID, site.ID)
		require.NoError(t, err)

		assert.Equal(t, queue.ID, found.ID)
	})

	t.Run("returns error for wrong site", func(t *testing.T) {
		_, err := repo.FindQueueByIDAndSite(ctx, queue.ID, "different_site")

		assert.ErrorIs(t, err, ErrQueueNotFound)
	})
}

func TestRepository_FindQueuesBySite(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	repo.CreateQueue(ctx, &Queue{SiteID: site.ID, ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU"})
	repo.CreateQueue(ctx, &Queue{SiteID: site.ID, ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU"})

	queues, err := repo.FindQueuesBySite(ctx, site.ID)
	require.NoError(t, err)

	assert.Len(t, queues, 2)
}

func TestRepository_FindQueuesByServer(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	site := createTestSite(t, repo, serverID, "example.com")
	repo.CreateQueue(ctx, &Queue{SiteID: site.ID, ServerID: serverID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU"})
	repo.CreateQueue(ctx, &Queue{SiteID: site.ID, ServerID: serverID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU"})

	queues, err := repo.FindQueuesByServer(ctx, serverID)
	require.NoError(t, err)

	assert.Len(t, queues, 2)
}

func TestRepository_CountQueuesBySite(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	repo.CreateQueue(ctx, &Queue{SiteID: site.ID, ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU"})
	repo.CreateQueue(ctx, &Queue{SiteID: site.ID, ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU"})

	count, err := repo.CountQueuesBySite(ctx, site.ID)
	require.NoError(t, err)

	assert.Equal(t, int64(2), count)
}

func TestRepository_UpdateQueue(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	queue := &Queue{SiteID: site.ID, ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU"}
	repo.CreateQueue(ctx, queue)

	now := time.Now()
	queue.InstalledAt = &now
	err := repo.UpdateQueue(ctx, queue)
	require.NoError(t, err)

	found, err := repo.FindQueueByID(ctx, queue.ID)
	require.NoError(t, err)

	assert.NotNil(t, found.InstalledAt)
}

func TestRepository_DeleteQueue(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	queue := &Queue{SiteID: site.ID, ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU"}
	repo.CreateQueue(ctx, queue)

	err := repo.DeleteQueue(ctx, queue.ID)
	require.NoError(t, err)

	_, err = repo.FindQueueByID(ctx, queue.ID)
	assert.ErrorIs(t, err, ErrQueueNotFound)
}

// Command repository tests

func TestRepository_CreateCommand(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	cmd := &Command{
		SiteID:  site.ID,
		UserID:  "01ARZ3NDEKTSV4RRFFQ69G5FAU",
		Command: "php artisan migrate",
	}

	err := repo.CreateCommand(ctx, cmd)
	require.NoError(t, err)

	assert.NotEmpty(t, cmd.ID)
}

func TestRepository_FindCommandByID(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	t.Run("finds existing command", func(t *testing.T) {
		cmd := &Command{SiteID: site.ID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU", Command: "test"}
		repo.CreateCommand(ctx, cmd)

		found, err := repo.FindCommandByID(ctx, cmd.ID)
		require.NoError(t, err)

		assert.Equal(t, cmd.ID, found.ID)
	})

	t.Run("returns error for non-existent command", func(t *testing.T) {
		_, err := repo.FindCommandByID(ctx, "non_existent_id")

		assert.ErrorIs(t, err, ErrCommandNotFound)
	})
}

func TestRepository_FindCommandsBySite(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	repo.CreateCommand(ctx, &Command{SiteID: site.ID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU", Command: "cmd1"})
	repo.CreateCommand(ctx, &Command{SiteID: site.ID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU", Command: "cmd2"})

	cmds, err := repo.FindCommandsBySite(ctx, site.ID)
	require.NoError(t, err)

	assert.Len(t, cmds, 2)
}

func TestRepository_UpdateCommand(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	cmd := &Command{SiteID: site.ID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU", Command: "test"}
	repo.CreateCommand(ctx, cmd)

	output := "success"
	exitCode := 0
	cmd.Output = &output
	cmd.ExitCode = &exitCode
	cmd.Status = CommandStatusFinished
	err := repo.UpdateCommand(ctx, cmd)
	require.NoError(t, err)

	found, err := repo.FindCommandByID(ctx, cmd.ID)
	require.NoError(t, err)

	assert.Equal(t, CommandStatusFinished, found.Status)
	assert.Equal(t, "success", *found.Output)
}

func TestRepository_DeleteCommand(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	cmd := &Command{SiteID: site.ID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU", Command: "test"}
	repo.CreateCommand(ctx, cmd)

	err := repo.DeleteCommand(ctx, cmd.ID)
	require.NoError(t, err)

	_, err = repo.FindCommandByID(ctx, cmd.ID)
	assert.ErrorIs(t, err, ErrCommandNotFound)
}

// Redirect repository tests

func TestRepository_CreateRedirect(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	redirect := &Redirect{
		SiteID: site.ID,
		UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU",
		Mode:   RedirectModePermanent,
		From:   "/old",
		To:     "/new",
	}

	err := repo.CreateRedirect(ctx, redirect)
	require.NoError(t, err)

	assert.NotEmpty(t, redirect.ID)
}

func TestRepository_FindRedirectByID(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	t.Run("finds existing redirect", func(t *testing.T) {
		redirect := &Redirect{SiteID: site.ID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU", Mode: RedirectModePermanent, From: "/a", To: "/b"}
		repo.CreateRedirect(ctx, redirect)

		found, err := repo.FindRedirectByID(ctx, redirect.ID)
		require.NoError(t, err)

		assert.Equal(t, redirect.ID, found.ID)
	})

	t.Run("returns error for non-existent redirect", func(t *testing.T) {
		_, err := repo.FindRedirectByID(ctx, "non_existent_id")

		assert.ErrorIs(t, err, ErrRedirectNotFound)
	})
}

func TestRepository_FindRedirectsBySite(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	repo.CreateRedirect(ctx, &Redirect{SiteID: site.ID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU", Mode: RedirectModePermanent, From: "/a", To: "/b"})
	repo.CreateRedirect(ctx, &Redirect{SiteID: site.ID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU", Mode: RedirectModeTemporary, From: "/c", To: "/d"})

	redirects, err := repo.FindRedirectsBySite(ctx, site.ID)
	require.NoError(t, err)

	assert.Len(t, redirects, 2)
}

func TestRepository_UpdateRedirect(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	redirect := &Redirect{SiteID: site.ID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU", Mode: RedirectModePermanent, From: "/a", To: "/b", Status: "pending"}
	repo.CreateRedirect(ctx, redirect)

	redirect.Status = "active"
	err := repo.UpdateRedirect(ctx, redirect)
	require.NoError(t, err)

	found, err := repo.FindRedirectByID(ctx, redirect.ID)
	require.NoError(t, err)

	assert.Equal(t, "active", found.Status)
}

func TestRepository_DeleteRedirect(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	redirect := &Redirect{SiteID: site.ID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU", Mode: RedirectModePermanent, From: "/a", To: "/b"}
	repo.CreateRedirect(ctx, redirect)

	err := repo.DeleteRedirect(ctx, redirect.ID)
	require.NoError(t, err)

	_, err = repo.FindRedirectByID(ctx, redirect.ID)
	assert.ErrorIs(t, err, ErrRedirectNotFound)
}

// Release repository tests

func TestRepository_CreateRelease(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	release := &Release{
		SiteID: site.ID,
		Path:   "/home/deploy/example.com/releases/20240101120000",
	}

	err := repo.CreateRelease(ctx, release)
	require.NoError(t, err)

	assert.NotEmpty(t, release.ID)
}

func TestRepository_FindReleaseByID(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	t.Run("finds existing release", func(t *testing.T) {
		release := &Release{SiteID: site.ID, Path: "/path/to/release"}
		repo.CreateRelease(ctx, release)

		found, err := repo.FindReleaseByID(ctx, release.ID)
		require.NoError(t, err)

		assert.Equal(t, release.ID, found.ID)
	})

	t.Run("returns error for non-existent release", func(t *testing.T) {
		_, err := repo.FindReleaseByID(ctx, "non_existent_id")

		assert.ErrorIs(t, err, ErrReleaseNotFound)
	})
}

func TestRepository_FindReleasesBySite(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	repo.CreateRelease(ctx, &Release{SiteID: site.ID, Path: "/path1"})
	repo.CreateRelease(ctx, &Release{SiteID: site.ID, Path: "/path2"})

	releases, err := repo.FindReleasesBySite(ctx, site.ID)
	require.NoError(t, err)

	assert.Len(t, releases, 2)
}

func TestRepository_CountReleasesBySite(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	repo.CreateRelease(ctx, &Release{SiteID: site.ID, Path: "/path1"})
	repo.CreateRelease(ctx, &Release{SiteID: site.ID, Path: "/path2"})
	repo.CreateRelease(ctx, &Release{SiteID: site.ID, Path: "/path3"})

	count, err := repo.CountReleasesBySite(ctx, site.ID)
	require.NoError(t, err)

	assert.Equal(t, int64(3), count)
}

func TestRepository_DeleteRelease(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")
	release := &Release{SiteID: site.ID, Path: "/path"}
	repo.CreateRelease(ctx, release)

	err := repo.DeleteRelease(ctx, release.ID)
	require.NoError(t, err)

	_, err = repo.FindReleaseByID(ctx, release.ID)
	assert.ErrorIs(t, err, ErrReleaseNotFound)
}

func TestRepository_DeleteOldReleases(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	site := createTestSite(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "example.com")

	// Create 5 releases
	for i := 0; i < 5; i++ {
		repo.CreateRelease(ctx, &Release{SiteID: site.ID, Path: "/path" + string(rune('0'+i))})
		time.Sleep(10 * time.Millisecond)
	}

	// Keep only 3
	err := repo.DeleteOldReleases(ctx, site.ID, 3)
	require.NoError(t, err)

	count, err := repo.CountReleasesBySite(ctx, site.ID)
	require.NoError(t, err)

	assert.Equal(t, int64(3), count)
}

// Transaction tests

func TestRepository_WithTransaction(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	t.Run("commits on success", func(t *testing.T) {
		var siteID string
		err := repo.WithTransaction(ctx, func(tx *Repository) error {
			site := &Site{
				ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
				UserID:   "01ARZ3NDEKTSV4RRFFQ69G5FAU",
				Address:  "tx-success.com",
			}
			if err := tx.Create(ctx, site); err != nil {
				return err
			}
			siteID = site.ID
			return nil
		})
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, siteID)
		require.NoError(t, err)
		assert.Equal(t, "tx-success.com", found.Address)
	})

	t.Run("rolls back on error", func(t *testing.T) {
		err := repo.WithTransaction(ctx, func(tx *Repository) error {
			site := &Site{
				ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
				UserID:   "01ARZ3NDEKTSV4RRFFQ69G5FAU",
				Address:  "tx-rollback.com",
			}
			if err := tx.Create(ctx, site); err != nil {
				return err
			}
			return assert.AnError
		})
		require.Error(t, err)

		_, err = repo.FindByAddress(ctx, "tx-rollback.com", "01ARZ3NDEKTSV4RRFFQ69G5FAV")
		assert.ErrorIs(t, err, ErrSiteNotFound)
	})
}
