package tasks

import (
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"

	"github.com/kkz6/launch-go/internal/modules/platform/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// UpdateTaskFactory creates a taskrunner.Task for a given server and platform update
type UpdateTaskFactory func(server *servermodels.Server, update *models.PlatformUpdate) taskrunner.Task

var registry = map[string]UpdateTaskFactory{}

// RegisterUpdateTask registers a task factory for a given update key
func RegisterUpdateTask(key string, factory UpdateTaskFactory) {
	registry[key] = factory
}

// GetUpdateTask looks up a task factory by update key
func GetUpdateTask(key string) (UpdateTaskFactory, bool) {
	factory, ok := registry[key]
	return factory, ok
}
