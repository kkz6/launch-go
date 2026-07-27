package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/database/serializers"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	sitemodels "github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/modules/staff/repositories"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// setupFailuresDB returns an in-memory sqlite DB with the servers, tasks and
// deployments tables. All three AutoMigrate cleanly under sqlite (encrypted
// columns are plain text/longtext, json columns are text). A 32-byte
// encryption key is configured so the Task.Output EncryptedString round-trips
// through Value/Scan exactly as it does in production.
func setupFailuresDB(t *testing.T) *gorm.DB {
	t.Helper()

	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&servermodels.Server{},
		&servermodels.Task{},
		&sitemodels.Deployment{},
	))

	return db
}

func intPtr(v int) *int       { return &v }
func strPtr(v string) *string { return &v }

func newFailuresService(db *gorm.DB) *Service {
	return NewService(repositories.NewRegistry(db))
}

func TestService_Failures_SurfacesAllKindsNewestFirst(t *testing.T) {
	db := setupFailuresDB(t)
	svc := newFailuresService(db)
	ctx := context.Background()

	now := time.Now()
	tProvision := now.Add(-1 * time.Minute)
	tInstall := now.Add(-2 * time.Minute)
	tDepTask := now.Add(-3 * time.Minute)
	tDeployment := now.Add(-4 * time.Minute)

	// Failed server (provision_error set, status not failed) → kind=provision.
	server := servermodels.Server{
		BaseModel:      basemodels.BaseModel{ID: "srv00000000000000000000001", CreatedAt: &tProvision, UpdatedAt: &tProvision},
		Name:           "web-1",
		Status:         servertypes.ServerStatusRunning,
		ProvisionError: strPtr("apt-get failed"),
	}
	server.TeamID = "team00000000000000000000001"
	require.NoError(t, db.Create(&server).Error)

	// Failed service-installation task ("Install …" name) → kind=service_installation.
	installTask := servermodels.Task{
		BaseModel: basemodels.BaseModel{ID: "tsk00000000000000000000001", CreatedAt: &tInstall, UpdatedAt: &tInstall},
		Name:      "Install PHP 8.3",
		User:      "root",
		Type:      "github.com/kkz6/launch-go/internal/pkg/taskrunner.BaseTask",
		Status:    string(servertypes.TaskStatusFailed),
		Output:    dbtype.EncryptedString("apt install php8.3 failed"),
		ExitCode:  intPtr(1),
	}
	installTask.ServerID = server.ID
	require.NoError(t, db.Create(&installTask).Error)

	// A non-install task must NOT surface (the monitor is scoped to the three
	// categories only).
	noiseTask := servermodels.Task{
		BaseModel: basemodels.BaseModel{ID: "tsk00000000000000000000099", CreatedAt: &tInstall, UpdatedAt: &tInstall},
		Name:      "Restart Caddy",
		User:      "root",
		Type:      "github.com/kkz6/launch-go/internal/pkg/taskrunner.BaseTask",
		Status:    string(servertypes.TaskStatusFailed),
		ExitCode:  intPtr(1),
	}
	noiseTask.ServerID = server.ID
	require.NoError(t, db.Create(&noiseTask).Error)

	// Failed deployment backed by its own failed task → kind=site_installation.
	depTask := servermodels.Task{
		BaseModel: basemodels.BaseModel{ID: "tsk00000000000000000000002", CreatedAt: &tDepTask, UpdatedAt: &tDepTask},
		Name:      "Deploy",
		User:      "deploy",
		Type:      "github.com/kkz6/launch-go/internal/modules/site/tasks.deploySiteTask",
		Status:    string(servertypes.TaskStatusFailed),
		Output:    dbtype.EncryptedString("npm build failed"),
		ExitCode:  intPtr(2),
	}
	depTask.ServerID = server.ID
	require.NoError(t, db.Create(&depTask).Error)

	deployment := sitemodels.Deployment{
		BaseModel: basemodels.BaseModel{ID: "dep00000000000000000000001", CreatedAt: &tDeployment, UpdatedAt: &tDeployment},
		TaskID:    strPtr(depTask.ID),
		Status:    sitetypes.DeploymentStatusFailed,
	}
	deployment.SiteID = "site0000000000000000000001"
	deployment.TeamID = "team00000000000000000000001"
	require.NoError(t, db.Create(&deployment).Error)

	resp, total, err := svc.Failures(ctx, "", 25, 0)
	require.NoError(t, err)
	// 3 rows: the failed server (provision), the install task (service
	// installation) and the failed deployment (site installation). The
	// "Restart Caddy" task and the deployment's backing deploySiteTask are
	// excluded — only the three scoped categories surface.
	require.Equal(t, int64(3), total)
	require.Len(t, resp.Failures, 3)

	// Newest-first: provision (-1m), service install (-2m), deployment (-4m).
	require.Equal(t, KindProvision, resp.Failures[0].Kind)
	require.Equal(t, KindServiceInstallation, resp.Failures[1].Kind)
	require.Equal(t, KindSiteInstallation, resp.Failures[2].Kind)

	// Provision mapping.
	prov := resp.Failures[0]
	require.Equal(t, server.ID, prov.ID)
	require.Equal(t, "web-1", prov.Title)
	require.Equal(t, server.ID, prov.ServerID)
	require.Equal(t, "team00000000000000000000001", prov.TeamID)
	require.Equal(t, "apt-get failed", prov.Error)

	// Service-install mapping: title is the human Name (not the reflect type),
	// exit code surfaced, output decrypted into Detail.
	installFailure := resp.Failures[1]
	require.Equal(t, installTask.ID, installFailure.ID)
	require.Equal(t, "Install PHP 8.3", installFailure.Title)
	require.Equal(t, server.ID, installFailure.ServerID)
	require.Equal(t, "exit code 1", installFailure.Error)
	require.Equal(t, "apt install php8.3 failed", installFailure.Detail)

	// Deployment mapping: joined task output decrypted, exit code surfaced.
	dep := resp.Failures[2]
	require.Equal(t, deployment.ID, dep.ID)
	require.Equal(t, "site site0000000000000000000001", dep.Title)
	require.Equal(t, "exit code 2", dep.Error)
	require.Equal(t, "npm build failed", dep.Detail)

	require.NotEmpty(t, resp.Caveat)
}

func TestService_Failures_KindFilterIsolatesSource(t *testing.T) {
	db := setupFailuresDB(t)
	svc := newFailuresService(db)
	ctx := context.Background()

	now := time.Now()

	server := servermodels.Server{
		BaseModel: basemodels.BaseModel{ID: "srv00000000000000000000010", CreatedAt: &now, UpdatedAt: &now},
		Name:      "db-1",
		Status:    servertypes.ServerStatusFailed,
	}
	server.TeamID = "team00000000000000000000010"
	require.NoError(t, db.Create(&server).Error)

	task := servermodels.Task{
		BaseModel: basemodels.BaseModel{ID: "tsk00000000000000000000010", CreatedAt: &now, UpdatedAt: &now},
		Name:      "Install PHP",
		User:      "root",
		Type:      "server:install_php",
		Status:    string(servertypes.TaskStatusTimeout),
	}
	task.ServerID = server.ID
	require.NoError(t, db.Create(&task).Error)

	resp, total, err := svc.Failures(ctx, "provision", 25, 0)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, resp.Failures, 1)
	require.Equal(t, "provision", resp.Failures[0].Kind)
	require.Equal(t, "provisioning failed", resp.Failures[0].Error)
}

func TestService_Failures_TimeoutTaskFallsBackToStatus(t *testing.T) {
	db := setupFailuresDB(t)
	svc := newFailuresService(db)
	ctx := context.Background()

	now := time.Now()
	task := servermodels.Task{
		BaseModel: basemodels.BaseModel{ID: "tsk00000000000000000000020", CreatedAt: &now, UpdatedAt: &now},
		Name:      "Install Redis",
		User:      "root",
		Type:      "github.com/kkz6/launch-go/internal/pkg/taskrunner.BaseTask",
		Status:    string(servertypes.TaskStatusTimeout),
		// no exit code -> error falls back to status string
	}
	task.ServerID = "srv00000000000000000000020"
	require.NoError(t, db.Create(&task).Error)

	resp, _, err := svc.Failures(ctx, KindServiceInstallation, 25, 0)
	require.NoError(t, err)
	require.Len(t, resp.Failures, 1)
	require.Equal(t, "timeout", resp.Failures[0].Error)
}

func TestService_Failures_DetailTruncation(t *testing.T) {
	db := setupFailuresDB(t)
	svc := newFailuresService(db)
	ctx := context.Background()

	now := time.Now()
	bigOutput := strings.Repeat("x", 5000)

	task := servermodels.Task{
		BaseModel: basemodels.BaseModel{ID: "tsk00000000000000000000030", CreatedAt: &now, UpdatedAt: &now},
		Name:      "Install Composer 2",
		User:      "root",
		Type:      "github.com/kkz6/launch-go/internal/pkg/taskrunner.BaseTask",
		Status:    string(servertypes.TaskStatusFailed),
		Output:    dbtype.EncryptedString(bigOutput),
		ExitCode:  intPtr(1),
	}
	task.ServerID = "srv00000000000000000000030"
	require.NoError(t, db.Create(&task).Error)

	resp, _, err := svc.Failures(ctx, KindServiceInstallation, 25, 0)
	require.NoError(t, err)
	require.Len(t, resp.Failures, 1)

	detail := resp.Failures[0].Detail
	require.True(t, strings.HasSuffix(detail, "…"), "detail should end with truncation marker")
	// 2048 bytes of content + 3-byte ellipsis rune.
	require.LessOrEqual(t, len(detail), detailMaxBytes+len("…"))
	require.Greater(t, len(detail), detailMaxBytes-4)
}

func TestService_Failures_IncludesLaunchAgentUpdates(t *testing.T) {
	db := setupFailuresDB(t)
	svc := newFailuresService(db)
	ctx := context.Background()

	now := time.Now()
	task := servermodels.Task{
		BaseModel: basemodels.BaseModel{ID: "tsk00000000000000000000031", CreatedAt: &now, UpdatedAt: &now},
		Name:      "Update Launch Agent",
		User:      "root",
		Type:      "github.com/kkz6/launch-go/internal/pkg/taskrunner.BaseTask",
		Status:    string(servertypes.TaskStatusFailed),
		Output:    dbtype.EncryptedString("installer download failed"),
		ExitCode:  intPtr(1),
	}
	task.ServerID = "srv00000000000000000000031"
	require.NoError(t, db.Create(&task).Error)

	resp, _, err := svc.Failures(ctx, KindServiceInstallation, 25, 0)
	require.NoError(t, err)
	require.Len(t, resp.Failures, 1)
	require.Equal(t, "Update Launch Agent", resp.Failures[0].Title)
	require.Equal(t, "installer download failed", resp.Failures[0].Detail)
}

func TestTruncate(t *testing.T) {
	require.Equal(t, "short", truncate("short", 2048))
	require.Equal(t, "", truncate("", 2048))

	out := truncate(strings.Repeat("a", 100), 10)
	require.Equal(t, "aaaaaaaaaa…", out)

	// Multi-byte rune at the cut boundary is dropped cleanly, not split.
	multi := strings.Repeat("あ", 100) // 3 bytes each
	cut := truncate(multi, 10)
	require.True(t, strings.HasSuffix(cut, "…"))
	require.True(t, utf8ValidLastRune(strings.TrimSuffix(cut, "…")))
}

func TestService_Failures_Pagination(t *testing.T) {
	db := setupFailuresDB(t)
	svc := newFailuresService(db)
	ctx := context.Background()

	base := time.Now()
	for i := 0; i < 5; i++ {
		ts := base.Add(time.Duration(-i) * time.Minute)
		s := servermodels.Server{
			BaseModel:      basemodels.BaseModel{ID: "srvpage000000000000000000" + string(rune('A'+i)), CreatedAt: &ts, UpdatedAt: &ts},
			Name:           "srv",
			Status:         servertypes.ServerStatusFailed,
			ProvisionError: strPtr("boom"),
		}
		require.NoError(t, db.Create(&s).Error)
	}

	resp, total, err := svc.Failures(ctx, "provision", 2, 2)
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
	require.Len(t, resp.Failures, 2)
}
