package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

func TestFindPhpByServerAndVersionUsesStableSoftwareIdentity(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:php-version-lookup?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.InstalledService{}))

	service := &models.InstalledService{
		BaseModel:    basemodels.BaseModel{ID: "service-php83"},
		ServerScoped: basemodels.ServerScoped{ServerID: "server-a"},
		Type:         types.ServiceTypePhp,
		Name:         "PHP 8.3",
		Version:      "8.3.6",
		Status:       types.ServiceStatusRunning,
		Software:     types.SoftwarePhp83.String(),
	}
	require.NoError(t, db.Create(service).Error)

	repo := NewServiceRepository(db)
	for _, version := range []string{"php83", "8.3", "8.3.6", "8.3.99"} {
		found, findErr := repo.FindPhpByServerAndVersion(context.Background(), "server-a", version)
		require.NoError(t, findErr)
		assert.Equal(t, service.ID, found.ID)
	}
}

func TestClaimPhpPatchOnlyClaimsExactActiveState(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:php-patch-claim?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.InstalledService{}))

	service := &models.InstalledService{
		BaseModel:    basemodels.BaseModel{ID: "service-claim-php83"},
		ServerScoped: basemodels.ServerScoped{ServerID: "server-a"},
		Type:         types.ServiceTypePhp,
		Name:         "PHP 8.3",
		Version:      "8.3.6",
		Status:       types.ServiceStatusRunning,
		Software:     types.SoftwarePhp83.String(),
	}
	require.NoError(t, db.Create(service).Error)

	repo := NewServiceRepository(db)
	claimed, err := repo.ClaimPhpPatch(context.Background(), service.ID, types.ServiceStatusRunning)
	require.NoError(t, err)
	assert.True(t, claimed)

	claimed, err = repo.ClaimPhpPatch(context.Background(), service.ID, types.ServiceStatusRunning)
	require.NoError(t, err)
	assert.False(t, claimed)

	staleAt := time.Now().Add(-45 * time.Minute)
	require.NoError(t, db.Model(service).Updates(map[string]any{
		"status":     types.ServiceStatusUpdating,
		"updated_at": staleAt,
	}).Error)
	claimed, err = repo.ClaimPhpPatch(context.Background(), service.ID, types.ServiceStatusRunning)
	require.NoError(t, err)
	assert.False(t, claimed, "an updating reservation must never be reclaimed based on age")

	released, err := repo.RestorePhpPatchStatus(
		context.Background(),
		service.ID,
		types.ServiceStatusRunning,
	)
	require.NoError(t, err)
	assert.True(t, released)

	var restoredService models.InstalledService
	require.NoError(t, db.First(&restoredService, "id = ?", service.ID).Error)
	assert.Equal(t, types.ServiceStatusRunning, restoredService.Status)

	claimed, err = repo.ClaimPhpPatch(context.Background(), service.ID, types.ServiceStatusStopped)
	require.NoError(t, err)
	assert.False(t, claimed, "inactive states cannot be used as patch claims")
}

func TestUpdateStatusFromProbePreservesPhpPatchReservation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:php-patch-probe?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.InstalledService{}))

	service := &models.InstalledService{
		BaseModel:    basemodels.BaseModel{ID: "service-probe-php83"},
		ServerScoped: basemodels.ServerScoped{ServerID: "server-a"},
		Type:         types.ServiceTypePhp,
		TypeData:     map[string]any{"patch_status": "running"},
		Name:         "PHP 8.3",
		Version:      "8.3.6",
		Status:       types.ServiceStatusUpdating,
		Software:     types.SoftwarePhp83.String(),
	}
	require.NoError(t, db.Create(service).Error)

	repo := NewServiceRepository(db)
	updated, err := repo.UpdateStatusFromProbe(
		context.Background(),
		service.ID,
		types.ServiceStatusRunning,
		map[string]any{"status_output": "active (running)"},
	)
	require.NoError(t, err)
	assert.False(t, updated)

	var preserved models.InstalledService
	require.NoError(t, db.First(&preserved, "id = ?", service.ID).Error)
	assert.Equal(t, types.ServiceStatusUpdating, preserved.Status)
	assert.Equal(t, "running", preserved.TypeData["patch_status"])
	assert.NotContains(t, preserved.TypeData, "status_output")

	released, err := repo.RestorePhpPatchStatus(
		context.Background(),
		service.ID,
		types.ServiceStatusRunning,
	)
	require.NoError(t, err)
	require.True(t, released)

	updated, err = repo.UpdateStatusFromProbe(
		context.Background(),
		service.ID,
		types.ServiceStatusStopped,
		map[string]any{"status_output": "inactive (dead)"},
	)
	require.NoError(t, err)
	assert.True(t, updated)

	require.NoError(t, db.First(&preserved, "id = ?", service.ID).Error)
	assert.Equal(t, types.ServiceStatusStopped, preserved.Status)
	assert.Equal(t, "running", preserved.TypeData["patch_status"])
	assert.Equal(t, "inactive (dead)", preserved.TypeData["status_output"])
}
