package tasks

import (
	"github.com/kkz6/launch-go/internal/modules/server/tasks/templates"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
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
	script := templates.MustRender("software/install_mysql80.sh", config)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Install MySQL 8.0"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(900),
	)
}

// PostgreSQLInstallConfig holds configuration for PostgreSQL installation
type PostgreSQLInstallConfig struct {
	DatabasePassword string
	DatabaseName     string
}

// InstallPostgreSQL16 creates a task to install PostgreSQL 16
func InstallPostgreSQL16(config PostgreSQLInstallConfig) *taskrunner.BaseTask {
	script := templates.MustRender("software/install_postgresql16.sh", config)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Install PostgreSQL 16"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(900),
	)
}

// CaddyInstallConfig holds configuration for Caddy installation
type CaddyInstallConfig struct {
	Username   string
	PublicIPv4 string
}

// InstallCaddy2 creates a task to install Caddy web server
func InstallCaddy2(config CaddyInstallConfig) *taskrunner.BaseTask {
	script := templates.MustRender("software/install_caddy2.sh", config)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Install Caddy 2"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(600),
	)
}

// InstallRedis creates a task to install Redis
func InstallRedis() *taskrunner.BaseTask {
	script := templates.MustRender("software/install_redis.sh", nil)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Install Redis"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(300),
	)
}

// InstallSupervisor creates a task to install Supervisor
func InstallSupervisor() *taskrunner.BaseTask {
	script := templates.MustRender("software/install_supervisor.sh", nil)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Install Supervisor"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(300),
	)
}

// ComposerInstallConfig holds configuration for Composer installation
type ComposerInstallConfig struct {
	Username string
}

// InstallComposer2 creates a task to install Composer 2
func InstallComposer2(config ComposerInstallConfig) *taskrunner.BaseTask {
	script := templates.MustRender("software/install_composer2.sh", config)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Install Composer 2"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(300),
	)
}

// InstallNode21 creates a task to install Node.js 21
func InstallNode21() *taskrunner.BaseTask {
	script := templates.MustRender("software/install_node21.sh", nil)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Install Node.js 21"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(600),
	)
}

// InstallBun creates a task to install Bun
func InstallBun() *taskrunner.BaseTask {
	script := templates.MustRender("software/install_bun.sh", nil)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Install Bun"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(300),
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
	script := templates.MustRender("software/install_php.sh", config)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Install PHP "+config.Version),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(900),
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
	script := templates.MustRender("software/install_launch_agent.sh", config)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Install Launch Agent"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(300),
	)
}

// RemoveRedis creates a task to remove Redis
func RemoveRedis() *taskrunner.BaseTask {
	script := templates.MustRender("software/remove_redis.sh", nil)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Remove Redis"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(300),
	)
}

// RemoveSupervisor creates a task to remove Supervisor
func RemoveSupervisor() *taskrunner.BaseTask {
	script := templates.MustRender("software/remove_supervisor.sh", nil)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Remove Supervisor"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(300),
	)
}

// RemoveLaunchAgent creates a task to remove the Launch Agent
func RemoveLaunchAgent() *taskrunner.BaseTask {
	script := templates.MustRender("software/remove_launch_agent.sh", nil)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Remove Launch Agent"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(120),
	)
}

// RemoveMySQL creates a task to remove MySQL
func RemoveMySQL() *taskrunner.BaseTask {
	script := templates.MustRender("software/remove_mysql.sh", nil)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Remove MySQL"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(300),
	)
}

// RemovePostgreSQL creates a task to remove PostgreSQL
func RemovePostgreSQL() *taskrunner.BaseTask {
	script := templates.MustRender("software/remove_postgresql.sh", nil)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Remove PostgreSQL"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(300),
	)
}

// PHPRemoveConfig holds configuration for PHP removal
type PHPRemoveConfig struct {
	Version string
}

// RemovePHP creates a task to remove a specific PHP version
func RemovePHP(config PHPRemoveConfig) *taskrunner.BaseTask {
	script := templates.MustRender("software/remove_php.sh", config)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Remove PHP "+config.Version),
		taskrunner.WithScript(script),
		taskrunner.WithTimeout(300),
	)
}
