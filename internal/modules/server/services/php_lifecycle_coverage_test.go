package services

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	queuepkg "github.com/kkz6/launch-go/internal/pkg/queue"
	pkgservice "github.com/kkz6/launch-go/internal/pkg/service"
)

func TestPHPServiceQueriesExposeLifecycleState(t *testing.T) {
	service, db, server, services := phpLifecycleDatabaseFixture(t)
	pendingID := services[1].ID
	require.NoError(t, db.Model(server).Update("pending_default_php_service_id", pendingID).Error)

	listed, err := service.ListServices(context.Background(), server.ID, server.TeamID)
	require.NoError(t, err)
	require.Len(t, listed, 3)
	assert.False(t, listed[0].DefaultPending)
	assert.True(t, listed[1].DefaultPending)

	status, err := service.GetServiceStatus(
		context.Background(), server.ID, server.TeamID, services[0].ID,
	)
	require.NoError(t, err)
	assert.Equal(t, services[0].ID, status.ID)

	_, err = service.GetServiceStatus(
		context.Background(), server.ID, server.TeamID, "service-other-server",
	)
	require.Error(t, err)

	versions, err := service.GetPhpVersions(context.Background(), server.ID, server.TeamID)
	require.NoError(t, err)
	require.NotEmpty(t, versions)
	var installed, defaultVersion bool
	for _, version := range versions {
		if version.Key == types.SoftwarePhp83.String() {
			installed = version.IsInstalled
			defaultVersion = version.IsDefault
		}
	}
	assert.True(t, installed)
	assert.True(t, defaultVersion)

	installedVersions, err := service.GetInstalledPhpVersions(
		context.Background(), server.ID, server.TeamID,
	)
	require.NoError(t, err)
	require.Len(t, installedVersions, 2)
	assert.Equal(t, "8.4.2", installedVersions[0].Version)
	assert.Equal(t, "8.3.12", installedVersions[1].Version)
	assert.True(t, installedVersions[1].IsDefault)

	available, err := service.GetAvailableServices(context.Background(), server.ID, server.TeamID)
	require.NoError(t, err)
	require.NotEmpty(t, available)
	for _, group := range available {
		assert.NotEqual(t, types.ServiceTypePostgreSQL.String(), group.Type)
	}
}

func TestDefaultPHPReservationClaimsAndRestoresExactState(t *testing.T) {
	service, db, server, services := phpLifecycleDatabaseFixture(t)
	target := services[1]

	err := service.reserveDefaultPHPChange(
		context.Background(), server.ID, target.ID, target.Status,
	)
	require.NoError(t, err)

	var persistedServer models.Server
	require.NoError(t, db.First(&persistedServer, "id = ?", server.ID).Error)
	require.NotNil(t, persistedServer.PendingDefaultPHPServiceID)
	assert.Equal(t, target.ID, *persistedServer.PendingDefaultPHPServiceID)

	var persistedTarget models.InstalledService
	require.NoError(t, db.First(&persistedTarget, "id = ?", target.ID).Error)
	assert.Equal(t, types.ServiceStatusUpdating, persistedTarget.Status)

	err = service.reserveDefaultPHPChange(
		context.Background(), server.ID, services[0].ID, services[0].Status,
	)
	assert.ErrorIs(t, err, ErrServiceBusy)

	restored, err := service.releaseDefaultPHPReservation(
		context.Background(), server.ID, target.ID, target.Status,
	)
	require.NoError(t, err)
	assert.True(t, restored)

	require.NoError(t, db.First(&persistedServer, "id = ?", server.ID).Error)
	assert.Nil(t, persistedServer.PendingDefaultPHPServiceID)
	require.NoError(t, db.First(&persistedTarget, "id = ?", target.ID).Error)
	assert.Equal(t, target.Status, persistedTarget.Status)

	restored, err = service.releaseDefaultPHPReservation(
		context.Background(), server.ID, target.ID, target.Status,
	)
	assert.False(t, restored)
	require.Error(t, err)
}

func TestDefaultPHPValidationDoesNotMutateInvalidTargets(t *testing.T) {
	service, db, server, services := phpLifecycleDatabaseFixture(t)
	target := services[1]

	target.Status = types.ServiceStatusStopped
	require.NoError(t, db.Model(&models.InstalledService{}).
		Where("id = ?", target.ID).Update("status", target.Status).Error)
	err := service.SetDefaultPhpVersion(
		context.Background(), target.ID, server.ID, server.TeamID, "user-1",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be active")

	err = service.SetDefaultPhpVersion(
		context.Background(), services[2].ID, server.ID, server.TeamID, "user-1",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a PHP installation")

	err = service.SetDefaultPhpVersion(
		context.Background(), "service-other-server", server.ID, server.TeamID, "user-1",
	)
	require.Error(t, err)

	err = service.reserveDefaultPHPChange(
		context.Background(), server.ID, target.ID, types.ServiceStatusStopped,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be active")
}

func TestCompareVersionsHandlesDifferentLengths(t *testing.T) {
	assert.Positive(t, compareVersions("8.4.1", "8.4"))
	assert.Negative(t, compareVersions("8.3", "8.4.0"))
	assert.Zero(t, compareVersions("8.4", "8.4.0"))
	assert.Zero(t, compareVersions("8.x", "8.0"))
}

func TestServiceInstallationAndOperationsValidateBeforeDispatch(t *testing.T) {
	service, db, server, services := phpLifecycleDatabaseFixture(t)

	_, err := service.InstallService(
		context.Background(), server.ID, server.TeamID, "user-1",
		&dto.CreateServiceRequest{Software: "not-real"},
	)
	assert.ErrorIs(t, err, ErrInvalidSoftware)

	_, err = service.InstallService(
		context.Background(), server.ID, server.TeamID, "user-1",
		&dto.CreateServiceRequest{Software: types.SoftwarePhp83.String()},
	)
	assert.ErrorIs(t, err, ErrServiceAlreadyExists)

	created, err := service.InstallService(
		context.Background(), server.ID, server.TeamID, "user-1",
		&dto.CreateServiceRequest{Software: types.SoftwareRedis.String()},
	)
	require.NoError(t, err)
	require.NotNil(t, created.Software)
	assert.Equal(t, types.SoftwareRedis.String(), *created.Software)
	var persisted models.InstalledService
	require.NoError(t, db.First(&persisted, "id = ?", created.ID).Error)
	assert.Equal(t, types.ServiceStatusPending, persisted.Status)

	for _, operation := range []types.ServiceOption{
		types.ServiceOptionStart,
		types.ServiceOptionRestart,
		types.ServiceOptionStop,
		types.ServiceOptionRemove,
		types.ServiceOptionStatus,
		types.ServiceOptionUpdate,
	} {
		err = service.HandleServiceOperation(
			context.Background(), server.ID, server.TeamID, services[2].ID, operation,
		)
		assert.ErrorIs(t, err, pkgservice.ErrQueueRequired)
	}

	err = service.HandleServiceOperation(
		context.Background(), server.ID, server.TeamID, services[2].ID, "invalid",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown operation")

	err = service.HandleServiceOperation(
		context.Background(), server.ID, server.TeamID, "service-other-server", types.ServiceOptionStart,
	)
	require.Error(t, err)

	err = service.CheckServiceStatus(
		context.Background(), server.ID, server.TeamID, services[2].ID,
	)
	assert.ErrorIs(t, err, pkgservice.ErrQueueRequired)
}

func TestPHPLifecycleOperationsReserveAndEnqueue(t *testing.T) {
	t.Run("set default", func(t *testing.T) {
		service, db, server, services := phpLifecycleDatabaseFixture(t)
		attachTestQueue(t, service)

		err := service.SetDefaultPhpVersion(
			context.Background(), services[1].ID, server.ID, server.TeamID, "user-1",
		)
		require.NoError(t, err)

		var persistedServer models.Server
		require.NoError(t, db.First(&persistedServer, "id = ?", server.ID).Error)
		require.NotNil(t, persistedServer.PendingDefaultPHPServiceID)
		assert.Equal(t, services[1].ID, *persistedServer.PendingDefaultPHPServiceID)
		var target models.InstalledService
		require.NoError(t, db.First(&target, "id = ?", services[1].ID).Error)
		assert.Equal(t, types.ServiceStatusUpdating, target.Status)
	})

	t.Run("patch", func(t *testing.T) {
		service, db, server, services := phpLifecycleDatabaseFixture(t)
		attachTestQueue(t, service)

		err := service.PatchPhpVersion(
			context.Background(), services[0].ID, server.ID, server.TeamID, "user-1",
		)
		require.NoError(t, err)

		var target models.InstalledService
		require.NoError(t, db.First(&target, "id = ?", services[0].ID).Error)
		assert.Equal(t, types.ServiceStatusUpdating, target.Status)
		assert.Equal(t, "queued", target.TypeData["patch_status"])
	})

	t.Run("generic operations", func(t *testing.T) {
		service, _, server, services := phpLifecycleDatabaseFixture(t)
		attachTestQueue(t, service)

		for _, operation := range []types.ServiceOption{
			types.ServiceOptionStart,
			types.ServiceOptionRestart,
			types.ServiceOptionStop,
			types.ServiceOptionRemove,
			types.ServiceOptionStatus,
			types.ServiceOptionUpdate,
		} {
			err := service.HandleServiceOperation(
				context.Background(), server.ID, server.TeamID, services[2].ID, operation,
			)
			require.NoError(t, err)
		}
		require.NoError(t, service.CheckServiceStatus(
			context.Background(), server.ID, server.TeamID, services[2].ID,
		))
	})
}

func attachTestQueue(t *testing.T, service *Service) {
	t.Helper()
	redis := miniredis.RunT(t)
	client := queuepkg.NewClient(config.RedisConfig{Address: redis.Addr()})
	service.Queue = client
	t.Cleanup(func() {
		require.NoError(t, client.Close())
	})
}

func phpLifecycleDatabaseFixture(
	t *testing.T,
) (*Service, *gorm.DB, *models.Server, []models.InstalledService) {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"),
		&gorm.Config{
			DisableForeignKeyConstraintWhenMigrating: true,
			Logger:                                   gormlogger.Default.LogMode(gormlogger.Silent),
		},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Server{}, &models.InstalledService{}))

	server := &models.Server{
		BaseModel:  basemodels.BaseModel{ID: "server-coverage"},
		TeamScoped: basemodels.TeamScoped{TeamID: "team-coverage"},
		Name:       "Coverage server",
		Provider:   types.ProviderCustom,
		Status:     types.ServerStatusRunning,
	}
	require.NoError(t, db.Create(server).Error)

	services := []models.InstalledService{
		{
			BaseModel: basemodels.BaseModel{ID: "php-83"}, ServerScoped: basemodels.ServerScoped{ServerID: server.ID},
			Type: types.ServiceTypePhp, Name: "PHP 8.3", Software: types.SoftwarePhp83.String(),
			Version: "8.3.12", Status: types.ServiceStatusRunning, IsDefault: true,
		},
		{
			BaseModel: basemodels.BaseModel{ID: "php-84"}, ServerScoped: basemodels.ServerScoped{ServerID: server.ID},
			Type: types.ServiceTypePhp, Name: "PHP 8.4", Software: types.SoftwarePhp84.String(),
			Version: "8.4.2", Status: types.ServiceStatusInstalled,
		},
		{
			BaseModel: basemodels.BaseModel{ID: "mysql"}, ServerScoped: basemodels.ServerScoped{ServerID: server.ID},
			Type: types.ServiceTypeMySQL, Name: "MySQL", Software: types.SoftwareMySQL80.String(),
			Version: "8.0", Status: types.ServiceStatusRunning,
		},
		{
			BaseModel: basemodels.BaseModel{ID: "service-other-server"}, ServerScoped: basemodels.ServerScoped{ServerID: "other"},
			Type: types.ServiceTypePhp, Name: "PHP 8.2", Software: types.SoftwarePhp82.String(),
			Version: "8.2.1", Status: types.ServiceStatusRunning,
		},
	}
	require.NoError(t, db.Create(&services).Error)

	logger := zerolog.New(io.Discard)
	registry := repositories.NewRegistry(db)
	return NewService(ServiceDeps{
		Dependencies: pkgservice.Dependencies{DB: db, Logger: &logger},
		Repos:        registry,
	}), db, server, services
}
