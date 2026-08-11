package schedule

import (
	"testing"

	"github.com/stretchr/testify/require"

	backupjobs "github.com/kkz6/launch-go/internal/modules/backup/jobs"
)

func TestServerBackupPollerIsRegistered(t *testing.T) {
	for _, task := range GetScheduledTasks() {
		if task.Name != "server-poll-due-backups" {
			continue
		}
		require.Equal(t, "*/1 * * * *", task.CronSpec)
		require.NotNil(t, task.Task)
		require.Equal(t, backupjobs.TypePollDueBackups, task.Task.Type())
		return
	}
	t.Fatal("server backup poller is not registered")
}
