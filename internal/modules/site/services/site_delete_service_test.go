package services

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/config"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/modules/site/jobs"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/pkg/queue"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

func siteDeleteFixture(t *testing.T) (*SiteService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true, Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&servermodels.Server{}, &models.Site{}))
	server := servermodels.Server{BaseModel: basemodels.BaseModel{ID: "server"}, TeamScoped: basemodels.TeamScoped{TeamID: "team"}, Status: servertypes.ServerStatusRunning}
	require.NoError(t, db.Create(&server).Error)
	site := models.Site{BaseModel: basemodels.BaseModel{ID: "site"}, ServerScoped: basemodels.ServerScoped{ServerID: server.ID}, TeamScoped: basemodels.TeamScoped{TeamID: server.TeamID}, Address: "example.test"}
	require.NoError(t, db.Create(&site).Error)
	log := zerolog.Nop()
	svc := NewSiteService(&ServiceDeps{ModuleDeps: service.ModuleDeps[*repositories.Registry]{Dependencies: service.Dependencies{DB: db, Logger: &log}, Repos: repositories.NewRegistry(db)}})
	return svc, db
}

func attachSiteDeleteQueue(t *testing.T, svc *SiteService) *miniredis.Miniredis {
	t.Helper()
	redis := miniredis.RunT(t)
	svc.Queue = queue.NewClient(config.RedisConfig{Address: redis.Addr()})
	t.Cleanup(func() { require.NoError(t, svc.Queue.Close()) })
	return redis
}

func TestDeleteSiteQueuesStaffActorAndRejectsDuplicate(t *testing.T) {
	svc, db := siteDeleteFixture(t)
	attachSiteDeleteQueue(t, svc)
	require.NoError(t, svc.DeleteSite(context.Background(), "site", "server", "team", "staff-admin"))
	require.Error(t, svc.DeleteSite(context.Background(), "site", "server", "team", "staff-admin"))
	tasks, err := svc.Queue.Inspector().ListPendingTasks(queue.QueueDefault)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	var payload jobs.UninstallSitePayload
	require.NoError(t, json.Unmarshal(tasks[0].Payload, &payload))
	require.Equal(t, "site", payload.SiteID)
	require.Equal(t, "server", payload.ServerID)
	require.Equal(t, "team", payload.TeamID)
	require.NotNil(t, payload.UserID)
	require.Equal(t, "staff-admin", *payload.UserID)
	var site models.Site
	require.NoError(t, db.First(&site, "id = ?", "site").Error)
	require.NotNil(t, site.UninstallationRequestedAt)
	require.NotNil(t, payload.RequestedAt)
	require.True(t, site.UninstallationRequestedAt.Equal(*payload.RequestedAt))
}

func TestRemoveSiteRecordDoesNotQueueCleanup(t *testing.T) {
	svc, db := siteDeleteFixture(t)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)
	require.NoError(t, db.Exec("CREATE TABLE site_children (site_id text REFERENCES sites(id) ON DELETE CASCADE)").Error)
	require.NoError(t, db.Exec("INSERT INTO site_children VALUES ('site')").Error)
	ctx := context.Background()
	require.Error(t, svc.RemoveSiteRecord(ctx, "site", "server", "other-team", "admin"))
	require.NoError(t, db.Exec("UPDATE sites SET uninstallation_requested_at = CURRENT_TIMESTAMP").Error)
	require.Error(t, svc.RemoveSiteRecord(ctx, "site", "server", "team", "admin"))
	require.NoError(t, db.Exec("UPDATE sites SET uninstallation_requested_at = NULL").Error)
	require.NoError(t, svc.RemoveSiteRecord(ctx, "site", "server", "team", "admin"))
	var count int64
	require.NoError(t, db.Table("sites").Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.Table("site_children").Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.First(&servermodels.Server{}, "id = ?", "server").Error)
}

func TestDeleteSiteQueueFailureRestoresStateAndAllowsRetry(t *testing.T) {
	for _, configured := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing queue", true: "queue rejected"}[configured], func(t *testing.T) {
			svc, db := siteDeleteFixture(t)
			previousFailure := time.Now().UTC().Add(-time.Hour)
			require.NoError(t, db.Model(&models.Site{}).Where("id = ?", "site").Update("uninstallation_failed_at", previousFailure).Error)
			var redis *miniredis.Miniredis
			if configured {
				redis = attachSiteDeleteQueue(t, svc)
				redis.SetError("ERR queue unavailable")
			}
			require.Error(t, svc.DeleteSite(context.Background(), "site", "server", "team", "admin"))
			var site models.Site
			require.NoError(t, db.First(&site, "id = ?", "site").Error)
			require.Nil(t, site.UninstallationRequestedAt)
			require.NotNil(t, site.UninstallationFailedAt)
			require.True(t, site.UninstallationFailedAt.Equal(previousFailure))
			if configured {
				redis.SetError("")
			} else {
				attachSiteDeleteQueue(t, svc)
			}
			require.NoError(t, svc.DeleteSite(context.Background(), "site", "server", "team", "admin"))
			site = models.Site{}
			require.NoError(t, db.First(&site, "id = ?", "site").Error)
			require.NotNil(t, site.UninstallationRequestedAt)
			require.Nil(t, site.UninstallationFailedAt)
		})
	}
}

func TestDeleteSiteRejectsWrongOwnershipBeforeQueue(t *testing.T) {
	for _, tc := range []struct{ name, server, team, setup string }{
		{"wrong server", "other", "team", ""},
		{"wrong team", "server", "other", ""},
		{"server ownership mismatch", "server", "team", "UPDATE servers SET team_id = 'other'"},
		{"server deleting", "server", "team", "UPDATE servers SET status = 'deleting'"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, db := siteDeleteFixture(t)
			attachSiteDeleteQueue(t, svc)
			if tc.setup != "" {
				require.NoError(t, db.Exec(tc.setup).Error)
			}
			require.Error(t, svc.DeleteSite(context.Background(), "site", tc.server, tc.team, "admin"))
			var site models.Site
			require.NoError(t, db.First(&site, "id = ?", "site").Error)
			require.Nil(t, site.UninstallationRequestedAt)
			queues, err := svc.Queue.Inspector().Queues()
			require.NoError(t, err)
			require.Empty(t, queues)
		})
	}
}
