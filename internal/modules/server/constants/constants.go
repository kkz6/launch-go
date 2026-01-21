package constants

import "time"

// Server statuses
const (
	StatusProvisioning = "provisioning"
	StatusRunning      = "running"
	StatusStopped      = "stopped"
	StatusFailed       = "failed"
	StatusDeleting     = "deleting"
)

// Server types
const (
	TypeApp      = "app"
	TypeDatabase = "database"
	TypeCache    = "cache"
	TypeWorker   = "worker"
	TypeCustom   = "custom"
)

// Cloud providers
const (
	ProviderDigitalOcean = "digitalocean"
	ProviderHetzner      = "hetzner"
	ProviderAWS          = "aws"
	ProviderLinode       = "linode"
	ProviderVultr        = "vultr"
	ProviderCustom       = "custom"
)

// AllProviders returns all supported cloud providers
var AllProviders = []string{
	ProviderDigitalOcean,
	ProviderHetzner,
	ProviderAWS,
	ProviderLinode,
	ProviderVultr,
	ProviderCustom,
}

// Default SSH settings
const (
	DefaultSSHPort    = 22
	DefaultSSHUser    = "root"
	DefaultSSHTimeout = 30 * time.Second
	SSHConnectTimeout = 10 * time.Second
	SSHCommandTimeout = 5 * time.Minute
)

// Provisioning settings
const (
	ProvisioningTimeout   = 15 * time.Minute
	ProvisioningRetries   = 3
	ProvisioningRetryWait = 30 * time.Second
)

// PHP versions
const (
	PHP74 = "7.4"
	PHP80 = "8.0"
	PHP81 = "8.1"
	PHP82 = "8.2"
	PHP83 = "8.3"
)

// SupportedPHPVersions returns all supported PHP versions
var SupportedPHPVersions = []string{PHP74, PHP80, PHP81, PHP82, PHP83}

// Database types
const (
	DatabaseMySQL      = "mysql"
	DatabasePostgreSQL = "postgresql"
	DatabaseMariaDB    = "mariadb"
)

// Database versions
const (
	MySQL80      = "8.0"
	PostgreSQL16 = "16"
	PostgreSQL15 = "15"
	MariaDB1011  = "10.11"
)

// Firewall rules
const (
	FirewallActionAllow = "allow"
	FirewallActionDeny  = "deny"
)

// Default ports
const (
	PortHTTP       = 80
	PortHTTPS      = 443
	PortMySQL      = 3306
	PortPostgreSQL = 5432
	PortRedis      = 6379
)

// Service types
const (
	ServiceCaddy      = "caddy"
	ServiceNginx      = "nginx"
	ServiceMySQL      = "mysql"
	ServicePostgreSQL = "postgresql"
	ServiceRedis      = "redis"
	ServiceSupervisor = "supervisor"
	ServicePHPFPM     = "php-fpm"
)

// Cron schedule presets
const (
	CronEveryMinute    = "* * * * *"
	CronEvery5Minutes  = "*/5 * * * *"
	CronEvery15Minutes = "*/15 * * * *"
	CronEvery30Minutes = "*/30 * * * *"
	CronHourly         = "0 * * * *"
	CronDaily          = "0 0 * * *"
	CronWeekly         = "0 0 * * 0"
	CronMonthly        = "0 0 1 * *"
)

// Metric types
const (
	MetricCPU     = "cpu"
	MetricMemory  = "memory"
	MetricDisk    = "disk"
	MetricNetwork = "network"
	MetricLoad    = "load"
)

// Task retention
const (
	TaskRetentionDays = 30
)
