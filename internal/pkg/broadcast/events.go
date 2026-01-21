package broadcast

// Event type constants for WebSocket broadcasts.
// These provide a centralized, type-safe way to reference event names
// across the codebase, reducing typos and improving discoverability.
//
// Naming convention: {Resource}{Action}
// Event format on wire: "{resource}.{action}" (e.g., "server.updated")
//
// Usage:
//
//	broadcaster.BroadcastToTeam(teamID, broadcast.ServerUpdated, data)

// Server events
const (
	// ServerCreated is broadcast when a new server is created
	ServerCreated = "server.created"

	// ServerUpdated is broadcast when a server is updated
	ServerUpdated = "server.updated"

	// ServerDeleted is broadcast when a server is deleted
	ServerDeleted = "server.deleted"

	// ServerConnected is broadcast when a server establishes connection
	ServerConnected = "server.connected"

	// ServerDisconnected is broadcast when a server loses connection
	ServerDisconnected = "server.disconnected"

	// ServerProvisioned is broadcast when server provisioning completes successfully
	ServerProvisioned = "server.provisioned"

	// ServerProvisionFailed is broadcast when server provisioning fails
	ServerProvisionFailed = "server.provision_failed"

	// ServerProvisionTimeout is broadcast when server provisioning times out
	ServerProvisionTimeout = "server.provision_timeout"

	// ServerMetrics is broadcast when server metrics are updated
	ServerMetrics = "server.metrics"

	// ServerStatus is broadcast when server status changes
	ServerStatus = "server.status"

	// ServerRebooting is broadcast when a server starts rebooting
	ServerRebooting = "server.rebooting"

	// ServerRebooted is broadcast when a server finishes rebooting
	ServerRebooted = "server.rebooted"

	// ServerArchived is broadcast when a server is archived
	ServerArchived = "server.archived"

	// ServerUnarchived is broadcast when a server is unarchived
	ServerUnarchived = "server.unarchived"
)

// Site events
const (
	// SiteCreated is broadcast when a new site is created
	SiteCreated = "site.created"

	// SiteUpdated is broadcast when a site is updated
	SiteUpdated = "site.updated"

	// SiteDeleted is broadcast when a site is deleted
	SiteDeleted = "site.deleted"

	// SiteInstalled is broadcast when a site installation completes
	SiteInstalled = "site.installed"

	// SiteUninstalled is broadcast when a site is uninstalled
	SiteUninstalled = "site.uninstalled"
)

// Deployment events
const (
	// DeploymentCreated is broadcast when a new deployment is created
	DeploymentCreated = "deployment.created"

	// DeploymentStarted is broadcast when a deployment begins
	DeploymentStarted = "deployment.started"

	// DeploymentProgress is broadcast during deployment with progress updates
	DeploymentProgress = "deployment.progress"

	// DeploymentFinished is broadcast when a deployment completes successfully
	DeploymentFinished = "deployment.finished"

	// DeploymentFailed is broadcast when a deployment fails
	DeploymentFailed = "deployment.failed"

	// DeploymentTimeout is broadcast when a deployment times out
	DeploymentTimeout = "deployment.timeout"

	// DeploymentCancelled is broadcast when a deployment is cancelled
	DeploymentCancelled = "deployment.cancelled"

	// DeploymentLog is broadcast for real-time deployment log output
	DeploymentLog = "deployment.log"
)

// Task events (for generic SSH tasks)
const (
	// TaskCreated is broadcast when a new task is created
	TaskCreated = "task.created"

	// TaskStarted is broadcast when a task begins execution
	TaskStarted = "task.started"

	// TaskRunning is broadcast when a task is actively running (with output)
	TaskRunning = "task.running"

	// TaskProgress is broadcast during task execution with progress updates
	TaskProgress = "task.progress"

	// TaskFinished is broadcast when a task completes successfully
	TaskFinished = "task.finished"

	// TaskFailed is broadcast when a task fails
	TaskFailed = "task.failed"

	// TaskTimeout is broadcast when a task times out
	TaskTimeout = "task.timeout"

	// TaskOutput is broadcast for real-time task output streaming
	TaskOutput = "task.output"
)

// Database events
const (
	// DatabaseCreated is broadcast when a new database is created
	DatabaseCreated = "database.created"

	// DatabaseUpdated is broadcast when a database is updated
	DatabaseUpdated = "database.updated"

	// DatabaseDeleted is broadcast when a database is deleted
	DatabaseDeleted = "database.deleted"

	// DatabaseStatus is broadcast when database status changes
	DatabaseStatus = "database.status"

	// DatabaseUserCreated is broadcast when a database user is created
	DatabaseUserCreated = "database_user.created"

	// DatabaseUserUpdated is broadcast when a database user is updated
	DatabaseUserUpdated = "database_user.updated"

	// DatabaseUserDeleted is broadcast when a database user is deleted
	DatabaseUserDeleted = "database_user.deleted"

	// DatabaseUserStatus is broadcast when database user status changes
	DatabaseUserStatus = "database_user.status"
)

// Backup events
const (
	// BackupCreated is broadcast when a new backup is created
	BackupCreated = "backup.created"

	// BackupUpdated is broadcast when a backup is updated
	BackupUpdated = "backup.updated"

	// BackupDeleted is broadcast when a backup is deleted
	BackupDeleted = "backup.deleted"

	// BackupStarted is broadcast when a backup job starts
	BackupStarted = "backup.started"

	// BackupFinished is broadcast when a backup job completes
	BackupFinished = "backup.finished"

	// BackupFailed is broadcast when a backup job fails
	BackupFailed = "backup.failed"

	// BackupJobStatus is broadcast when backup job status changes
	BackupJobStatus = "backup.job.status"
)

// Cron events
const (
	// CronCreated is broadcast when a new cron job is created
	CronCreated = "cron.created"

	// CronUpdated is broadcast when a cron job is updated
	CronUpdated = "cron.updated"

	// CronDeleted is broadcast when a cron job is deleted
	CronDeleted = "cron.deleted"
)

// SSH Key events
const (
	// SSHKeyCreated is broadcast when a new SSH key is added
	SSHKeyCreated = "ssh_key.created"

	// SSHKeyDeleted is broadcast when an SSH key is removed
	SSHKeyDeleted = "ssh_key.deleted"
)

// Firewall events
const (
	// FirewallRuleCreated is broadcast when a firewall rule is created
	FirewallRuleCreated = "firewall_rule.created"

	// FirewallRuleDeleted is broadcast when a firewall rule is deleted
	FirewallRuleDeleted = "firewall_rule.deleted"
)

// Certificate events
const (
	// CertificateCreated is broadcast when a certificate is created
	CertificateCreated = "certificate.created"

	// CertificateUpdated is broadcast when a certificate is updated
	CertificateUpdated = "certificate.updated"

	// CertificateDeleted is broadcast when a certificate is deleted
	CertificateDeleted = "certificate.deleted"

	// CertificateRenewed is broadcast when a certificate is renewed
	CertificateRenewed = "certificate.renewed"

	// CertificateExpiring is broadcast when a certificate is about to expire
	CertificateExpiring = "certificate.expiring"
)

// Daemon events
const (
	// DaemonCreated is broadcast when a daemon is created
	DaemonCreated = "daemon.created"

	// DaemonUpdated is broadcast when a daemon is updated
	DaemonUpdated = "daemon.updated"

	// DaemonDeleted is broadcast when a daemon is deleted
	DaemonDeleted = "daemon.deleted"

	// DaemonStarted is broadcast when a daemon starts
	DaemonStarted = "daemon.started"

	// DaemonStopped is broadcast when a daemon stops
	DaemonStopped = "daemon.stopped"

	// DaemonRestarted is broadcast when a daemon restarts
	DaemonRestarted = "daemon.restarted"
)

// Queue events (Laravel queue workers)
const (
	// QueueCreated is broadcast when a queue worker is created
	QueueCreated = "queue.created"

	// QueueUpdated is broadcast when a queue worker is updated
	QueueUpdated = "queue.updated"

	// QueueDeleted is broadcast when a queue worker is deleted
	QueueDeleted = "queue.deleted"

	// QueueRestarted is broadcast when a queue worker is restarted
	QueueRestarted = "queue.restarted"
)

// Script execution events
const (
	// ScriptExecutionStarted is broadcast when a script execution starts
	ScriptExecutionStarted = "script.execution.started"

	// ScriptExecutionCompleted is broadcast when a script execution completes
	ScriptExecutionCompleted = "script.execution.completed"

	// ScriptExecutionFailed is broadcast when a script execution fails
	ScriptExecutionFailed = "script.execution.failed"

	// ScriptExecutionOutput is broadcast for real-time script output
	ScriptExecutionOutput = "script.execution.output"
)

// Service events (PHP, MySQL, PostgreSQL, Redis, etc.)
const (
	// ServiceStarted is broadcast when a service starts
	ServiceStarted = "service.started"

	// ServiceStopped is broadcast when a service stops
	ServiceStopped = "service.stopped"

	// ServiceRestarted is broadcast when a service restarts
	ServiceRestarted = "service.restarted"

	// ServiceStatus is broadcast when service status is checked
	ServiceStatus = "service.status"
)

// Notification events
const (
	// NotificationCreated is broadcast when a notification is created
	NotificationCreated = "notification.created"

	// NotificationRead is broadcast when a notification is marked as read
	NotificationRead = "notification.read"

	// NotificationDeleted is broadcast when a notification is deleted
	NotificationDeleted = "notification.deleted"
)

// Team events
const (
	// TeamUpdated is broadcast when team settings are updated
	TeamUpdated = "team.updated"

	// TeamMemberAdded is broadcast when a member is added to the team
	TeamMemberAdded = "team.member.added"

	// TeamMemberRemoved is broadcast when a member is removed from the team
	TeamMemberRemoved = "team.member.removed"

	// TeamMemberUpdated is broadcast when a member's role is updated
	TeamMemberUpdated = "team.member.updated"
)

// Activity events
const (
	// ActivityCreated is broadcast when a new activity is logged
	ActivityCreated = "activity.created"
)
