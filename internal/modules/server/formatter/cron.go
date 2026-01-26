package formatter

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/server/models"
)

// CronFileContents generates the cron file contents for a cron job.
// Format: expression user command >> logfile 2>&1
func CronFileContents(cron *models.Cron) string {
	return fmt.Sprintf("%s %s %s >> %s 2>&1\n",
		cron.Expression,
		cron.User,
		cron.GetCommand(),
		cron.GetLogPath(),
	)
}
