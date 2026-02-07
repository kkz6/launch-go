package tasks

import (
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner/templates"
)

// ManageConfig holds the configuration for container management tasks
type ManageConfig struct {
	ContainerName string
}

// StopDockerService creates a task to stop a docker container
func StopDockerService(config ManageConfig) *taskrunner.BaseTask {
	script := templates.MustRender("docker", "docker/stop.sh", config)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Stop Docker Service"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(60),
	)
}

// StartDockerService creates a task to start a docker container
func StartDockerService(config ManageConfig) *taskrunner.BaseTask {
	script := templates.MustRender("docker", "docker/start.sh", config)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Start Docker Service"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(60),
	)
}

// RestartDockerService creates a task to restart a docker container
func RestartDockerService(config ManageConfig) *taskrunner.BaseTask {
	script := templates.MustRender("docker", "docker/restart.sh", config)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Restart Docker Service"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(60),
	)
}
