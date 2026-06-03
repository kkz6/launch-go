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
	tTask := now.Add(-2 * time.Minute)
	tDepTask := now.Add(-3 * time.Minute)
	tDeployment := now.Add(-4 * time.Minute)

	// Failed server (provision_error set, status not failed).
	server := servermodels.Server{
		BaseModel:      basemodels.BaseModel{ID: "srv00000000000000000000001", CreatedAt: &tProvision, UpdatedAt: &tProvision},
		Name:           "web-1",
		Status:         servertypes.ServerStatusRunning,
		ProvisionError: strPtr("apt-get failed"),
	}
	server.TeamID = "team00000000000000000000001"
	require.NoError(t, db.Create(&server).Error)

	// Failed task with an exit code and output.
	task := servermodels.Task{
		BaseModel: basemodels.BaseModel{ID: "tsk00000000000000000000001", CreatedAt: &tTask, UpdatedAt: &tTask},
		Name:      "Deploy Site",
		User:      "deploy",
		Type:      "site:deploy",
		Status:    string(servertypes.TaskStatusFailed),
		Output:    dbtype.EncryptedString("composer install failed"),
		ExitCode:  intPtr(1),
	}
	task.ServerID = server.ID
	require.NoError(t, db.Create(&task).Error)

	// Failed deployment backed by its own failed task.
	depTask := servermodels.Task{
		BaseModel: basemodels.BaseModel{ID: "tsk00000000000000000000002", CreatedAt: &tDepTask, UpdatedAt: &tDepTask},
		Name:      "Deploy",
		User:      "deploy",
		Type:      "site:deploy",
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
	// 4 rows: the failed server, the standalone failed task, the deployment's
	// own backing task (itself a failed task row), and the failed deployment.
	require.Equal(t, int64(4), total)
	require.Len(t, resp.Failures, 4)

	// Newest-first: provision (-1m), task (-2m), depTask (-3m), deployment (-4m).
	require.Equal(t, "provision", resp.Failures[0].Kind)
	require.Equal(t, "task", resp.Failures[1].Kind)
	require.Equal(t, "task", resp.Failures[2].Kind)
	require.Equal(t, "deployment", resp.Failures[3].Kind)

	// Provision mapping.
	prov := resp.Failures[0]
	require.Equal(t, server.ID, prov.ID)
	require.Equal(t, "web-1", prov.Title)
	require.Equal(t, server.ID, prov.ServerID)
	require.Equal(t, "team00000000000000000000001", prov.TeamID)
	require.Equal(t, "apt-get failed", prov.Error)

	// Task mapping: exit code surfaced, output decrypted into Detail.
	taskFailure := resp.Failures[1]
	require.Equal(t, task.ID, taskFailure.ID)
	require.Equal(t, "site:deploy", taskFailure.Title)
	require.Equal(t, server.ID, taskFailure.ServerID)
	require.Equal(t, "exit code 1", taskFailure.Error)
	require.Equal(t, "composer install failed", taskFailure.Detail)

	// Deployment mapping: joined task output decrypted, exit code surfaced.
	dep := resp.Failures[3]
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
		Name:      "Reboot",
		User:      "root",
		Type:      "server:reboot",
		Status:    string(servertypes.TaskStatusTimeout),
		// no exit code -> error falls back to status string
	}
	task.ServerID = "srv00000000000000000000020"
	require.NoError(t, db.Create(&task).Error)

	resp, _, err := svc.Failures(ctx, "task", 25, 0)
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
		Name:      "Deploy",
		User:      "deploy",
		Type:      "site:deploy",
		Status:    string(servertypes.TaskStatusFailed),
		Output:    dbtype.EncryptedString(bigOutput),
		ExitCode:  intPtr(1),
	}
	task.ServerID = "srv00000000000000000000030"
	require.NoError(t, db.Create(&task).Error)

	resp, _, err := svc.Failures(ctx, "task", 25, 0)
	require.NoError(t, err)
	require.Len(t, resp.Failures, 1)

	detail := resp.Failures[0].Detail
	require.True(t, strings.HasSuffix(detail, "…"), "detail should end with truncation marker")
	// 2048 bytes of content + 3-byte ellipsis rune.
	require.LessOrEqual(t, len(detail), detailMaxBytes+len("…"))
	require.Greater(t, len(detail), detailMaxBytes-4)
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
