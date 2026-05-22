package tasks

import (
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// RegisterTaskCallbacks registers all server task callback handlers.
// This should be called during application startup.
func RegisterTaskCallbacks() {
	// Register provision fresh server callback
	taskrunner.RegisterCallbackState[provisionCallbackData](ProvisionFreshServerTaskType)

	// Register provision docker server callback
	taskrunner.RegisterCallbackState[provisionDockerCallbackData](ProvisionDockerServerTaskType)
}
