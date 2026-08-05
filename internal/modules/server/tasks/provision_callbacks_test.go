package tasks

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/database/serializers"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

func provisionCallbackDB(t *testing.T) *gorm.DB {
	t.Helper()
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))

	db, err := gorm.Open(
		sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"),
		&gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Server{},
		&models.FirewallRule{},
		&models.InstalledService{},
		&models.Task{},
	))
	return db
}

func provisionCallbackContext(t *testing.T, db *gorm.DB, broadcaster *recoveryBroadcaster) *taskrunner.CallbackContext {
	t.Helper()
	cbCtx := &taskrunner.CallbackContext{
		DB:     db,
		Logger: recoveryLogger(),
	}
	cbCtx.SetBroadcaster(broadcaster)
	return cbCtx
}

func seedProvisioningServer(t *testing.T, db *gorm.DB) {
	t.Helper()
	server := &models.Server{Name: "web-1", Status: types.ServerStatusProvisioning}
	server.ID = "server-123"
	server.TeamID = "team-123"
	require.NoError(t, db.Create(server).Error)
}

func storedServer(t *testing.T, db *gorm.DB) models.Server {
	t.Helper()
	var server models.Server
	require.NoError(t, db.Select("id", "status", "progress").
		Where("id = ?", "server-123").First(&server).Error)
	return server
}

// =============================================================================
// Task identity and payload round-trip
// =============================================================================

// The Instance payload is what recovery reconstructs a handler from after a
// worker restart, so it has to survive a marshal/unmarshal round trip.
func TestProvisionFreshServerTaskPayloadRoundTrip(t *testing.T) {
	task := ProvisionFreshServer(testConfig())
	require.NotNil(t, task)

	assert.Equal(t, ProvisionFreshServerTaskType, task.TypeName())

	payload, err := task.MarshalPayload()
	require.NoError(t, err)

	var restored provisionCallbackData
	require.NoError(t, json.Unmarshal(payload, &restored))
	assert.Equal(t, "server-123", restored.ServerID)
	assert.Equal(t, "team-123", restored.TeamID)
	assert.Equal(t, "Test Server", restored.ServerName)
	assert.Equal(t, "192.168.1.100", restored.ServerIP)
	assert.Equal(t, "launch", restored.ServerUsername)

	handler := restored.NewTask()
	require.NotNil(t, handler)
}

func TestProvisionFreshServerBuildsAScript(t *testing.T) {
	task := ProvisionFreshServer(testConfig())
	require.NotNil(t, task)

	script := task.Script()
	assert.Contains(t, script, "#!/bin/bash")
	assert.Contains(t, script, "quiesceAptForProvisioning")
	assert.Contains(t, script, "function aptGet()")
}

// =============================================================================
// ProvisionFreshServerTask.OnSuccess
// =============================================================================

func TestProvisionFreshServerOnSuccess(t *testing.T) {
	db := provisionCallbackDB(t)
	seedProvisioningServer(t, db)

	rule := &models.FirewallRule{Name: "http", Port: "80"}
	rule.ID = "fw-1"
	rule.ServerID = "server-123"
	require.NoError(t, db.Create(rule).Error)

	service := &models.InstalledService{Software: "redis", Status: types.ServiceStatusInstalling}
	service.ID = "svc-1"
	service.ServerID = "server-123"
	require.NoError(t, db.Create(service).Error)

	broadcaster := &recoveryBroadcaster{}
	task := ProvisionFreshServer(testConfig())

	require.NoError(t, task.OnSuccess(context.Background(),
		provisionCallbackContext(t, db, broadcaster), "task-1"))

	server := storedServer(t, db)
	assert.Equal(t, types.ServerStatusRunning, server.Status)
	assert.Equal(t, 100, server.Progress)

	var storedRule models.FirewallRule
	require.NoError(t, db.First(&storedRule, "id = ?", "fw-1").Error)
	assert.NotNil(t, storedRule.InstalledAt, "rules configured during provisioning are marked installed")

	var storedService models.InstalledService
	require.NoError(t, db.First(&storedService, "id = ?", "svc-1").Error)
	assert.Equal(t, types.ServiceStatusRunning, storedService.Status)

	assert.Equal(t, []string{"server.provisioned"}, eventNames(broadcaster))
}

func TestProvisionFreshServerOnSuccessFailsWhenServerUpdateFails(t *testing.T) {
	db := provisionCallbackDB(t)
	require.NoError(t, db.Migrator().DropTable(&models.Server{}))

	task := ProvisionFreshServer(testConfig())
	err := task.OnSuccess(context.Background(),
		provisionCallbackContext(t, db, &recoveryBroadcaster{}), "task-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update server status")
}

// Firewall and service bookkeeping is best-effort: the server is provisioned
// either way, so a failure there is logged rather than returned.
func TestProvisionFreshServerOnSuccessToleratesBookkeepingFailures(t *testing.T) {
	db := provisionCallbackDB(t)
	seedProvisioningServer(t, db)
	require.NoError(t, db.Migrator().DropTable(&models.FirewallRule{}))
	require.NoError(t, db.Migrator().DropTable(&models.InstalledService{}))

	broadcaster := &recoveryBroadcaster{}
	task := ProvisionFreshServer(testConfig())

	require.NoError(t, task.OnSuccess(context.Background(),
		provisionCallbackContext(t, db, broadcaster), "task-1"))

	assert.Equal(t, types.ServerStatusRunning, storedServer(t, db).Status)
	assert.NotEmpty(t, broadcaster.events)
}

// =============================================================================
// ProvisionFreshServerTask.OnFailure / OnExpired
// =============================================================================

func TestProvisionFreshServerTerminalFailurePaths(t *testing.T) {
	tests := []struct {
		name      string
		invoke    func(*ProvisionFreshServerTask, *taskrunner.CallbackContext) error
		wantEvent string
	}{
		{
			name: "failure",
			invoke: func(task *ProvisionFreshServerTask, cbCtx *taskrunner.CallbackContext) error {
				return task.OnFailure(context.Background(), cbCtx, "task-1", 3)
			},
			wantEvent: "server.provision_failed",
		},
		{
			name: "timeout",
			invoke: func(task *ProvisionFreshServerTask, cbCtx *taskrunner.CallbackContext) error {
				return task.OnExpired(context.Background(), cbCtx, "task-1")
			},
			wantEvent: "server.provision_timeout",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := provisionCallbackDB(t)
			seedProvisioningServer(t, db)

			broadcaster := &recoveryBroadcaster{}
			task := ProvisionFreshServer(testConfig())

			require.NoError(t, tc.invoke(task, provisionCallbackContext(t, db, broadcaster)))

			assert.Equal(t, types.ServerStatusFailed, storedServer(t, db).Status)
			assert.Equal(t, []string{tc.wantEvent}, eventNames(broadcaster))
		})
	}
}

func TestProvisionFreshServerTerminalPathsSurfaceUpdateFailures(t *testing.T) {
	db := provisionCallbackDB(t)
	require.NoError(t, db.Migrator().DropTable(&models.Server{}))

	task := ProvisionFreshServer(testConfig())
	cbCtx := provisionCallbackContext(t, db, &recoveryBroadcaster{})

	assert.Error(t, task.OnFailure(context.Background(), cbCtx, "task-1", 3))
	assert.Error(t, task.OnExpired(context.Background(), cbCtx, "task-1"))
}

// Cleanup runs through the queue; without one it logs and moves on rather
// than failing the callback.
func TestProvisionFreshServerDispatchCleanupJobWithoutQueue(t *testing.T) {
	db := provisionCallbackDB(t)
	task := ProvisionFreshServer(testConfig())

	assert.NotPanics(t, func() {
		task.dispatchCleanupJob(provisionCallbackContext(t, db, &recoveryBroadcaster{}))
	})
}

// =============================================================================
// ProvisionDockerServerTask
// =============================================================================

func dockerConfig() ProvisionDockerServerConfig {
	base := testConfig()
	return ProvisionDockerServerConfig{
		ServerID:         base.ServerID,
		TeamID:           base.TeamID,
		ServerName:       base.ServerName,
		MemoryInMB:       base.MemoryInMB,
		PublicIPv4:       base.PublicIPv4,
		Provider:         base.Provider,
		PublicKey:        base.PublicKey,
		Username:         base.Username,
		Password:         base.Password,
		WorkingDirectory: base.WorkingDirectory,
		AppURL:           base.AppURL,
		AppName:          base.AppName,
		SSHPort:          base.SSHPort,
		NetworkName:      "launch-network",
		RootDir:          "/etc/launch",
		TraefikVersion:   "v3.1",
	}
}

func TestProvisionDockerServerOnSuccess(t *testing.T) {
	db := provisionCallbackDB(t)
	seedProvisioningServer(t, db)

	broadcaster := &recoveryBroadcaster{}
	task := ProvisionDockerServer(dockerConfig())
	require.NotNil(t, task)

	require.NoError(t, task.OnSuccess(context.Background(),
		provisionCallbackContext(t, db, broadcaster), "task-1"))

	server := storedServer(t, db)
	assert.Equal(t, types.ServerStatusRunning, server.Status)
	assert.Equal(t, 100, server.Progress)
	assert.Equal(t, []string{"server.provisioned"}, eventNames(broadcaster))
}

func TestProvisionDockerServerTerminalFailurePaths(t *testing.T) {
	tests := []struct {
		name      string
		invoke    func(*ProvisionDockerServerTask, *taskrunner.CallbackContext) error
		wantEvent string
	}{
		{
			name: "failure",
			invoke: func(task *ProvisionDockerServerTask, cbCtx *taskrunner.CallbackContext) error {
				return task.OnFailure(context.Background(), cbCtx, "task-1", 3)
			},
			wantEvent: "server.provision_failed",
		},
		{
			name: "timeout",
			invoke: func(task *ProvisionDockerServerTask, cbCtx *taskrunner.CallbackContext) error {
				return task.OnExpired(context.Background(), cbCtx, "task-1")
			},
			wantEvent: "server.provision_timeout",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := provisionCallbackDB(t)
			seedProvisioningServer(t, db)

			broadcaster := &recoveryBroadcaster{}
			task := ProvisionDockerServer(dockerConfig())

			require.NoError(t, tc.invoke(task, provisionCallbackContext(t, db, broadcaster)))

			assert.Equal(t, types.ServerStatusFailed, storedServer(t, db).Status)
			assert.Equal(t, []string{tc.wantEvent}, eventNames(broadcaster))
		})
	}
}

func TestProvisionDockerServerSurfacesUpdateFailures(t *testing.T) {
	db := provisionCallbackDB(t)
	require.NoError(t, db.Migrator().DropTable(&models.Server{}))

	task := ProvisionDockerServer(dockerConfig())
	cbCtx := provisionCallbackContext(t, db, &recoveryBroadcaster{})

	assert.Error(t, task.OnSuccess(context.Background(), cbCtx, "task-1"))
	assert.Error(t, task.OnFailure(context.Background(), cbCtx, "task-1", 3))
	assert.Error(t, task.OnExpired(context.Background(), cbCtx, "task-1"))
}

func TestProvisionDockerServerPayloadRoundTrip(t *testing.T) {
	task := ProvisionDockerServer(dockerConfig())
	require.NotNil(t, task)

	assert.Equal(t, ProvisionDockerServerTaskType, task.TypeName())

	payload, err := task.MarshalPayload()
	require.NoError(t, err)

	var restored provisionDockerCallbackData
	require.NoError(t, json.Unmarshal(payload, &restored))
	assert.Equal(t, "server-123", restored.ServerID)
	assert.Equal(t, "team-123", restored.TeamID)

	require.NotNil(t, restored.NewTask())
}

func TestProvisionDockerServerDispatchCleanupJobWithoutQueue(t *testing.T) {
	db := provisionCallbackDB(t)
	task := ProvisionDockerServer(dockerConfig())

	assert.NotPanics(t, func() {
		task.dispatchCleanupJob(provisionCallbackContext(t, db, &recoveryBroadcaster{}))
	})
}
