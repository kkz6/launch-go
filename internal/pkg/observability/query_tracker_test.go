package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
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
		tracker.Record(QueryEntry{SQL: "SELECT * FROM users WHERE id = ?", RequestID: "request-1", DurationNS: int64(time.Millisecond)})
	}

	summary := tracker.Summary()
	require.Equal(t, 3, summary.Total)
	require.Equal(t, 1, summary.N1Count)
	require.Equal(t, 3, summary.N1Patterns[0].Count)
	require.Equal(t, float64(1), summary.N1Patterns[0].AvgMS)
	require.Equal(t, defaultCapacity, summary.Capacity)
	require.Equal(t, 3, summary.N1Threshold)
	require.Equal(t, float64(1000), summary.SlowThresholdMS)
}

func TestTrackerDoesNotMergeUnrelatedWork(t *testing.T) {
	for _, scenario := range []string{"different requests", "background", "writes", "errors", "different callers"} {
		t.Run(scenario, func(t *testing.T) {
			tracker := NewTracker(time.Second)
			for i := range 3 {
				entry := QueryEntry{SQL: "SELECT * FROM users WHERE id = ?", RequestID: "request", TraceID: "reused-external-trace"}
				switch scenario {
				case "different requests":
					entry.RequestID = fmt.Sprint(i)
				case "background":
					entry.RequestID = ""
				case "writes":
					entry.SQL = "UPDATE users SET name = ? WHERE id = ?"
				case "errors":
					entry.Error = "driver error containing secret"
				case "different callers":
					entry.Caller = fmt.Sprint(i)
				}
				tracker.Record(entry)
			}
			require.Empty(t, tracker.Summary().N1Patterns)
		})
	}
}

func TestTrackerRecordsRawScansWithoutValues(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	connection, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = connection.Close() })
	tracker := NewTracker(time.Hour)
	tracker.Register(db)
	request := NewRequest("trace", "GET")
	ctx := WithRequest(context.Background(), request)
	for range 3 {
		var value string
		require.NoError(t, db.WithContext(ctx).Raw("SELECT ?", "private-value").Scan(&value).Error)
		require.Equal(t, "private-value", value)
	}
	request.Finish("/widgets/:id")
	summary := tracker.Summary()
	require.Equal(t, 3, summary.Total)
	require.Len(t, summary.N1Patterns, 1)
	require.Equal(t, "SELECT ?", summary.N1Patterns[0].SQL)
	require.Equal(t, "/widgets/:id", summary.N1Patterns[0].Route)
	require.Equal(t, "GET", summary.N1Patterns[0].Method)
	require.NotEmpty(t, summary.N1Patterns[0].RequestID)
	encoded, err := json.Marshal(summary)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "private-value")
}

func TestTrackerBoundsStoredDataAndReportsConfiguredSlowThreshold(t *testing.T) {
	tracker := NewTracker(2 * time.Millisecond)
	for i := range defaultCapacity + 5 {
		tracker.Record(QueryEntry{SQL: "SELECT " + strings.Repeat("x", 10000), TraceID: strings.Repeat("t", 1000), Caller: strings.Repeat("c", 1000), Error: "password=secret", DurationNS: int64(time.Millisecond), Timestamp: time.Unix(int64(i), 0)})
	}
	tracker.Record(QueryEntry{SQL: "SELECT 42", DurationNS: int64(3 * time.Millisecond)})
	summary := tracker.Summary()
	require.Equal(t, defaultCapacity, summary.Total)
	require.Equal(t, 1, summary.SlowCount)
	require.Equal(t, float64(2), summary.SlowThresholdMS)
	queries := tracker.Queries(200, false)
	require.Equal(t, int64(6), queries[len(queries)-1].Timestamp.Unix())
	for _, entry := range queries {
		require.LessOrEqual(t, len(entry.SQL), maxSQLBytes)
		require.LessOrEqual(t, len(entry.TraceID), 128)
		require.LessOrEqual(t, len(entry.Caller), 512)
		require.NotContains(t, entry.Error, "secret")
	}
	require.Len(t, tracker.Queries(20, true), 1)
}

func TestTrackerConcurrentRequestsAndSnapshots(t *testing.T) {
	tracker := NewTracker(time.Second)
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			request := NewRequest("shared", "GET")
			for range 30 {
				tracker.Record(QueryEntry{SQL: "SELECT * FROM users", RequestID: request.id, request: request})
				request.Finish("/users/:id")
				_ = tracker.Summary()
				_ = tracker.Queries(5, false)
			}
		})
	}
	workers.Wait()
	require.Equal(t, defaultCapacity, tracker.Summary().Total)
}
