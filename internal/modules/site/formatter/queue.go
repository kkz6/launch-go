package formatter

import (
	"fmt"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/site/models"
)

// QueueCommand builds the artisan queue command for a queue worker.
// Example output: "php artisan queue:work redis --queue=default --tries=3"
func QueueCommand(queue *models.Queue) string {
	var parts []string

	run := "work"
	if queue.RunWithListen {
		run = "listen"
	}

	parts = append(parts, fmt.Sprintf("php artisan queue:%s %s --queue=%s",
		run, queue.QueueConnection, queue.QueueName))

	if queue.MaxTries != nil && *queue.MaxTries > 0 {
		parts = append(parts, fmt.Sprintf("--tries=%d", *queue.MaxTries))
	}

	if queue.Environment != nil && *queue.Environment != "" {
		parts = append(parts, fmt.Sprintf("--env=%s", *queue.Environment))
	}

	if queue.RestSecondsOnEmpty != nil {
		parts = append(parts, fmt.Sprintf("--sleep=%d", *queue.RestSecondsOnEmpty))
	}

	if queue.MaxSecondsPerJob != nil {
		parts = append(parts, fmt.Sprintf("--timeout=%d", *queue.MaxSecondsPerJob))
	}

	if queue.FailedJobDelaySeconds != nil && *queue.FailedJobDelaySeconds > 0 {
		parts = append(parts, fmt.Sprintf("--backoff=%d", *queue.FailedJobDelaySeconds))
	}

	if queue.RunOnMaintenance {
		parts = append(parts, "--force")
	}

	if queue.MaxMemory != nil && *queue.MaxMemory > 0 {
		parts = append(parts, fmt.Sprintf("--memory=%d", *queue.MaxMemory))
	}

	return strings.Join(parts, " ")
}
