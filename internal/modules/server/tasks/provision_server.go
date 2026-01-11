package tasks

import (
	"context"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// ProvisionServer provisions a fresh server with all required software
type ProvisionServer struct {
	BaseServerTask
	sshKeys []SSHKey
}

// SSHKey represents an SSH key to authorize
type SSHKey struct {
	PublicKey string
}

// NewProvisionServer creates a new ProvisionServer task
func NewProvisionServer(server *models.Server, sshKeys []SSHKey) *ProvisionServer {
	task := &ProvisionServer{
		BaseServerTask: BaseServerTask{
			BaseTask: taskrunner.BaseTask{
				TemplateName: "server/provision/provision-fresh-server",
				TaskTimeout:  15 * time.Minute,
			},
			server: server,
		},
		sshKeys: sshKeys,
	}

	task.FinishedCallback = task.onFinished
	task.FailedCallback = task.onFailed

	return task
}

// Data returns the template data
func (t *ProvisionServer) Data() map[string]interface{} {
	return map[string]interface{}{
		"Server":          t.server,
		"SSHKeys":         t.sshKeys,
		"SwapInMegabytes": t.SwapInMegabytes(),
		"Swappiness":      t.Swappiness(),
		"AppURL":          "", // TODO: Get from config
		"AppName":         "", // TODO: Get from config
	}
}

// SwapInMegabytes returns the calculated swap size based on server memory
func (t *ProvisionServer) SwapInMegabytes() int {
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

// Swappiness returns the calculated swappiness value based on server memory
func (t *ProvisionServer) Swappiness() int {
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

// MySQLMaxConnections returns the calculated MySQL max connections based on server memory
func (t *ProvisionServer) MySQLMaxConnections() int {
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

// MaxChildrenPhpPoolValue returns the calculated PHP-FPM max children based on server memory
func (t *ProvisionServer) MaxChildrenPhpPoolValue() int {
	memoryMB := t.GetMemoryMB()
	gigabytes := memoryMB/1024 - 1
	if gigabytes < 1 {
		gigabytes = 1
	}
	return int(float64(gigabytes) * 5 * 0.9)
}

// ProvisionSteps returns the provision steps for the server
func (t *ProvisionServer) ProvisionSteps() []string {
	return []string{
		"server/provision/apt-update-upgrade",
		"server/provision/setup-root",
		"server/provision/configure-swap",
		"server/provision/install-essential-packages",
		"server/provision/setup-default-user",
		"server/provision/setup-unattended-upgrades",
		"server/provision/ssh-security",
		"server/provision/configure-firewall",
	}
}

// SoftwareStack returns the software to install
func (t *ProvisionServer) SoftwareStack() []string {
	// TODO: Get from server services
	return []string{}
}

// onFinished handles successful provisioning
func (t *ProvisionServer) onFinished(ctx context.Context, result *taskrunner.TaskResult) {
	// Update server status to Running
	// Set provisioned_at timestamp
	// Update progress to 100%
	// Dispatch InstallTaskCleanupCron job
	// Dispatch UpdateUserPublicKey job
	// Send ServerProvisioned notification
}

// onFailed handles provisioning failure
func (t *ProvisionServer) onFailed(ctx context.Context, result *taskrunner.TaskResult) {
	// Update server status to Failed
	// Dispatch CleanupFailedServerProvisioning job
}
