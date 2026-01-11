package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/taskrunner"
)

// CheckServiceStatus is the base struct for service status check tasks
type CheckServiceStatus struct {
	tasks.BaseServerTask
	serviceName   string
	grepPattern   string
	port          *int
	additionalCmd []string
}

// CheckServiceStatusConfig holds configuration for CheckServiceStatus
type CheckServiceStatusConfig struct {
	ServiceName   string
	GrepPattern   string
	Port          *int
	AdditionalCmd []string
}

// NewCheckServiceStatus creates a new CheckServiceStatus task
func NewCheckServiceStatus(server *models.Server, cfg CheckServiceStatusConfig) *CheckServiceStatus {
	grepPattern := cfg.GrepPattern
	if grepPattern == "" && cfg.ServiceName != "" {
		// Default grep pattern: first char in brackets
		grepPattern = fmt.Sprintf("[%s]%s", string(cfg.ServiceName[0]), cfg.ServiceName[1:])
	}

	task := &CheckServiceStatus{
		BaseServerTask: tasks.BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/services/check-service-status",
				TaskTimeout:  30 * time.Second,
			},
		},
		serviceName:   cfg.ServiceName,
		grepPattern:   grepPattern,
		port:          cfg.Port,
		additionalCmd: cfg.AdditionalCmd,
	}
	task.SetServer(server)

	return task
}

// Data returns the template data
func (t *CheckServiceStatus) Data() map[string]interface{} {
	return map[string]interface{}{
		"ServiceName":   t.serviceName,
		"GrepPattern":   t.grepPattern,
		"Port":          t.port,
		"AdditionalCmd": t.additionalCmd,
	}
}

// ServiceName returns the systemd service name
func (t *CheckServiceStatus) ServiceName() string {
	return t.serviceName
}

// GrepPattern returns the pattern for grep
func (t *CheckServiceStatus) GrepPattern() string {
	return t.grepPattern
}

// Port returns the optional port number to check
func (t *CheckServiceStatus) Port() *int {
	return t.port
}

// AdditionalCommands returns additional diagnostic commands
func (t *CheckServiceStatus) AdditionalCommands() []string {
	return t.additionalCmd
}

// BuildScript generates the shell script for checking service status
func (t *CheckServiceStatus) BuildScript() string {
	commands := []string{
		fmt.Sprintf("sudo systemctl status %s --no-pager --full", t.serviceName),
		`echo "===PROCESSES==="`,
		fmt.Sprintf("ps aux | grep %s | head -5", t.grepPattern),
	}

	if t.port != nil {
		commands = append(commands, `echo "===CONNECTIONS==="`)
		commands = append(commands, fmt.Sprintf("sudo ss -tulpn | grep :%d", *t.port))
	}

	commands = append(commands, `echo "===MEMORY==="`)
	commands = append(commands, fmt.Sprintf("sudo systemctl show %s --property=MemoryCurrent,MemoryPeak", t.serviceName))

	commands = append(commands, t.additionalCmd...)

	return strings.Join(commands, " && ")
}
