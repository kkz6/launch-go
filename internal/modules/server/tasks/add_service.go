package tasks

import (
	"context"
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// AddService installs a software service on a server
type AddService struct {
	BaseServerTask
	service      *models.InstalledService
	softwareEnum enums.Software
}

// NewAddService creates a new AddService task
func NewAddService(server *models.Server, service *models.InstalledService, softwareEnum enums.Software) *AddService {
	task := &AddService{
		BaseServerTask: BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TaskName:     "add-service",
				TemplateName: fmt.Sprintf("server/software/install-%s", softwareEnum),
				TaskTimeout:  10 * time.Minute,
			},
			server: server,
		},
		service:      service,
		softwareEnum: softwareEnum,
	}

	task.FinishedCallback = task.onFinished
	task.FailedCallback = task.onFailed

	return task
}

// Data returns the template data
func (t *AddService) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":              t.server,
		"Service":             t.service,
		"Software":            t.softwareEnum,
		"Username":            t.server.GetUsername(),
		"SwapInMegabytes":     t.SwapInMegabytes(),
		"Swappiness":          t.Swappiness(),
		"MySQLMaxConnections": t.MySQLMaxConnections(),
		"MaxChildrenPhpPool":  t.MaxChildrenPhpPool(),
		"AgentURL":            t.AgentURL(),
	}
}

// Service returns the service being installed
func (t *AddService) Service() *models.InstalledService {
	return t.service
}

// SoftwareEnum returns the software enum being installed
func (t *AddService) SoftwareEnum() enums.Software {
	return t.softwareEnum
}

// SwapInMegabytes calculates swap size based on server memory
func (t *AddService) SwapInMegabytes() int {
	memoryMB := t.GetMemoryMB()

	switch {
	case memoryMB <= 2048:
		return 1024
	case memoryMB <= 4096:
		return 2048
	case memoryMB <= 8192:
		return 3072
	default:
		return 4096
	}
}

// Swappiness calculates swappiness based on server memory
func (t *AddService) Swappiness() int {
	memoryMB := t.GetMemoryMB()

	switch {
	case memoryMB <= 1024:
		return 20
	case memoryMB <= 2048:
		return 35
	case memoryMB <= 4096:
		return 50
	default:
		return 60
	}
}

// MySQLMaxConnections calculates max connections based on server memory
func (t *AddService) MySQLMaxConnections() int {
	memoryMB := t.GetMemoryMB()

	switch {
	case memoryMB <= 1024:
		return 100
	case memoryMB <= 2048:
		return 200
	case memoryMB <= 4096:
		return 400
	default:
		return 500
	}
}

// MaxChildrenPhpPool calculates PHP-FPM max children based on server memory
func (t *AddService) MaxChildrenPhpPool() int {
	memoryMB := t.GetMemoryMB()
	gigabytes := max(1, memoryMB/1024-1)

	return int(float64(gigabytes) * 5 * 0.9)
}

// AgentURL returns the agent URL from service type data
func (t *AddService) AgentURL() string {
	if t.service == nil {
		return ""
	}

	typeData := t.service.GetTypeData()
	if typeData == nil {
		return ""
	}

	if url, ok := typeData["url"].(string); ok {
		return url
	}

	return ""
}

// onFinished handles successful completion
func (t *AddService) onFinished(ctx context.Context, result *taskrunner.TaskResult) {
	// Update service status to installed
	// If MySQL, create default database and user
}

// onFailed handles task failure
func (t *AddService) onFailed(ctx context.Context, result *taskrunner.TaskResult) {
	// Update service status to failed
}

func max(a, b int) int {
	if a > b {
		return a
	}

	return b
}
