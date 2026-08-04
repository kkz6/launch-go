package formatter

import (
	"fmt"
	"strings"

	"github.com/kkz6/launch-go/internal/modules/site/models"
)

// QueueCommand builds the artisan queue command for a queue worker.
// Example output: "php artisan queue:work redis --queue=default --tries=3"
func QueueCommand(queue *models.Queue) string {
	command := strings.TrimSpace(queue.Command)
	if command != "" && !isLaravelQueueCommand(command) {
		// Horizon, Octane, Inertia SSR, and other feature-managed
		// Supervisor programs store their complete command on the queue
		// record. Rebuilding every record as `php artisan queue:work`
		// silently starts the wrong process.
		return command
	}

	var parts []string

	run := "work"
	if queue.RunWithListen {
		run = "listen"
	}

	if command == "" {
		command = fmt.Sprintf("php artisan queue:%s", run)
	} else {
		command = normalizeQueueVerb(command, run)
	}

	parts = append(parts, command, queue.QueueConnection, fmt.Sprintf("--queue=%s", queue.QueueName))

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

func isLaravelQueueCommand(command string) bool {
	return strings.Contains(command, "artisan queue:work") ||
		strings.Contains(command, "artisan queue:listen")
}

func normalizeQueueVerb(command, run string) string {
	command = strings.Replace(command, "artisan queue:work", "artisan queue:"+run, 1)
	return strings.Replace(command, "artisan queue:listen", "artisan queue:"+run, 1)
}
