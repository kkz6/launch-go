package observability

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type trackedWidget struct {
	ID   int
	Name string
}

func TestTrackerRecordsGORMQueries(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&trackedWidget{}))

	tracker := NewTracker(time.Hour)
	tracker.Register(db)
	require.NoError(t, db.Create(&trackedWidget{Name: "test"}).Error)

	queries := tracker.Queries(10, false)
	require.Len(t, queries, 1)
	require.Contains(t, queries[0].SQL, "INSERT")
	require.False(t, queries[0].Slow)
	require.GreaterOrEqual(t, queries[0].DurationNS, int64(0))
}

func TestTrackerDetectsRepeatedQueries(t *testing.T) {
	tracker := NewTracker(time.Second)
	for range 3 {
		tracker.Record(QueryEntry{SQL: "SELECT * FROM users WHERE id = ?", DurationNS: int64(time.Millisecond)})
	}

	summary := tracker.Summary()
	require.Equal(t, 3, summary.Total)
	require.Equal(t, 1, summary.N1Count)
	require.Equal(t, 3, summary.N1Patterns[0].Count)
}
