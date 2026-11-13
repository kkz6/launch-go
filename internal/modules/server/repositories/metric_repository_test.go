package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
)

func TestMetricsUseStoredCreationTime(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&models.Metric{}))
	ctx := context.Background()
	repo := NewMetricRepository(db)
	latest := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	older := latest.Add(-time.Hour)
	require.NoError(t, db.Create(&[]models.Metric{
		{ID: 1, ServerID: "one", CreatedAt: &older},
		{ID: 2, ServerID: "one", CreatedAt: &latest},
		{ID: 3, ServerID: "two", CreatedAt: &older},
		{ID: 4, ServerID: "one", CreatedAt: &latest},
	}).Error)
	current, err := repo.FindLatestByServer(ctx, "one")
	require.NoError(t, err)
	require.NotNil(t, current)
	require.EqualValues(t, 4, current.ID)
	from := older.Add(time.Minute)
	metrics, err := repo.FindByServer(ctx, "one", &from, nil, 1)
	require.NoError(t, err)
	require.Len(t, metrics, 1)
	require.EqualValues(t, 4, metrics[0].ID)
	require.NoError(t, repo.DeleteOld(ctx, "one", latest))
	metrics, err = repo.FindByServer(ctx, "one", nil, nil, 0)
	require.NoError(t, err)
	require.Len(t, metrics, 2)
	metrics, err = repo.FindByServer(ctx, "two", nil, nil, 0)
	require.NoError(t, err)
	require.Len(t, metrics, 1)
}
