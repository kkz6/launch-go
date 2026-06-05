package tables

import (
	"context"
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
	"github.com/kkz6/launch-go/internal/modules/staff/repositories"
	"github.com/kkz6/launch-go/internal/modules/staff/services"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/pkg/table"
)

func intPtr(v int) *int       { return &v }
func strPtr(v string) *string { return &v }

// setupFailuresDB mirrors the services package harness: an in-memory sqlite DB
// with the servers, tasks and deployments tables and a 32-byte encryption key
// so EncryptedString round-trips.
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

func buildFailuresTable(db *gorm.DB) *FailuresTable {
	return NewFailuresTable(services.NewService(repositories.NewRegistry(db)))
}

func TestFailuresTableMeta(t *testing.T) {
	tbl := NewFailuresTable(nil)
	meta := table.Render(tbl)

	cols := map[string]bool{}
	for _, c := range meta.Columns {
		cols[c.Key] = true
	}
	require.True(t, cols["kind"], "kind column present")
	require.True(t, cols["title"], "title column present")
	require.True(t, cols["when"], "when column present")
	require.True(t, cols["error"], "error column present")

	require.Len(t, meta.Filters, 1)
	require.Equal(t, "kind", meta.Filters[0].Key)
	require.Equal(t, "set", meta.Filters[0].Type)
	require.Len(t, meta.Filters[0].Options, 3)

	// Read-only: no row or bulk actions.
	require.Empty(t, meta.Actions.Row)
	require.Empty(t, meta.Actions.Bulk)
}

func TestFailuresResolve(t *testing.T) {
	db := setupFailuresDB(t)
	tbl := buildFailuresTable(db)
	ctx := context.Background()

	now := time.Now()
	tProvision := now.Add(-1 * time.Minute)
	tInstall := now.Add(-2 * time.Minute)

	server := servermodels.Server{
		BaseModel:      basemodels.BaseModel{ID: "srv00000000000000000000001", CreatedAt: &tProvision, UpdatedAt: &tProvision},
		Name:           "web-1",
		Status:         servertypes.ServerStatusRunning,
		ProvisionError: strPtr("apt-get failed"),
	}
	server.TeamID = "team00000000000000000000001"
	require.NoError(t, db.Create(&server).Error)

	task := servermodels.Task{
		BaseModel: basemodels.BaseModel{ID: "tsk00000000000000000000001", CreatedAt: &tInstall, UpdatedAt: &tInstall},
		Name:      "Install PHP 8.3",
		User:      "root",
		Type:      "github.com/kkz6/launch-go/internal/pkg/taskrunner.BaseTask",
		Status:    string(servertypes.TaskStatusFailed),
		Output:    dbtype.EncryptedString("apt install php8.3 failed"),
		ExitCode:  intPtr(1),
	}
	task.ServerID = server.ID
	require.NoError(t, db.Create(&task).Error)

	resp, err := tbl.Resolve(ctx, table.Request{PerPage: 25})
	require.NoError(t, err)

	require.Equal(t, int64(2), resp.Pagination.Total)
	require.Equal(t, 1, resp.Pagination.CurrentPage)
	require.Equal(t, 25, resp.Pagination.PerPage)
	require.Equal(t, 1, resp.Pagination.LastPage)
	require.Equal(t, int64(1), resp.Pagination.From)
	require.Equal(t, int64(2), resp.Pagination.To)
	require.Len(t, resp.Data, 2)

	// Newest-first: provision (-1m) then service install (-2m). The kind cell is
	// mapped to a labelled badge {value,variant} by table.MapRow.
	prov := resp.Data[0]
	require.Equal(t, "Provision", badgeValue(prov["kind"]))
	require.Equal(t, "web-1", prov["title"])
	require.Equal(t, "apt-get failed", prov["error"])

	taskRow := resp.Data[1]
	require.Equal(t, "Service installation", badgeValue(taskRow["kind"]))
	require.Equal(t, "Install PHP 8.3", taskRow["title"])
	require.Equal(t, "exit code 1", taskRow["error"])
	require.Equal(t, "apt install php8.3 failed", taskRow["detail"])

	// Meta schema travels with the data response.
	require.NotEmpty(t, resp.Meta.Columns)
}

// badgeValue extracts the display text from a mapped badge cell
// ({value,variant}); returns "" when the cell is not a badge map.
func badgeValue(cell any) string {
	m, ok := cell.(map[string]any)
	if !ok {
		return ""
	}
	v, _ := m["value"].(string)
	return v
}

func TestFailuresResolveKindFilter(t *testing.T) {
	db := setupFailuresDB(t)
	tbl := buildFailuresTable(db)
	ctx := context.Background()

	now := time.Now()

	server := servermodels.Server{
		BaseModel:      basemodels.BaseModel{ID: "srv00000000000000000000010", CreatedAt: &now, UpdatedAt: &now},
		Name:           "db-1",
		Status:         servertypes.ServerStatusRunning,
		ProvisionError: strPtr("boom"),
	}
	server.TeamID = "team00000000000000000000010"
	require.NoError(t, db.Create(&server).Error)

	task := servermodels.Task{
		BaseModel: basemodels.BaseModel{ID: "tsk00000000000000000000010", CreatedAt: &now, UpdatedAt: &now},
		Name:      "Install PHP 8.3",
		User:      "root",
		Type:      "github.com/kkz6/launch-go/internal/pkg/taskrunner.BaseTask",
		Status:    string(servertypes.TaskStatusFailed),
		ExitCode:  intPtr(1),
	}
	task.ServerID = server.ID
	require.NoError(t, db.Create(&task).Error)

	req := table.Request{
		PerPage: 25,
		Filters: map[string]map[table.Clause]any{
			"kind": {table.ClauseEquals: services.KindServiceInstallation},
		},
	}
	resp, err := tbl.Resolve(ctx, req)
	require.NoError(t, err)

	require.Equal(t, int64(1), resp.Pagination.Total)
	require.Len(t, resp.Data, 1)
	require.Equal(t, "Service installation", badgeValue(resp.Data[0]["kind"]))
	require.Equal(t, "Install PHP 8.3", resp.Data[0]["title"])
}
