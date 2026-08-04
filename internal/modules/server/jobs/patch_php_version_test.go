package jobs

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/database/serializers"
	"github.com/kkz6/launch-go/internal/modules/server/contracts"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	servertemplates "github.com/kkz6/launch-go/internal/modules/server/tasks/templates"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/broadcast"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	tasktemplates "github.com/kkz6/launch-go/internal/pkg/taskrunner/templates"
)

type patchJobServerRepository struct {
	contracts.ServerRepository
	server *models.Server
}

func (r *patchJobServerRepository) FindByID(_ context.Context, id string) (*models.Server, error) {
	if r.server != nil && r.server.ID == id {
		return r.server, nil
	}
	return nil, gorm.ErrRecordNotFound
}

type patchJobServiceRepository struct {
	contracts.ServiceRepository
	service              *models.InstalledService
	services             []models.InstalledService
	unsetDefaultServerID string
	defaultServiceID     string
	failedStateUpdates   int
	rejectCanceledCtx    bool
	deletedServiceID     string
	removalFailed        bool
}

func (r *patchJobServiceRepository) FindByID(_ context.Context, id string) (*models.InstalledService, error) {
	if r.service != nil && r.service.ID == id {
		return r.service, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *patchJobServiceRepository) FindByServerAndType(
	_ context.Context,
	serverID string,
	serviceType types.ServiceType,
) ([]models.InstalledService, error) {
	if len(r.services) > 0 {
		matches := make([]models.InstalledService, 0, len(r.services))
		for i := range r.services {
			if r.services[i].ServerID == serverID && r.services[i].Type == serviceType {
				matches = append(matches, r.services[i])
			}
		}
		return matches, nil
	}
	if r.service != nil &&
		r.service.ServerID == serverID &&
		r.service.Type == serviceType {
		return []models.InstalledService{*r.service}, nil
	}
	return nil, nil
}

func (r *patchJobServiceRepository) UpdateFields(ctx context.Context, id string, fields map[string]any) error {
	if r.service == nil || r.service.ID != id {
		return gorm.ErrRecordNotFound
	}
	if r.rejectCanceledCtx && ctx.Err() != nil {
		return ctx.Err()
	}
	if typeData, ok := fields["type_data"].(dbtype.JSONMap); ok &&
		typeData["patch_status"] == "failed" &&
		r.failedStateUpdates > 0 {
		r.failedStateUpdates--
		return errors.New("temporary database failure")
	}
	if serviceStatus, ok := fields["status"].(types.ServiceStatus); ok {
		r.service.Status = serviceStatus
	}
	if version, ok := fields["version"].(string); ok {
		r.service.Version = version
	}
	if typeData, ok := fields["type_data"].(dbtype.JSONMap); ok {
		r.service.TypeData = typeData
	}
	return nil
}

func (r *patchJobServiceRepository) UnsetDefaultPhp(_ context.Context, serverID string) error {
	r.unsetDefaultServerID = serverID
	r.service.IsDefault = false
	return nil
}

func (r *patchJobServiceRepository) SetDefault(_ context.Context, id string, isDefault bool) error {
	r.defaultServiceID = id
	r.service.IsDefault = isDefault
	return nil
}

func (r *patchJobServiceRepository) UpdateStatus(
	ctx context.Context,
	id string,
	status types.ServiceStatus,
) error {
	return r.UpdateFields(ctx, id, map[string]any{"status": status})
}

func (r *patchJobServiceRepository) RestorePhpPatchStatus(
	_ context.Context,
	id string,
	status types.ServiceStatus,
) (bool, error) {
	if r.service == nil ||
		r.service.ID != id ||
		r.service.Status != types.ServiceStatusUpdating ||
		!status.IsActive() {
		return false, nil
	}
	r.service.Status = status
	return true, nil
}

func (r *patchJobServiceRepository) Delete(_ context.Context, id string) error {
	if r.service == nil || r.service.ID != id {
		return gorm.ErrRecordNotFound
	}
	r.deletedServiceID = id
	return nil
}

func (r *patchJobServiceRepository) MarkRemovalFailed(_ context.Context, id string) error {
	if r.service == nil || r.service.ID != id {
		return gorm.ErrRecordNotFound
	}
	r.removalFailed = true
	return nil
}

type patchJobRegistry struct {
	serverRepository  contracts.ServerRepository
	serviceRepository contracts.ServiceRepository
	db                *gorm.DB
}

func (r *patchJobRegistry) Server() contracts.ServerRepository { return r.serverRepository }
func (r *patchJobRegistry) Service() contracts.ServiceRepository {
	return r.serviceRepository
}
func (r *patchJobRegistry) FirewallRule() contracts.FirewallRuleRepository { return nil }
func (r *patchJobRegistry) Cron() contracts.CronRepository                 { return nil }
func (r *patchJobRegistry) Daemon() contracts.DaemonRepository             { return nil }
func (r *patchJobRegistry) SSHKey() contracts.SSHKeyRepository             { return nil }
func (r *patchJobRegistry) Task() contracts.TaskRepository                 { return nil }
func (r *patchJobRegistry) Metric() contracts.MetricRepository             { return nil }
func (r *patchJobRegistry) ServerProvider() contracts.ServerProviderRepository {
	return nil
}
func (r *patchJobRegistry) Database() contracts.DatabaseRepository { return nil }
func (r *patchJobRegistry) LoadBalancerUpstream() contracts.LoadBalancerUpstreamRepository {
	return nil
}
func (r *patchJobRegistry) LoadBalancerBackend() contracts.LoadBalancerBackendRepository {
	return nil
}
func (r *patchJobRegistry) DB() *gorm.DB { return r.db }

type patchBroadcastEvent struct {
	event string
	data  map[string]any
}

type patchRecordingBroadcaster struct {
	broadcast.NopBroadcaster
	mu     sync.Mutex
	events []patchBroadcastEvent
}

var (
	registerPatchTemplatesOnce sync.Once
	registerPatchTemplatesErr  error
)

func (b *patchRecordingBroadcaster) BroadcastToTeam(_ string, event string, data any) {
	payload, _ := data.(map[string]any)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, patchBroadcastEvent{event: event, data: payload})
}

func (b *patchRecordingBroadcaster) hasEvent(event, status string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, item := range b.events {
		if item.event == event && item.data["status"] == status {
			return true
		}
	}
	return false
}

func TestPatchPhpVersionJobPersistsDetectedVersionAndTrackedTask(t *testing.T) {
	job, service, db, dispatcher, broadcaster := newPatchPhpVersionTestJob(t)
	dispatcher.DefaultResult.Output = "Packages patched\nLAUNCH_PHP_PATCH_VERSION=8.3.12\n"

	require.NoError(t, job.Handle(context.Background()))

	assert.Equal(t, "8.3.12", service.Version)
	assert.Equal(t, types.ServiceStatusRunning, service.Status)
	assert.Equal(t, "finished", service.TypeData["patch_status"])
	assert.Empty(t, service.TypeData["patch_error"])
	assert.True(t, broadcaster.hasEvent("php.patch", "running"))
	assert.True(t, broadcaster.hasEvent("php.patch", "finished"))
	assert.Equal(t, 1, dispatcher.ExecutionCount())
	require.Len(t, dispatcher.Executions, 1)
	assert.Equal(t, "Patch PHP 8.3", dispatcher.Executions[0].Task.Name())

	var persistedTask models.Task
	require.NoError(t, db.First(&persistedTask).Error)
	assert.Equal(t, "Patch PHP 8.3", persistedTask.Name)
	assert.Equal(t, string(types.TaskStatusFinished), persistedTask.Status)
	require.NotNil(t, persistedTask.ExitCode)
	assert.Zero(t, *persistedTask.ExitCode)
}

func TestPatchPhpVersionJobRecordsTaskFailure(t *testing.T) {
	job, service, db, dispatcher, broadcaster := newPatchPhpVersionTestJob(t)
	dispatcher.SetDefaultFailure(1, "apt repository unavailable")

	err := job.Handle(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "apt repository unavailable")
	assert.Equal(t, "8.3.6", service.Version)
	assert.Equal(t, types.ServiceStatusRunning, service.Status)
	assert.Equal(t, "failed", service.TypeData["patch_status"])
	assert.Contains(t, service.TypeData["patch_error"], "apt repository unavailable")
	assert.True(t, broadcaster.hasEvent("php.patch", "failed"))

	var persistedTask models.Task
	require.NoError(t, db.First(&persistedTask).Error)
	assert.Equal(t, string(types.TaskStatusFailed), persistedTask.Status)
	require.NotNil(t, persistedTask.ExitCode)
	assert.Equal(t, 1, *persistedTask.ExitCode)
}

func TestPatchPhpVersionJobRestoresInstalledStatusOnFailure(t *testing.T) {
	job, service, _, dispatcher, _ := newPatchPhpVersionTestJob(t)
	job.Payload.PreviousStatus = types.ServiceStatusInstalled
	dispatcher.SetDefaultFailure(1, "package upgrade failed")

	err := job.Handle(context.Background())

	require.Error(t, err)
	assert.Equal(t, types.ServiceStatusInstalled, service.Status)
	assert.Equal(t, "failed", service.TypeData["patch_status"])
	assert.Contains(t, service.TypeData["patch_error"], "package upgrade failed")
}

func TestPatchPhpVersionFailedCallbackRetriesStateRestoration(t *testing.T) {
	job, service, _, dispatcher, _ := newPatchPhpVersionTestJob(t)
	serviceRepository := job.Deps.Repos.Service().(*patchJobServiceRepository)
	serviceRepository.failedStateUpdates = 1
	dispatcher.SetDefaultFailure(1, "package upgrade failed")

	err := job.Handle(context.Background())

	require.Error(t, err)
	assert.Equal(t, types.ServiceStatusUpdating, service.Status)
	assert.False(t, job.failureRecorded)

	job.Failed(context.Background(), err)

	assert.True(t, job.failureRecorded)
	assert.Equal(t, types.ServiceStatusRunning, service.Status)
	assert.Equal(t, "failed", service.TypeData["patch_status"])
}

func TestPatchPhpVersionFailureCleanupSurvivesCanceledContext(t *testing.T) {
	job, service, _, _, _ := newPatchPhpVersionTestJob(t)
	serviceRepository := job.Deps.Repos.Service().(*patchJobServiceRepository)
	serviceRepository.rejectCanceledCtx = true
	job.service = service
	job.server = job.Deps.Repos.Server().(*patchJobServerRepository).server
	job.previousStatus = types.ServiceStatusRunning
	job.versionSeries = "8.3"
	job.validated = true

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := job.recordFailure(ctx, errors.New("package upgrade timed out"))

	require.Error(t, err)
	assert.Equal(t, types.ServiceStatusRunning, service.Status)
	assert.Equal(t, "failed", service.TypeData["patch_status"])
	assert.Contains(t, service.TypeData["patch_error"], "package upgrade timed out")
	assert.True(t, job.failureRecorded)
}

func TestPatchPhpVersionJobRejectsMismatchedServiceOwnership(t *testing.T) {
	job, service, _, dispatcher, _ := newPatchPhpVersionTestJob(t)
	service.ServerID = "different-server"

	err := job.Handle(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong")
	assert.Zero(t, dispatcher.ExecutionCount())
}

func TestNewPatchPhpVersionTaskCarriesReservationState(t *testing.T) {
	t.Parallel()

	task, err := NewPatchPhpVersionTask(
		"server-a",
		"service-php83",
		types.ServiceStatusRunning,
		nil,
	)

	require.NoError(t, err)
	assert.Equal(t, TypePatchPhpVersion, task.Type())
	payload, err := pkgjobs.UnmarshalPayload[PatchPhpVersionPayload](task)
	require.NoError(t, err)
	assert.Equal(t, "server-a", payload.ServerID)
	assert.Equal(t, "service-php83", payload.ServiceID)
	assert.Equal(t, types.ServiceStatusRunning, payload.PreviousStatus)
}

func TestBoundedPatchErrorLimitsPersistedAndBroadcastPayload(t *testing.T) {
	message := strings.Repeat("x", 20)

	assert.Equal(t, "xxxx…", boundedPatchError(message, 5))
	assert.Equal(t, message, boundedPatchError(message, len(message)))
}

func TestSetDefaultPhpJobUsesSeriesAndCreatesTrackedTask(t *testing.T) {
	patchJob, service, db, dispatcher, _ := newPatchPhpVersionTestJob(t)
	serviceRepo := patchJob.Deps.Repos.Service().(*patchJobServiceRepository)
	job := &SetDefaultPhpJob{
		Deps: patchJob.Deps,
		Payload: SetDefaultPhpPayload{
			ServerID:       "server-a",
			ServiceID:      service.ID,
			Version:        "8.3",
			PreviousStatus: types.ServiceStatusRunning,
		},
	}

	require.NoError(t, job.Handle(context.Background()))

	assert.True(t, service.IsDefault)
	assert.Empty(t, serviceRepo.unsetDefaultServerID)
	assert.Empty(t, serviceRepo.defaultServiceID)
	require.Len(t, dispatcher.Executions, 1)
	assert.Equal(t, "Set Default PHP 8.3", dispatcher.Executions[0].Task.Name())
	assert.Contains(t, dispatcher.Executions[0].Script, "/usr/bin/php8.3")
	assert.NotContains(t, dispatcher.Executions[0].Script, "php8.3.6")

	var persistedServer models.Server
	require.NoError(t, db.First(&persistedServer, "id = ?", "server-a").Error)
	assert.Nil(t, persistedServer.PendingDefaultPHPServiceID)
	var persistedService models.InstalledService
	require.NoError(t, db.First(&persistedService, "id = ?", service.ID).Error)
	assert.True(t, persistedService.IsDefault)

	var persistedTask models.Task
	require.NoError(t, db.First(&persistedTask).Error)
	assert.Equal(t, "Set Default PHP 8.3", persistedTask.Name)
	assert.Equal(t, string(types.TaskStatusFinished), persistedTask.Status)
}

func TestSetDefaultPhpJobRejectsFullVersionPayload(t *testing.T) {
	patchJob, service, _, dispatcher, _ := newPatchPhpVersionTestJob(t)
	job := &SetDefaultPhpJob{
		Deps: patchJob.Deps,
		Payload: SetDefaultPhpPayload{
			ServerID:       "server-a",
			ServiceID:      service.ID,
			Version:        "8.3.6",
			PreviousStatus: types.ServiceStatusRunning,
		},
	}

	err := job.Handle(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
	assert.Zero(t, dispatcher.ExecutionCount())
}

func TestSetDefaultPhpJobRejectsLostReservation(t *testing.T) {
	patchJob, service, _, dispatcher, _ := newPatchPhpVersionTestJob(t)
	patchJob.Deps.Repos.Server().(*patchJobServerRepository).
		server.PendingDefaultPHPServiceID = nil
	job := &SetDefaultPhpJob{
		Deps: patchJob.Deps,
		Payload: SetDefaultPhpPayload{
			ServerID:       "server-a",
			ServiceID:      service.ID,
			Version:        "8.3",
			PreviousStatus: types.ServiceStatusRunning,
		},
	}

	err := job.Handle(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "reservation was lost")
	assert.Zero(t, dispatcher.ExecutionCount())
}

func TestSetDefaultPhpJobRollsBackRuntimeWhenPersistenceFails(t *testing.T) {
	patchJob, service, db, dispatcher, _ := newPatchPhpVersionTestJob(t)
	previous := models.InstalledService{
		BaseModel:    basemodels.BaseModel{ID: "service-php82"},
		ServerScoped: basemodels.ServerScoped{ServerID: "server-a"},
		Type:         types.ServiceTypePhp,
		Name:         "PHP 8.2",
		Version:      "8.2.20",
		Status:       types.ServiceStatusRunning,
		Software:     types.SoftwarePhp82.String(),
		IsDefault:    true,
	}
	require.NoError(t, db.Create(&previous).Error)
	serviceRepo := patchJob.Deps.Repos.Service().(*patchJobServiceRepository)
	serviceRepo.services = []models.InstalledService{previous, *service}

	// Keep the mocked API lookup alive while making the target disappear from
	// the persistence transaction after the remote alternatives command runs.
	require.NoError(t, db.Delete(&models.InstalledService{}, "id = ?", service.ID).Error)
	job := &SetDefaultPhpJob{
		Deps: patchJob.Deps,
		Payload: SetDefaultPhpPayload{
			ServerID:       "server-a",
			ServiceID:      service.ID,
			Version:        "8.3",
			PreviousStatus: types.ServiceStatusRunning,
		},
	}

	err := job.Handle(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "runtime was rolled back")
	require.Equal(t, 2, dispatcher.ExecutionCount())
	assert.Equal(t, "Rollback Default PHP to 8.2", dispatcher.LastExecution().Task.Name())
}

func TestSetDefaultPhpJobUsesDetectedRuntimeWhenNoDefaultIsRecorded(t *testing.T) {
	patchJob, service, db, dispatcher, _ := newPatchPhpVersionTestJob(t)
	dispatcher.DefaultResult.Output = "LAUNCH_PREVIOUS_DEFAULT_PHP_VERSION=8.1\n"

	// Simulate a persistence race after the remote runtime was switched. There
	// is intentionally no is_default row to exercise legacy/corrupt state.
	require.NoError(t, db.Delete(&models.InstalledService{}, "id = ?", service.ID).Error)
	job := &SetDefaultPhpJob{
		Deps: patchJob.Deps,
		Payload: SetDefaultPhpPayload{
			ServerID:       "server-a",
			ServiceID:      service.ID,
			Version:        "8.3",
			PreviousStatus: types.ServiceStatusRunning,
		},
	}

	err := job.Handle(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "runtime was rolled back")
	require.Equal(t, 2, dispatcher.ExecutionCount())
	assert.Equal(t, "Rollback Default PHP to 8.1", dispatcher.LastExecution().Task.Name())
}

func TestSetDefaultPhpFailedClearsReservation(t *testing.T) {
	patchJob, service, db, _, _ := newPatchPhpVersionTestJob(t)
	job := &SetDefaultPhpJob{
		Deps: patchJob.Deps,
		Payload: SetDefaultPhpPayload{
			ServerID:       "server-a",
			ServiceID:      service.ID,
			Version:        "8.3",
			PreviousStatus: types.ServiceStatusRunning,
		},
		server: patchJob.Deps.Repos.Server().(*patchJobServerRepository).server,
	}

	job.Failed(context.Background(), errors.New("ssh unavailable"))

	var server models.Server
	require.NoError(t, db.First(&server, "id = ?", "server-a").Error)
	assert.Nil(t, server.PendingDefaultPHPServiceID)
}

func TestSetDefaultPhpJobRejectsUpdatingService(t *testing.T) {
	patchJob, service, _, dispatcher, _ := newPatchPhpVersionTestJob(t)
	service.Status = types.ServiceStatusRunning
	job := &SetDefaultPhpJob{
		Deps: patchJob.Deps,
		Payload: SetDefaultPhpPayload{
			ServerID:       "server-a",
			ServiceID:      service.ID,
			Version:        "8.3",
			PreviousStatus: types.ServiceStatusRunning,
		},
	}

	err := job.Handle(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "target service reservation was lost")
	assert.Zero(t, dispatcher.ExecutionCount())
}

func newPatchPhpVersionTestJob(
	t *testing.T,
) (*PatchPhpVersionJob, *models.InstalledService, *gorm.DB, *taskrunner.FakeDispatcher, *patchRecordingBroadcaster) {
	t.Helper()

	registerPatchTemplatesOnce.Do(func() {
		tasktemplates.Reset()
		registerPatchTemplatesErr = tasktemplates.Register("server", servertemplates.FS, nil)
	})
	require.NoError(t, registerPatchTemplatesErr)
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))

	db, err := gorm.Open(
		sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"),
		&gorm.Config{DisableForeignKeyConstraintWhenMigrating: true},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Server{},
		&models.InstalledService{},
		&models.Task{},
	))

	ipAddress := "192.0.2.10"
	serviceID := "service-php83"
	server := &models.Server{
		BaseModel:                  basemodels.BaseModel{ID: "server-a"},
		TeamScoped:                 basemodels.TeamScoped{TeamID: "team-a"},
		PublicIPv4:                 &ipAddress,
		PrivateKey:                 dbtype.EncryptedString("private-key"),
		PendingDefaultPHPServiceID: &serviceID,
	}
	service := &models.InstalledService{
		BaseModel:    basemodels.BaseModel{ID: serviceID},
		ServerScoped: basemodels.ServerScoped{ServerID: server.ID},
		Type:         types.ServiceTypePhp,
		TypeData:     dbtype.JSONMap{"extensions": map[string]any{"redis": map[string]any{"status": "installed"}}},
		Name:         "PHP 8.3",
		Version:      "8.3.6",
		Status:       types.ServiceStatusUpdating,
		Software:     types.SoftwarePhp83.String(),
	}
	require.NoError(t, db.Create(server).Error)
	require.NoError(t, db.Create(service).Error)

	serverRepo := &patchJobServerRepository{server: server}
	serviceRepo := &patchJobServiceRepository{
		service:  service,
		services: []models.InstalledService{*service},
	}
	registry := &patchJobRegistry{
		serverRepository:  serverRepo,
		serviceRepository: serviceRepo,
		db:                db,
	}
	logger := zerolog.New(io.Discard)
	dispatcher := taskrunner.NewFakeDispatcher()
	broadcaster := &patchRecordingBroadcaster{}
	dependencies := &JobDeps{
		Deps: &pkgjobs.Deps{
			DB:          db,
			Logger:      &logger,
			Broadcaster: broadcaster,
			Dispatcher:  dispatcher,
		},
		Repos: registry,
		TaskRunnerDeps: &tasks.TaskRunnerDeps{
			DB:          db,
			Dispatcher:  dispatcher,
			Logger:      &logger,
			Broadcaster: broadcaster,
		},
	}

	job := &PatchPhpVersionJob{
		Deps: dependencies,
		Payload: PatchPhpVersionPayload{
			ServerID:       server.ID,
			ServiceID:      service.ID,
			PreviousStatus: types.ServiceStatusRunning,
		},
	}
	return job, service, db, dispatcher, broadcaster
}
