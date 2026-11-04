package jobs

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/database/serializers"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
)

func TestResolveDeploymentReleaseUsesPersistedName(t *testing.T) {
	name := "20261104124500-01JCF5T5B1ANB6JNTMDK8M7PGV"
	deployment := &models.Deployment{ReleaseName: &name}

	got, err := resolveDeploymentRelease(context.Background(), nil, deployment, "server-1")

	require.NoError(t, err)
	require.Equal(t, name, got)
}

func TestResolveDeploymentReleaseRejectsUnsafePersistedName(t *testing.T) {
	for _, name := range []string{"", "../previous", "/tmp/release", "release/name", "name;echo", "name\nother", "-option", strings.Repeat("a", 65)} {
		t.Run(name, func(t *testing.T) {
			deployment := &models.Deployment{ReleaseName: &name}
			got, err := resolveDeploymentRelease(context.Background(), nil, deployment, "server-1")
			require.Empty(t, got)
			require.ErrorContains(t, err, "invalid release name")
		})
	}
}

func TestResolveDeploymentReleaseRecoversEncryptedLegacyTask(t *testing.T) {
	db, deployment := releaseResolverFixture(t)
	const releaseName = "20261104124500"
	task := createLegacyReleaseTask(t, db, deployment, "server-1", nil, "#!/bin/bash\nDEPLOYMENT_KEEP=\""+releaseName+"\"\n")

	var storedScript string
	require.NoError(t, db.Raw("SELECT script FROM tasks WHERE id = ?", task.ID).Scan(&storedScript).Error)
	require.NotContains(t, storedScript, releaseName)
	require.NotEqual(t, releaseName, deployment.CreatedAt.Format("20060102150405"))

	got, err := resolveDeploymentRelease(context.Background(), db, deployment, "server-1")

	require.NoError(t, err)
	require.Equal(t, releaseName, got)
	require.NotNil(t, deployment.ReleaseName)
	require.Equal(t, releaseName, *deployment.ReleaseName)
	var saved models.Deployment
	require.NoError(t, db.First(&saved, "id = ?", deployment.ID).Error)
	require.Equal(t, deployment.ReleaseName, saved.ReleaseName)
	// The recovered identity remains usable after legacy task history is gone.
	require.NoError(t, db.Delete(&task).Error)
	got, err = resolveDeploymentRelease(context.Background(), db, &saved, "server-1")
	require.NoError(t, err)
	require.Equal(t, releaseName, got)
}

func TestResolveDeploymentReleaseRejectsMissingLegacyMetadata(t *testing.T) {
	for _, taskID := range []string{"", "missing-task"} {
		t.Run(taskID, func(t *testing.T) {
			db, deployment := releaseResolverFixture(t)
			if taskID != "" {
				deployment.TaskID = &taskID
			}
			got, err := resolveDeploymentRelease(context.Background(), db, deployment, "server-1")
			require.Empty(t, got)
			require.ErrorContains(t, err, "release information unavailable")
			require.Nil(t, deployment.ReleaseName)
		})
	}
}

func TestResolveDeploymentReleaseChecksLegacyTaskScope(t *testing.T) {
	for _, test := range []struct {
		name     string
		serverID string
		siteID   string
		wantErr  bool
	}{
		{name: "matching site", serverID: "server-1", siteID: "site-1"},
		{name: "different server", serverID: "server-2", siteID: "site-1", wantErr: true},
		{name: "different site", serverID: "server-1", siteID: "site-2", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, deployment := releaseResolverFixture(t)
			createLegacyReleaseTask(t, db, deployment, test.serverID, &test.siteID, "DEPLOYMENT_KEEP=\"20261104124500\"\n")
			got, err := resolveDeploymentRelease(context.Background(), db, deployment, "server-1")
			if test.wantErr {
				require.ErrorContains(t, err, "release information unavailable")
				require.Empty(t, got)
				require.Nil(t, deployment.ReleaseName)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "20261104124500", got)
		})
	}
}

func TestResolveDeploymentReleaseRejectsUnreliableLegacyScripts(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
	}{
		{name: "missing marker", script: "echo 20261104124500\n"},
		{name: "path instead of name", script: "DEPLOYMENT_KEEP=\"../20261104124500\"\n"},
		{name: "not generated line", script: "echo 'DEPLOYMENT_KEEP=\"20261104124500\"'\n"},
		{name: "multiple markers", script: "DEPLOYMENT_KEEP=\"20261104124500\"\nDEPLOYMENT_KEEP=\"20261104124600\"\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, deployment := releaseResolverFixture(t)
			createLegacyReleaseTask(t, db, deployment, "server-1", nil, test.script)
			got, err := resolveDeploymentRelease(context.Background(), db, deployment, "server-1")
			require.ErrorContains(t, err, "no unambiguous release name")
			require.Empty(t, got)
			require.Nil(t, deployment.ReleaseName)
		})
	}
}

func TestResolveDeploymentReleaseRequiresSuccessfulPersistence(t *testing.T) {
	db, deployment := releaseResolverFixture(t)
	createLegacyReleaseTask(t, db, deployment, "server-1", nil, "DEPLOYMENT_KEEP=\"20261104124500\"\n")
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("test:release-save-failure", func(tx *gorm.DB) {
		tx.AddError(errors.New("write unavailable"))
	}))

	got, err := resolveDeploymentRelease(context.Background(), db, deployment, "server-1")

	require.Empty(t, got)
	require.ErrorContains(t, err, "persist recovered deployment release: write unavailable")
	require.Nil(t, deployment.ReleaseName)
	var saved models.Deployment
	require.NoError(t, db.First(&saved, "id = ?", deployment.ID).Error)
	require.Nil(t, saved.ReleaseName)
}

func releaseResolverFixture(t *testing.T) (*gorm.DB, *models.Deployment) {
	t.Helper()
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&models.Deployment{}, &servermodels.Task{}))
	createdAt := time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)
	deployment := &models.Deployment{Status: sitetypes.DeploymentStatusFinished}
	deployment.ID = "deployment-1"
	deployment.SiteID = "site-1"
	deployment.TeamID = "team-1"
	deployment.CreatedAt = &createdAt
	require.NoError(t, db.Create(deployment).Error)
	return db, deployment
}

func createLegacyReleaseTask(t *testing.T, db *gorm.DB, deployment *models.Deployment, serverID string, siteID *string, script string) *servermodels.Task {
	t.Helper()
	task := &servermodels.Task{
		Name:   "Deploy Site (Zero Downtime)",
		User:   "launcher",
		Type:   "site:deploy",
		Script: dbtype.EncryptedString(script),
		SiteID: siteID,
	}
	task.ServerID = serverID
	require.NoError(t, db.Create(task).Error)
	deployment.TaskID = &task.ID
	require.NoError(t, db.Model(deployment).Update("task_id", task.ID).Error)
	return task
}
