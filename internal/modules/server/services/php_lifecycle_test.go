package services

import (
	"context"
	"io"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	pkgservice "github.com/kkz6/launch-go/internal/pkg/service"
)

type phpLifecycleServerRepository struct {
	contracts.ServerRepository
	server *models.Server
}

func (r *phpLifecycleServerRepository) FindByIDAndTeam(
	_ context.Context,
	id,
	teamID string,
	_ ...string,
) (*models.Server, error) {
	if r.server != nil && r.server.ID == id && r.server.TeamID == teamID {
		return r.server, nil
	}
	return nil, fiberutil.NotFound()
}

type phpLifecycleServiceRepository struct {
	contracts.ServiceRepository
	service           *models.InstalledService
	rejectCanceledCtx bool
}

func (r *phpLifecycleServiceRepository) FindByID(
	_ context.Context,
	id string,
) (*models.InstalledService, error) {
	if r.service != nil && r.service.ID == id {
		return r.service, nil
	}
	return nil, fiberutil.NotFound()
}

func (r *phpLifecycleServiceRepository) FindPhpByServerAndVersion(
	_ context.Context,
	serverID,
	_ string,
) (*models.InstalledService, error) {
	if r.service != nil && r.service.ServerID == serverID {
		return r.service, nil
	}
	return nil, fiberutil.NotFound()
}

func (r *phpLifecycleServiceRepository) ClaimPhpPatch(
	_ context.Context,
	id string,
	previousStatus types.ServiceStatus,
) (bool, error) {
	if r.service == nil ||
		r.service.ID != id ||
		!previousStatus.IsActive() ||
		r.service.Status != previousStatus {
		return false, nil
	}
	r.service.Status = types.ServiceStatusUpdating
	return true, nil
}

func (r *phpLifecycleServiceRepository) RestorePhpPatchStatus(
	ctx context.Context,
	id string,
	serviceStatus types.ServiceStatus,
) (bool, error) {
	if r.rejectCanceledCtx && ctx.Err() != nil {
		return false, ctx.Err()
	}
	if r.service != nil &&
		r.service.ID == id &&
		r.service.Status == types.ServiceStatusUpdating &&
		serviceStatus.IsActive() {
		r.service.Status = serviceStatus
		return true, nil
	}
	return false, nil
}

func TestPatchReservationRollbackSurvivesCanceledRequest(t *testing.T) {
	service, installed := newPhpLifecycleService()
	repository := service.repos.Service().(*phpLifecycleServiceRepository)
	repository.rejectCanceledCtx = true
	installed.Status = types.ServiceStatusUpdating

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	restored, err := service.restorePhpPatchReservation(
		ctx,
		installed.ID,
		types.ServiceStatusRunning,
	)

	require.NoError(t, err)
	assert.True(t, restored)
	assert.Equal(t, types.ServiceStatusRunning, installed.Status)
}

type phpLifecycleRegistry struct {
	serverRepository  contracts.ServerRepository
	serviceRepository contracts.ServiceRepository
}

func (r *phpLifecycleRegistry) Server() contracts.ServerRepository { return r.serverRepository }
func (r *phpLifecycleRegistry) Service() contracts.ServiceRepository {
	return r.serviceRepository
}
func (r *phpLifecycleRegistry) FirewallRule() contracts.FirewallRuleRepository { return nil }
func (r *phpLifecycleRegistry) Cron() contracts.CronRepository                 { return nil }
func (r *phpLifecycleRegistry) Daemon() contracts.DaemonRepository             { return nil }
func (r *phpLifecycleRegistry) SSHKey() contracts.SSHKeyRepository             { return nil }
func (r *phpLifecycleRegistry) Task() contracts.TaskRepository                 { return nil }
func (r *phpLifecycleRegistry) Metric() contracts.MetricRepository             { return nil }
func (r *phpLifecycleRegistry) ServerProvider() contracts.ServerProviderRepository {
	return nil
}
func (r *phpLifecycleRegistry) Database() contracts.DatabaseRepository { return nil }
func (r *phpLifecycleRegistry) LoadBalancerUpstream() contracts.LoadBalancerUpstreamRepository {
	return nil
}
func (r *phpLifecycleRegistry) LoadBalancerBackend() contracts.LoadBalancerBackendRepository {
	return nil
}
func (r *phpLifecycleRegistry) DB() *gorm.DB { return nil }

func TestPatchPhpVersionValidatesOwnershipTypeAndBusyState(t *testing.T) {
	service, installed := newPhpLifecycleService()

	t.Run("rejects service from another server", func(t *testing.T) {
		installed.ServerID = "server-b"
		err := service.PatchPhpVersion(
			context.Background(),
			installed.ID,
			"server-a",
			"team-a",
			"user-a",
		)
		require.Error(t, err)
		assert.True(t, fiberutil.IsNotFound(err))
	})

	t.Run("rejects non PHP service", func(t *testing.T) {
		installed.ServerID = "server-a"
		installed.Type = types.ServiceTypeRedis
		installed.Software = types.SoftwareRedis.String()
		err := service.PatchPhpVersion(
			context.Background(),
			installed.ID,
			"server-a",
			"team-a",
			"user-a",
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a PHP installation")
	})

	t.Run("rejects concurrent lifecycle operation", func(t *testing.T) {
		installed.Type = types.ServiceTypePhp
		installed.Software = types.SoftwarePhp83.String()
		installed.Status = types.ServiceStatusUpdating
		err := service.PatchPhpVersion(
			context.Background(),
			installed.ID,
			"server-a",
			"team-a",
			"user-a",
		)
		assert.ErrorIs(t, err, ErrServiceBusy)
	})

	t.Run("accepts installed PHP before queue dispatch", func(t *testing.T) {
		installed.Status = types.ServiceStatusRunning
		err := service.PatchPhpVersion(
			context.Background(),
			installed.ID,
			"server-a",
			"team-a",
			"user-a",
		)
		assert.ErrorIs(t, err, pkgservice.ErrQueueRequired)
		assert.Equal(t, types.ServiceStatusRunning, installed.Status)
	})
}

func TestSetDefaultPhpVersionUsesInstalledServiceIdentity(t *testing.T) {
	service, installed := newPhpLifecycleService()

	err := service.SetDefaultPhpVersion(
		context.Background(),
		installed.ID,
		"server-a",
		"team-a",
		"user-a",
	)

	assert.ErrorIs(t, err, pkgservice.ErrQueueRequired)

	installed.Status = types.ServiceStatusUpdating
	err = service.SetDefaultPhpVersion(
		context.Background(),
		installed.ID,
		"server-a",
		"team-a",
		"user-a",
	)
	assert.ErrorIs(t, err, ErrServiceBusy)
}

func TestPatchPhpVersionReleasesExactClaimWhenDispatchFails(t *testing.T) {
	for _, previousStatus := range []types.ServiceStatus{
		types.ServiceStatusRunning,
		types.ServiceStatusInstalled,
	} {
		t.Run(previousStatus.String(), func(t *testing.T) {
			service, installed := newPhpLifecycleService()
			installed.Status = previousStatus

			err := service.PatchPhpVersion(
				context.Background(),
				installed.ID,
				"server-a",
				"team-a",
				"user-a",
			)

			assert.ErrorIs(t, err, pkgservice.ErrQueueRequired)
			assert.Equal(t, previousStatus, installed.Status)
		})
	}
}

func TestServiceOperationsRejectPHPWhilePatchIsRunning(t *testing.T) {
	service, installed := newPhpLifecycleService()
	installed.Status = types.ServiceStatusUpdating

	err := service.HandleServiceOperation(
		context.Background(),
		"server-a",
		"team-a",
		installed.ID,
		types.ServiceOptionRemove,
	)

	assert.ErrorIs(t, err, ErrServiceBusy)
}

func TestPhpExtensionChangesRejectPatchInProgress(t *testing.T) {
	service, installed := newPhpLifecycleService()
	installed.Status = types.ServiceStatusUpdating

	installErr := service.InstallPhpExtension(
		context.Background(),
		"server-a",
		"team-a",
		"8.3",
		"redis",
		nil,
	)
	uninstallErr := service.UninstallPhpExtension(
		context.Background(),
		"server-a",
		"team-a",
		"8.3",
		"redis",
		nil,
	)

	assert.ErrorIs(t, installErr, ErrServiceBusy)
	assert.ErrorIs(t, uninstallErr, ErrServiceBusy)
}

func newPhpLifecycleService() (*Service, *models.InstalledService) {
	server := &models.Server{
		BaseModel:  basemodels.BaseModel{ID: "server-a"},
		TeamScoped: basemodels.TeamScoped{TeamID: "team-a"},
	}
	installed := &models.InstalledService{
		BaseModel:    basemodels.BaseModel{ID: "service-php83"},
		ServerScoped: basemodels.ServerScoped{ServerID: server.ID},
		Type:         types.ServiceTypePhp,
		Name:         "PHP 8.3",
		Version:      "8.3.6",
		Status:       types.ServiceStatusRunning,
		Software:     types.SoftwarePhp83.String(),
	}
	registry := &phpLifecycleRegistry{
		serverRepository:  &phpLifecycleServerRepository{server: server},
		serviceRepository: &phpLifecycleServiceRepository{service: installed},
	}
	logger := zerolog.New(io.Discard)
	service := NewService(ServiceDeps{
		Dependencies: pkgservice.Dependencies{Logger: &logger},
		Repos:        registry,
	})
	return service, installed
}
