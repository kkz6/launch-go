package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/server/jobs"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

func serverDeleteFixture(t *testing.T, provider types.ServerProvider) (*Service, *gorm.DB, models.Server) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true, Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&models.Server{}, &models.InstalledService{}))
	require.NoError(t, db.Exec("CREATE TABLE sites (id text PRIMARY KEY, server_id text)").Error)
	require.NoError(t, db.Exec("CREATE TABLE docker_projects (id text PRIMARY KEY, server_id text, deleted_at timestamp)").Error)
	server := models.Server{BaseModel: basemodels.BaseModel{ID: "server-delete"}, TeamScoped: basemodels.TeamScoped{TeamID: "customer-team"}, Name: "Customer server", Provider: provider, Status: types.ServerStatusRunning}
	require.NoError(t, db.Create(&server).Error)
	log := zerolog.Nop()
	svc := NewService(ServiceDeps{Dependencies: service.Dependencies{DB: db, Logger: &log}, Repos: repositories.NewRegistry(db)})
	return svc, db, server
}

func TestDeleteServerChecksResourcesAndScopeBeforeMutation(t *testing.T) {
	for _, tc := range []struct{ name, setup, team string }{
		{"sites", "INSERT INTO sites VALUES ('site', 'server-delete')", "customer-team"},
		{"projects", "INSERT INTO docker_projects (id, server_id) VALUES ('project', 'server-delete')", "customer-team"},
		{"wrong team", "", "other-team"},
		{"provisioning", "UPDATE servers SET status = 'provisioning'", "customer-team"},
		{"starting", "UPDATE servers SET status = 'starting'", "customer-team"},
		{"already deleting", "UPDATE servers SET status = 'deleting'", "customer-team"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, db, server := serverDeleteFixture(t, types.ProviderCustom)
			if tc.setup != "" {
				require.NoError(t, db.Exec(tc.setup).Error)
			}
			var before models.Server
			require.NoError(t, db.First(&before, "id = ?", server.ID).Error)
			require.Error(t, svc.DeleteServer(context.Background(), server.ID, tc.team, "admin"))
			var after models.Server
			require.NoError(t, db.First(&after, "id = ?", server.ID).Error)
			require.Equal(t, before.Status, after.Status)
		})
	}
}

func TestDeleteCustomServerOnlyRemovesRequestedRecord(t *testing.T) {
	svc, db, server := serverDeleteFixture(t, types.ProviderCustom)
	require.NoError(t, db.Exec("INSERT INTO docker_projects VALUES ('old-project', 'server-delete', CURRENT_TIMESTAMP)").Error)
	other := server
	other.ID = "other-server"
	require.NoError(t, db.Create(&other).Error)
	require.NoError(t, svc.DeleteServer(context.Background(), server.ID, server.TeamID, "admin"))
	var count int64
	require.NoError(t, db.Model(&models.Server{}).Where("id = ?", server.ID).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.First(&models.Server{}, "id = ?", other.ID).Error)
}

func TestCloudServerDeleteQueuesActorAndRejectsDuplicate(t *testing.T) {
	svc, db, server := serverDeleteFixture(t, types.ProviderHetzner)
	attachTestQueue(t, svc)
	require.NoError(t, svc.DeleteServer(context.Background(), server.ID, server.TeamID, "staff-admin"))
	require.Error(t, svc.DeleteServer(context.Background(), server.ID, server.TeamID, "staff-admin"))
	tasks, err := svc.Queue.Inspector().ListPendingTasks(queue.QueueDefault)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	var payload jobs.DeleteServerPayload
	require.NoError(t, json.Unmarshal(tasks[0].Payload, &payload))
	require.Equal(t, server.TeamID, payload.TeamID)
	require.NotNil(t, payload.UserID)
	require.Equal(t, "staff-admin", *payload.UserID)
	var stored models.Server
	require.NoError(t, db.First(&stored, "id = ?", server.ID).Error)
	require.Equal(t, types.ServerStatusDeleting, stored.Status)
}

func TestCloudServerQueueFailurePreservesRecordAndStatus(t *testing.T) {
	for _, configured := range []bool{false, true} {
		svc, db, server := serverDeleteFixture(t, types.ProviderHetzner)
		if configured {
			redis := miniredis.RunT(t)
			redis.SetError("ERR queue unavailable")
			svc.Queue = queue.NewClient(config.RedisConfig{Address: redis.Addr()})
			t.Cleanup(func() { require.NoError(t, svc.Queue.Close()) })
		}
		require.Error(t, svc.DeleteServer(context.Background(), server.ID, server.TeamID, "admin"))
		var stored models.Server
		require.NoError(t, db.First(&stored, "id = ?", server.ID).Error)
		require.Equal(t, types.ServerStatusRunning, stored.Status)
	}
}
