package tasks

import (
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner/templates"
)

// UpdateTraefikConfig holds configuration for updating Traefik dynamic config
type UpdateTraefikConfig struct {
	ContainerName string
	YAMLContent   string
}

// UpdateTraefik creates a task to update the Traefik dynamic config for a service
func UpdateTraefik(config UpdateTraefikConfig) *taskrunner.BaseTask {
	script := templates.MustRender("docker", "docker/update_traefik.sh", config)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Update Traefik Config"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// RemoveTraefikConfig creates a task to remove the Traefik dynamic config for a service
func RemoveTraefikConfig(config ManageConfig) *taskrunner.BaseTask {
	script := templates.MustRender("docker", "docker/remove_traefik.sh", config)

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Remove Traefik Config"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}
