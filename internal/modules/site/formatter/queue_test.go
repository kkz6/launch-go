package formatter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/site/models"
)

func TestQueueCommandPreservesConfiguredPHPBinary(t *testing.T) {
	maxTries := 3
	queue := &models.Queue{
		Command:         "php8.3 /home/launch/example.test/current/artisan queue:work",
		QueueConnection: "redis",
		QueueName:       "emails",
		MaxTries:        &maxTries,
	}

	command := QueueCommand(queue)

	require.Equal(
		t,
		"php8.3 /home/launch/example.test/current/artisan queue:work redis --queue=emails --tries=3",
		command,
	)
}

func TestQueueCommandUsesCompleteFeatureCommand(t *testing.T) {
	tests := []string{
		"php8.3 /home/launch/example.test/current/artisan horizon",
		"php8.3 /home/launch/example.test/current/artisan octane:start --server=swoole --port=8000",
		"node /home/launch/example.test/current/bootstrap/ssr/ssr.js",
	}

	for _, command := range tests {
		t.Run(command, func(t *testing.T) {
			queue := &models.Queue{
				Command:         command,
				QueueConnection: "redis",
				QueueName:       "default",
			}

			require.Equal(t, command, QueueCommand(queue))
		})
	}
}

func TestQueueCommandUpdatesWorkListenModeWithoutLosingBinary(t *testing.T) {
	queue := &models.Queue{
		Command:         "php8.4 /srv/app/artisan queue:work",
		QueueConnection: "database",
		QueueName:       "default",
		RunWithListen:   true,
	}

	require.Equal(
		t,
		"php8.4 /srv/app/artisan queue:listen database --queue=default",
		QueueCommand(queue),
	)
}
