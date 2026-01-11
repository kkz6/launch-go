package tasks

import (
	"context"
	"time"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/taskrunner"
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

// SwapInMegabytes returns the calculated swap size
func (t *ProvisionServer) SwapInMegabytes() int {
	return taskrunner.SwapInMegabytes(t.GetMemoryMB())
}

// Swappiness returns the calculated swappiness value
func (t *ProvisionServer) Swappiness() int {
	return taskrunner.Swappiness(t.GetMemoryMB())
}

// MySQLMaxConnections returns the calculated MySQL max connections
func (t *ProvisionServer) MySQLMaxConnections() int {
	return taskrunner.MySQLMaxConnections(t.GetMemoryMB())
}

// MaxChildrenPhpPoolValue returns the calculated PHP-FPM max children
func (t *ProvisionServer) MaxChildrenPhpPoolValue() int {
	return taskrunner.MaxChildrenPhpPool(t.GetMemoryMB())
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
