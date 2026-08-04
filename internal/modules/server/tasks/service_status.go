package tasks

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// Task type constants for service status operations
const (
	CheckPhpStatusTaskType        = "server:check_php_status"
	CheckMySQLStatusTaskType      = "server:check_mysql_status"
	CheckPostgreSQLStatusTaskType = "server:check_postgresql_status"
	CheckRedisStatusTaskType      = "server:check_redis_status"
	CheckCaddyStatusTaskType      = "server:check_caddy_status"
	CheckSupervisorStatusTaskType = "server:check_supervisor_status"
	CheckBunStatusTaskType        = "server:check_bun_status"
	CheckNodeStatusTaskType       = "server:check_node_status"
	GetServiceStatusTaskType      = "server:get_service_status"
)

// CheckPhpStatus creates a task to check PHP-FPM status with detailed diagnostics.
func CheckPhpStatus(version string) *taskrunner.BaseTask {
	script := fmt.Sprintf(`sudo systemctl status php%s-fpm --no-pager --full
echo "===PROCESSES==="
ps aux | grep [p]hp%s-fpm | head -10
echo "===CONNECTIONS==="
sudo ss -tulpn | grep php-fpm || echo "No connections"
echo "===MEMORY==="
sudo systemctl show php%s-fpm --property=MemoryCurrent,MemoryPeak
echo "===FPM_STATUS==="
sudo php%s -v
echo "===POOL_CONFIG==="
sudo ls -la /etc/php/%s/fpm/pool.d/ 2>/dev/null || echo "Pool config not accessible"`, version, version, version, version, version)

	return taskrunner.NewBaseTask(
		taskrunner.WithName(fmt.Sprintf("Check PHP %s Status", version)),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// CheckMySQLStatus creates a task to check MySQL status with detailed diagnostics.
func CheckMySQLStatus() *taskrunner.BaseTask {
	script := `sudo systemctl status mysql --no-pager --full
echo "===PROCESSES==="
ps aux | grep [m]ysql | head -5
echo "===CONNECTIONS==="
sudo ss -tulpn | grep :3306 || echo "No connections"
echo "===MEMORY==="
sudo systemctl show mysql --property=MemoryCurrent,MemoryPeak`

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Check MySQL Status"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// CheckPostgreSQLStatus creates a task to check PostgreSQL status with detailed diagnostics.
func CheckPostgreSQLStatus() *taskrunner.BaseTask {
	script := `sudo systemctl status postgresql --no-pager --full
echo "===PROCESSES==="
ps aux | grep [p]ostgres | head -5
echo "===CONNECTIONS==="
sudo ss -tulpn | grep :5432 || echo "No connections"
echo "===MEMORY==="
sudo systemctl show postgresql --property=MemoryCurrent,MemoryPeak`

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Check PostgreSQL Status"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// CheckRedisStatus creates a task to check Redis status with detailed diagnostics.
func CheckRedisStatus() *taskrunner.BaseTask {
	script := `sudo systemctl status redis-server --no-pager --full
echo "===PROCESSES==="
ps aux | grep [r]edis | head -5
echo "===CONNECTIONS==="
sudo ss -tulpn | grep :6379 || echo "No connections"
echo "===MEMORY==="
sudo systemctl show redis-server --property=MemoryCurrent,MemoryPeak
echo "===REDIS_INFO==="
timeout 5 redis-cli info server 2>/dev/null || echo "Redis CLI not accessible"`

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Check Redis Status"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// CheckCaddyStatus creates a task to check Caddy status with detailed diagnostics.
func CheckCaddyStatus() *taskrunner.BaseTask {
	script := `sudo systemctl status caddy --no-pager --full
echo "===PROCESSES==="
ps aux | grep [c]addy | head -5
echo "===CONNECTIONS==="
sudo ss -tulpn | grep caddy || echo "No connections"
echo "===MEMORY==="
sudo systemctl show caddy --property=MemoryCurrent,MemoryPeak
echo "===CONFIG_TEST==="
timeout 5 sudo caddy validate --config /etc/caddy/Caddyfile 2>/dev/null || echo "Config validation not available"`

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Check Caddy Status"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// CheckSupervisorStatus creates a task to check Supervisor status with detailed diagnostics.
func CheckSupervisorStatus() *taskrunner.BaseTask {
	script := `sudo systemctl status supervisor --no-pager --full
echo "===PROCESSES==="
ps aux | grep [s]upervisor | head -5
echo "===MEMORY==="
sudo systemctl show supervisor --property=MemoryCurrent,MemoryPeak
echo "===SUPERVISOR_STATUS==="
timeout 5 sudo supervisorctl status 2>/dev/null || echo "Supervisorctl not accessible"
echo "===PROGRAMS==="
sudo ls -la /etc/supervisor/conf.d/ 2>/dev/null || echo "Config directory not accessible"`

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Check Supervisor Status"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// CheckBunStatus creates a task to check Bun status.
func CheckBunStatus() *taskrunner.BaseTask {
	script := `echo "===BUN_VERSION==="
bun --version 2>/dev/null || echo "Bun not installed"
echo "===BUN_PATH==="
which bun 2>/dev/null || echo "Bun not in PATH"`

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Check Bun Status"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(15),
	)
}

// CheckNodeStatus creates a task to check Node.js status.
func CheckNodeStatus() *taskrunner.BaseTask {
	script := `echo "===NODE_VERSION==="
node --version 2>/dev/null || echo "Node not installed"
echo "===NPM_VERSION==="
npm --version 2>/dev/null || echo "NPM not installed"
echo "===NODE_PATH==="
which node 2>/dev/null || echo "Node not in PATH"`

	return taskrunner.NewBaseTask(
		taskrunner.WithName("Check Node Status"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(15),
	)
}

// GetServiceStatusTask returns the appropriate status check task based on software type.
func GetServiceStatusTask(software string, version string) *taskrunner.BaseTask {
	parsedSoftware := types.Software(software)
	if parsedSoftware.IsPhp() {
		return CheckPhpStatus(parsedSoftware.GetVersion())
	}

	switch software {
	case "mysql80", "mysql":
		return CheckMySQLStatus()
	case "postgresql16", "postgresql":
		return CheckPostgreSQLStatus()
	case "redis":
		return CheckRedisStatus()
	case "caddy2", "caddy":
		return CheckCaddyStatus()
	case "supervisor":
		return CheckSupervisorStatus()
	case "bun":
		return CheckBunStatus()
	case "node", "node18", "node20", "node22":
		return CheckNodeStatus()
	default:
		return CheckServiceStatus(software)
	}
}
