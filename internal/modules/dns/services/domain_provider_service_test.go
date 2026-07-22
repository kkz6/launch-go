package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	dnstypes "github.com/kkz6/launch-go/internal/modules/dns/types"
)

func TestCompletedSyncFieldsUsesTimestamp(t *testing.T) {
	now := time.Date(2026, time.July, 22, 10, 30, 0, 0, time.UTC)
	fields := completedSyncFields(now)

	require.Equal(t, dnstypes.SyncStatusCompleted, fields["sync_status"])
	require.Equal(t, now, fields["last_synced_at"])
	require.Nil(t, fields["sync_error_message"])
}
