package logger

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/pkg/observability"
)

func TestGormLoggerRedactsBoundRawAndErrorValues(t *testing.T) {
	var output bytes.Buffer
	log := zerolog.New(&output).Level(zerolog.DebugLevel)
	logger := NewGormLoggerWithConfig(&log, gormlogger.Info, time.Hour, "sqlite")
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger})
	require.NoError(t, err)
	connection, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = connection.Close() })
	request := observability.NewRequest("trace", "GET")
	ctx := observability.WithRequest(context.Background(), request)
	var value string
	require.NoError(t, db.WithContext(ctx).Raw("SELECT ?", "bound-password").Scan(&value).Error)
	require.Equal(t, "bound-password", value)
	require.NoError(t, db.WithContext(ctx).Raw("SELECT 'raw-password' /* private-comment */").Scan(&value).Error)
	logger.Trace(ctx, time.Now(), func() (string, int64) { return "SELECT 'error-password'", 0 }, errors.New("duplicate value private-email@example.com"))
	_, params := logger.ParamsFilter(ctx, "SELECT ?", "bound-password")
	require.Nil(t, params)
	text := output.String()
	for _, secret := range []string{"bound-password", "raw-password", "private-comment", "error-password", "private-email@example.com"} {
		require.NotContains(t, text, secret)
	}
	require.Contains(t, text, `"error":"query failed"`)
	require.Contains(t, text, `"request_id":`)
	require.Contains(t, text, `"trace_id":"trace"`)
	require.Contains(t, text, "SELECT ?")
}
