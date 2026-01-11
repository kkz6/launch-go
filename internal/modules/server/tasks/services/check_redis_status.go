package services

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// CheckRedisStatus checks the status of Redis server
type CheckRedisStatus struct {
	tasks.BaseServerTask
	checkServiceStatus *CheckServiceStatus
}

// NewCheckRedisStatus creates a new CheckRedisStatus task
func NewCheckRedisStatus(server *models.Server) *CheckRedisStatus {
	port := 6379
	checkServiceStatus := NewCheckServiceStatus(server, CheckServiceStatusConfig{
		ServiceName: "redis-server",
		GrepPattern: "[r]edis",
		Port:        &port,
		AdditionalCmd: []string{
			`echo "===REDIS_INFO==="`,
			`timeout 5 redis-cli info server 2>/dev/null || echo "Redis CLI not accessible"`,
		},
	})

	task := &CheckRedisStatus{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/services/check-redis-status",
				TaskTimeout:  30 * time.Second,
			},
		},
		checkServiceStatus: checkServiceStatus,
	}
	task.SetServer(server)

	return task
}

// Data returns the template data
func (t *CheckRedisStatus) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":      t.Server(),
		"ServiceName": "redis-server",
		"GrepPattern": "[r]edis",
		"Port":        6379,
	}
}

// ServiceName returns the systemd service name
func (t *CheckRedisStatus) ServiceName() string {
	return "redis-server"
}

// GrepPattern returns the pattern for grep
func (t *CheckRedisStatus) GrepPattern() string {
	return "[r]edis"
}

// Port returns the Redis port
func (t *CheckRedisStatus) Port() int {
	return 6379
}

// BuildScript generates the shell script for checking Redis status
func (t *CheckRedisStatus) BuildScript() string {
	return t.checkServiceStatus.BuildScript()
}
