package tasks

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner/templates"
)

// Task type constants for software installation/removal operations
const (
	InstallMySQL80TaskType      = "server:install_mysql80"
	InstallPostgreSQL16TaskType = "server:install_postgresql16"
	InstallCaddy2TaskType       = "server:install_caddy2"
	InstallRedisTaskType        = "server:install_redis"
	InstallSupervisorTaskType   = "server:install_supervisor"
	InstallComposer2TaskType    = "server:install_composer2"
	InstallNode21TaskType       = "server:install_node21"
	InstallBunTaskType          = "server:install_bun"
	InstallPHPTaskType          = "server:install_php"
	InstallLaunchAgentTaskType  = "server:install_launch_agent"
	InstallSoftwareTaskType     = "server:install_software"
	RemoveRedisTaskType         = "server:remove_redis"
	RemoveSupervisorTaskType    = "server:remove_supervisor"
	RemoveLaunchAgentTaskType   = "server:remove_launch_agent"
	RemoveMySQLTaskType         = "server:remove_mysql"
	RemovePostgreSQLTaskType    = "server:remove_postgresql"
	RemovePHPTaskType           = "server:remove_php"
	RemoveSoftwareTaskType      = "server:remove_software"
)

// MySQLInstallConfig holds configuration for MySQL installation
type MySQLInstallConfig struct {
	RootPassword   string
	DatabaseName   string
	PublicIPv4     string
	MaxConnections int
}

// InstallMySQL80 creates a task to install MySQL 8.0
func InstallMySQL80(config MySQLInstallConfig) *taskrunner.BaseTask {
	if config.MaxConnections == 0 {
		config.MaxConnections = 100
	}
	script := templates.MustRender("server", "software/install_mysql80.sh", config)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Install MySQL 8.0"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(900),
	)
}

// PostgreSQLInstallConfig holds configuration for PostgreSQL installation
type PostgreSQLInstallConfig struct {
	DatabasePassword string
	DatabaseName     string
}

// InstallPostgreSQL16 creates a task to install PostgreSQL 16
func InstallPostgreSQL16(config PostgreSQLInstallConfig) *taskrunner.BaseTask {
	script := templates.MustRender("server", "software/install_postgresql16.sh", config)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Install PostgreSQL 16"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(900),
	)
}

// CaddyInstallConfig holds configuration for Caddy installation
type CaddyInstallConfig struct {
	Username   string
	PublicIPv4 string
}

// InstallCaddy2 creates a task to install Caddy web server
func InstallCaddy2(config CaddyInstallConfig) *taskrunner.BaseTask {
	script := templates.MustRender("server", "software/install_caddy2.sh", config)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Install Caddy 2"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(600),
	)
}

// InstallRedis creates a task to install Redis
func InstallRedis() *taskrunner.BaseTask {
	script := templates.MustRender("server", "software/install_redis.sh", nil)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Install Redis"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(300),
	)
}

// InstallSupervisor creates a task to install Supervisor
func InstallSupervisor() *taskrunner.BaseTask {
	script := templates.MustRender("server", "software/install_supervisor.sh", nil)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Install Supervisor"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(300),
	)
}

// ComposerInstallConfig holds configuration for Composer installation
type ComposerInstallConfig struct {
	Username string
}

// InstallComposer2 creates a task to install Composer 2
func InstallComposer2(config ComposerInstallConfig) *taskrunner.BaseTask {
	script := templates.MustRender("server", "software/install_composer2.sh", config)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Install Composer 2"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(300),
	)
}

// InstallNode21 creates a task to install Node.js 21
func InstallNode21() *taskrunner.BaseTask {
	script := templates.MustRender("server", "software/install_node21.sh", nil)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Install Node.js 21"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(600),
	)
}

// InstallBun creates a task to install Bun
func InstallBun() *taskrunner.BaseTask {
	script := templates.MustRender("server", "software/install_bun.sh", nil)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Install Bun"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(300),
	)
}

// PHPInstallConfig holds configuration for PHP installation
type PHPInstallConfig struct {
	Version     string
	Username    string
	MaxChildren int
}

// InstallPHP creates a task to install a specific PHP version
func InstallPHP(config PHPInstallConfig) *taskrunner.BaseTask {
	if config.MaxChildren == 0 {
		config.MaxChildren = 5
	}
	script := templates.MustRender("server", "software/install_php.sh", config)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Install PHP "+config.Version),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(900),
	)
}

// LaunchAgentInstallConfig holds configuration for Launch Agent installation
type LaunchAgentInstallConfig struct {
	AgentConfigPath string
	AgentURL        string
	RootUsername    string
}

// InstallLaunchAgent creates a task to install the Launch Agent
func InstallLaunchAgent(config LaunchAgentInstallConfig) *taskrunner.BaseTask {
	if config.RootUsername == "" {
		config.RootUsername = "root"
	}
	script := templates.MustRender("server", "software/install_launch_agent.sh", config)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Install Launch Agent"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(300),
	)
}

// UpdateLaunchAgent re-runs the agent installer to swap the binary to the
// latest published release and restarts the service. Unlike
// InstallLaunchAgent it does NOT rewrite the agent config or systemd unit,
// so it needs no config vars (AgentConfigPath/AgentURL) — those already
// exist from the original install. Backs the "Update" action on the
// agent-version banner.
func UpdateLaunchAgent() *taskrunner.BaseTask {
	script := templates.MustRender("server", "software/update_launch_agent.sh", nil)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Update Launch Agent"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(300),
	)
}

// RemoveRedis creates a task to remove Redis
func RemoveRedis() *taskrunner.BaseTask {
	script := templates.MustRender("server", "software/remove_redis.sh", nil)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Remove Redis"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(300),
	)
}

// RemoveSupervisor creates a task to remove Supervisor
func RemoveSupervisor() *taskrunner.BaseTask {
	script := templates.MustRender("server", "software/remove_supervisor.sh", nil)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Remove Supervisor"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(300),
	)
}

// RemoveLaunchAgent creates a task to remove the Launch Agent
func RemoveLaunchAgent() *taskrunner.BaseTask {
	script := templates.MustRender("server", "software/remove_launch_agent.sh", nil)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Remove Launch Agent"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(120),
	)
}

// RemoveMySQL creates a task to remove MySQL
func RemoveMySQL() *taskrunner.BaseTask {
	script := templates.MustRender("server", "software/remove_mysql.sh", nil)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Remove MySQL"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(300),
	)
}

// RemovePostgreSQL creates a task to remove PostgreSQL
func RemovePostgreSQL() *taskrunner.BaseTask {
	script := templates.MustRender("server", "software/remove_postgresql.sh", nil)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Remove PostgreSQL"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(300),
	)
}

// PHPRemoveConfig holds configuration for PHP removal
type PHPRemoveConfig struct {
	Version string
}

// RemovePHP creates a task to remove a specific PHP version
func RemovePHP(config PHPRemoveConfig) *taskrunner.BaseTask {
	script := templates.MustRender("server", "software/remove_php.sh", config)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Remove PHP "+config.Version),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(300),
	)
}

// SoftwareInstallConfig holds server-specific configuration for generic software installation.
type SoftwareInstallConfig struct {
	Username         string
	MemoryInMB       int
	DatabasePassword string
	DatabaseName     string
	PublicIPv4       string
}

// InstallSoftware creates a task to install software based on the software enum.
// The config provides server-specific data needed by templates (e.g., PHP needs Username/MaxChildren,
// MySQL needs RootPassword/DatabaseName/PublicIPv4, etc.).
func InstallSoftware(software types.Software, config SoftwareInstallConfig) *taskrunner.BaseTask {
	templateName := software.InstallTemplateName()

	maxChildren := 5
	if software.IsPhp() && config.MemoryInMB > 0 {
		maxChildren = software.MaxChildren(config.MemoryInMB)
	}

	maxConnections := 100
	if software.IsDatabase() && config.MemoryInMB > 0 {
		maxConnections = software.MaxConnections(config.MemoryInMB)
	}

	data := map[string]interface{}{
		"Version":          software.GetVersion(),
		"Username":         config.Username,
		"MaxChildren":      maxChildren,
		"RootPassword":     config.DatabasePassword,
		"DatabasePassword": config.DatabasePassword,
		"DatabaseName":     config.DatabaseName,
		"PublicIPv4":       config.PublicIPv4,
		"MaxConnections":   maxConnections,
	}
	script := templates.MustRender("server", templateName, data)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Install "+software.Label()),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(900),
	)
}

// RemoveSoftware creates a task to remove software based on the software enum.
//
// Returns an error rather than panicking when no removal script exists. This
// runs inside RemoveServiceJob: a panic there unwinds past the job wrapper so
// Failed() never runs, leaving the service stuck in "uninstalling" while asynq
// recovers and retries the task until it is archived.
func RemoveSoftware(software types.Software) (*taskrunner.BaseTask, error) {
	if !software.SupportsRemove() {
		return nil, fmt.Errorf("%s cannot be uninstalled: no removal script", software.Label())
	}

	templateName := software.RemoveTemplateName()

	script, err := templates.Render("server", templateName, map[string]interface{}{
		"Version": software.GetVersion(),
	})
	if err != nil {
		return nil, fmt.Errorf("render removal script for %s: %w", software.Label(), err)
	}

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Remove "+software.Label()),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(300),
	), nil
}
