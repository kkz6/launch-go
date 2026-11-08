package middleware

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/observability"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

type traceWidget struct{ ID string }

func TestTraceAttributesBothRepositoryContextFormsAndDoesNotMergeExternalTraces(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	connection, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = connection.Close() })
	tracker := observability.NewTracker(time.Second)
	tracker.Register(db)
	// Exercise the real repository/GORM callback paths without a driver
	// retaining Fiber's pooled RequestCtx in a query cancellation goroutine.
	repo := repository.NewBase[traceWidget](db.Session(&gorm.Session{DryRun: true}))
	app := fiber.New()
	app.Use(Trace())
	app.Use(func(c *fiber.Ctx) error {
		_, queryErr := repo.FindByID(c.Context(), "private-id")
		if queryErr != nil {
			return queryErr
		}
		return c.Next()
	})
	app.Get("/widgets/:id", func(c *fiber.Ctx) error {
		for range c.QueryInt("reads", 1) {
			if _, queryErr := repo.FindByID(c.UserContext(), "private-id"); queryErr != nil {
				return queryErr
			}
		}
		return c.SendStatus(fiber.StatusOK)
	})
	for range 3 {
		request := httptest.NewRequest("GET", "/widgets/private-id?token=private-token", nil)
		request.Header.Set(TraceIDHeader, "shared-external-trace")
		response, requestErr := app.Test(request)
		require.NoError(t, requestErr)
		require.Equal(t, fiber.StatusOK, response.StatusCode)
		require.NoError(t, response.Body.Close())
	}
	summary := tracker.Summary()
	require.Equal(t, 6, summary.Total)
	require.Empty(t, summary.N1Patterns)
	requests := map[string]int{}
	for _, query := range summary.Recent {
		require.NotEmpty(t, query.RequestID)
		requests[query.RequestID]++
		require.Equal(t, "shared-external-trace", query.TraceID)
		require.Equal(t, "GET", query.Method)
		require.Equal(t, "/widgets/:id", query.Route)
		require.Contains(t, query.Caller, "TestTraceAttributes")
	}
	require.Len(t, requests, 3)
	for _, count := range requests {
		require.Equal(t, 2, count)
	}
	request := httptest.NewRequest("GET", "/widgets/private-id?reads=3", nil)
	response, err := app.Test(request)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	summary = tracker.Summary()
	require.Len(t, summary.N1Patterns, 1)
	require.Equal(t, 3, summary.N1Patterns[0].Count)
	require.Equal(t, "/widgets/:id", summary.N1Patterns[0].Route)
	encoded, err := json.Marshal(summary)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "private-id")
	require.NotContains(t, string(encoded), "private-token")
}
