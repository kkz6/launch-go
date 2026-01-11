package tasks

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// LaunchConfigGenerator is an interface for generating launch agent configuration
type LaunchConfigGenerator interface {
	GenerateConfig(server *models.Server) (string, error)
}

// UpdateLaunchConfig updates the Launch Agent configuration on a server
type UpdateLaunchConfig struct {
	BaseServerTask
	configPath      string
	configGenerator LaunchConfigGenerator
}

// NewUpdateLaunchConfig creates a new UpdateLaunchConfig task
func NewUpdateLaunchConfig(server *models.Server, configPath string, generator LaunchConfigGenerator) *UpdateLaunchConfig {
	task := &UpdateLaunchConfig{
		BaseServerTask: BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TaskName:     "update-launch-config",
				TemplateName: "server/update-launch-config",
				TaskTimeout:  2 * time.Minute,
			},
			server: server,
		},
		configPath:      configPath,
		configGenerator: generator,
	}

	return task
}

// Data returns the template data
func (t *UpdateLaunchConfig) Data() map[string]interface{} {
	config := ""
	if t.configGenerator != nil {
		config, _ = t.configGenerator.GenerateConfig(t.server)
	}

	return map[string]interface{}{
		"Server":     t.server,
		"ConfigPath": t.configPath,
		"Config":     config,
	}
}

// ConfigPath returns the path where the config will be written
func (t *UpdateLaunchConfig) ConfigPath() string {
	return t.configPath
}

// Config generates and returns the configuration content
func (t *UpdateLaunchConfig) Config() (string, error) {
	if t.configGenerator == nil {
		return "", nil
	}

	return t.configGenerator.GenerateConfig(t.server)
}
