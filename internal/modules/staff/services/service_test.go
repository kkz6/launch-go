package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
)

type fakeLogReader struct {
	serverID string
	limit    int
	tasks    []servermodels.Task
}

func (f *fakeLogReader) FindByServer(_ context.Context, serverID string, limit int) ([]servermodels.Task, error) {
	f.serverID = serverID
	f.limit = limit
	return f.tasks, nil
}

func TestServerLogs_ReaderNotWired(t *testing.T) {
	svc := NewService(nil)

	logs, err := svc.ServerLogs(context.Background(), "srv-1", 50)

	assert.Nil(t, logs)
	assert.ErrorIs(t, err, ErrServerLogReaderUnavailable)
}

func TestServerLogs_DelegatesToReader(t *testing.T) {
	reader := &fakeLogReader{tasks: []servermodels.Task{{Name: "provision"}}}
	svc := NewService(nil)
	svc.SetServerLogReader(reader)

	logs, err := svc.ServerLogs(context.Background(), "srv-1", 25)

	require.NoError(t, err)
	assert.Equal(t, "srv-1", reader.serverID)
	assert.Equal(t, 25, reader.limit)
	require.Len(t, logs, 1)
	assert.Equal(t, "provision", logs[0].Name)
}
