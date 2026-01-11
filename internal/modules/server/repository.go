package server

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

var (
	ErrServerNotFound       = errors.New("server not found")
	ErrServiceNotFound      = errors.New("service not found")
	ErrFirewallRuleNotFound = errors.New("firewall rule not found")
	ErrCronNotFound         = errors.New("cron job not found")
	ErrDaemonNotFound       = errors.New("daemon not found")
	ErrSshKeyNotFound       = errors.New("ssh key not found")
	ErrTaskNotFound         = errors.New("task not found")
	ErrQueueNotConfigured   = errors.New("queue not configured")
)

// Repository provides database operations for server module
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new Repository instance
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Server operations

// CreateServer creates a new server
func (r *Repository) CreateServer(ctx context.Context, server *Server) error {
	return r.db.WithContext(ctx).Create(server).Error
}

// FindServerByID finds a server by ID
func (r *Repository) FindServerByID(ctx context.Context, id string) (*Server, error) {
	var server Server
	err := r.db.WithContext(ctx).
		Preload("Services").
		First(&server, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServerNotFound
		}
		return nil, err
	}
	return &server, nil
}

// FindServerByIDAndTeam finds a server by ID and team ID
func (r *Repository) FindServerByIDAndTeam(ctx context.Context, id, teamID string) (*Server, error) {
	var server Server
	err := r.db.WithContext(ctx).
		Preload("Services").
		First(&server, "id = ? AND team_id = ?", id, teamID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServerNotFound
		}
		return nil, err
	}
	return &server, nil
}

// FindServerWithRelations finds a server with all relations
func (r *Repository) FindServerWithRelations(ctx context.Context, id, teamID string) (*Server, error) {
	var server Server
	err := r.db.WithContext(ctx).
		Preload("Services").
		Preload("FirewallRules").
		Preload("Crons").
		Preload("Daemons").
		Preload("SshKeys").
		First(&server, "id = ? AND team_id = ?", id, teamID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServerNotFound
		}
		return nil, err
	}
	return &server, nil
}

// FindAllServersByTeam finds all servers for a team
func (r *Repository) FindAllServersByTeam(ctx context.Context, teamID string) ([]Server, error) {
	var servers []Server
	err := r.db.WithContext(ctx).
		Preload("Services").
		Where("team_id = ?", teamID).
		Order("created_at DESC").
		Find(&servers).Error
	return servers, err
}

// FindAllServersByTeamPaginated finds all servers for a team with pagination
func (r *Repository) FindAllServersByTeamPaginated(ctx context.Context, teamID string, limit, offset int) ([]Server, int64, error) {
	var servers []Server
	var total int64

	query := r.db.WithContext(ctx).Model(&Server{}).Where("team_id = ?", teamID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.
		Preload("Services").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&servers).Error

	return servers, total, err
}

// FindArchivedServersByTeam finds all archived servers for a team
func (r *Repository) FindArchivedServersByTeam(ctx context.Context, teamID string) ([]Server, error) {
	var servers []Server
	err := r.db.WithContext(ctx).
		Unscoped().
		Where("team_id = ? AND archived_at IS NOT NULL", teamID).
		Order("archived_at DESC").
		Find(&servers).Error
	return servers, err
}

// UpdateServer updates a server
func (r *Repository) UpdateServer(ctx context.Context, server *Server) error {
	return r.db.WithContext(ctx).Save(server).Error
}

// UpdateServerStatus updates only the server status
func (r *Repository) UpdateServerStatus(ctx context.Context, id string, status ServerStatus) error {
	return r.db.WithContext(ctx).
		Model(&Server{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// UpdateServerProgress updates server provisioning progress
func (r *Repository) UpdateServerProgress(ctx context.Context, id string, progress int, step string) error {
	return r.db.WithContext(ctx).
		Model(&Server{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"progress":      progress,
			"progress_step": step,
		}).Error
}

// UpdateServerFields updates specific fields on a server
func (r *Repository) UpdateServerFields(ctx context.Context, id string, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&Server{}).
		Where("id = ?", id).
		Updates(fields).Error
}

// ArchiveServer archives a server
func (r *Repository) ArchiveServer(ctx context.Context, id string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&Server{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"archived_at": now,
			"status":      ServerStatusArchived,
		}).Error
}

// UnarchiveServer unarchives a server
func (r *Repository) UnarchiveServer(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).
		Model(&Server{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"archived_at": nil,
			"status":      ServerStatusStopped,
		}).Error
}

// DeleteServer deletes a server
func (r *Repository) DeleteServer(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&Server{}, "id = ?", id).Error
}

// Service operations

// CreateService creates a new service
func (r *Repository) CreateService(ctx context.Context, service *InstalledService) error {
	return r.db.WithContext(ctx).Create(service).Error
}

// FindServiceByID finds a service by ID
func (r *Repository) FindServiceByID(ctx context.Context, id string) (*InstalledService, error) {
	var service InstalledService
	err := r.db.WithContext(ctx).First(&service, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServiceNotFound
		}
		return nil, err
	}
	return &service, nil
}

// FindServicesByServer finds all services for a server
func (r *Repository) FindServicesByServer(ctx context.Context, serverID string) ([]InstalledService, error) {
	var services []InstalledService
	err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&services).Error
	return services, err
}

// FindServiceByServerAndType finds a service by server and type
func (r *Repository) FindServiceByServerAndType(ctx context.Context, serverID string, serviceType ServiceType) (*InstalledService, error) {
	var service InstalledService
	err := r.db.WithContext(ctx).
		First(&service, "server_id = ? AND type = ?", serverID, serviceType).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServiceNotFound
		}
		return nil, err
	}
	return &service, nil
}

// FindServiceByServerAndSoftware finds a service by server and software
func (r *Repository) FindServiceByServerAndSoftware(ctx context.Context, serverID string, software Software) (*InstalledService, error) {
	var service InstalledService
	err := r.db.WithContext(ctx).
		First(&service, "server_id = ? AND software = ?", serverID, software).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServiceNotFound
		}
		return nil, err
	}
	return &service, nil
}

// FindDatabaseService finds the database service for a server
func (r *Repository) FindDatabaseService(ctx context.Context, serverID string) (*InstalledService, error) {
	var service InstalledService
	err := r.db.WithContext(ctx).
		First(&service, "server_id = ? AND (type = ? OR type = ?)", serverID, ServiceTypeMySql, ServiceTypePostgreSql).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServiceNotFound
		}
		return nil, err
	}
	return &service, nil
}

// UpdateService updates a service
func (r *Repository) UpdateService(ctx context.Context, service *InstalledService) error {
	return r.db.WithContext(ctx).Save(service).Error
}

// UpdateServiceStatus updates the service status
func (r *Repository) UpdateServiceStatus(ctx context.Context, id string, status ServiceStatus) error {
	return r.db.WithContext(ctx).
		Model(&InstalledService{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// DeleteService deletes a service
func (r *Repository) DeleteService(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&InstalledService{}, "id = ?", id).Error
}

// FirewallRule operations

// CreateFirewallRule creates a new firewall rule
func (r *Repository) CreateFirewallRule(ctx context.Context, rule *FirewallRule) error {
	return r.db.WithContext(ctx).Create(rule).Error
}

// FindFirewallRuleByID finds a firewall rule by ID
func (r *Repository) FindFirewallRuleByID(ctx context.Context, id string) (*FirewallRule, error) {
	var rule FirewallRule
	err := r.db.WithContext(ctx).First(&rule, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFirewallRuleNotFound
		}
		return nil, err
	}
	return &rule, nil
}

// FindFirewallRuleByIDAndServer finds a firewall rule by ID and server ID
func (r *Repository) FindFirewallRuleByIDAndServer(ctx context.Context, id, serverID string) (*FirewallRule, error) {
	var rule FirewallRule
	err := r.db.WithContext(ctx).
		First(&rule, "id = ? AND server_id = ?", id, serverID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFirewallRuleNotFound
		}
		return nil, err
	}
	return &rule, nil
}

// FindFirewallRulesByServer finds all firewall rules for a server
func (r *Repository) FindFirewallRulesByServer(ctx context.Context, serverID string) ([]FirewallRule, error) {
	var rules []FirewallRule
	err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&rules).Error
	return rules, err
}

// UpdateFirewallRule updates a firewall rule
func (r *Repository) UpdateFirewallRule(ctx context.Context, rule *FirewallRule) error {
	return r.db.WithContext(ctx).Save(rule).Error
}

// MarkFirewallRuleInstalled marks a firewall rule as installed
func (r *Repository) MarkFirewallRuleInstalled(ctx context.Context, id string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&FirewallRule{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"installed_at":           now,
			"installation_failed_at": nil,
		}).Error
}

// MarkFirewallRuleFailed marks a firewall rule installation as failed
func (r *Repository) MarkFirewallRuleFailed(ctx context.Context, id string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&FirewallRule{}).
		Where("id = ?", id).
		Update("installation_failed_at", now).Error
}

// DeleteFirewallRule deletes a firewall rule
func (r *Repository) DeleteFirewallRule(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&FirewallRule{}, "id = ?", id).Error
}

// Cron operations

// CreateCron creates a new cron job
func (r *Repository) CreateCron(ctx context.Context, cron *Cron) error {
	return r.db.WithContext(ctx).Create(cron).Error
}

// FindCronByID finds a cron job by ID
func (r *Repository) FindCronByID(ctx context.Context, id string) (*Cron, error) {
	var cron Cron
	err := r.db.WithContext(ctx).First(&cron, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCronNotFound
		}
		return nil, err
	}
	return &cron, nil
}

// FindCronByIDAndServer finds a cron job by ID and server ID
func (r *Repository) FindCronByIDAndServer(ctx context.Context, id, serverID string) (*Cron, error) {
	var cron Cron
	err := r.db.WithContext(ctx).
		First(&cron, "id = ? AND server_id = ?", id, serverID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCronNotFound
		}
		return nil, err
	}
	return &cron, nil
}

// FindCronsByServer finds all cron jobs for a server
func (r *Repository) FindCronsByServer(ctx context.Context, serverID string) ([]Cron, error) {
	var crons []Cron
	err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&crons).Error
	return crons, err
}

// FindVisibleCronsByServer finds all visible (non-hidden) cron jobs for a server
func (r *Repository) FindVisibleCronsByServer(ctx context.Context, serverID string) ([]Cron, error) {
	var crons []Cron
	err := r.db.WithContext(ctx).
		Where("server_id = ? AND hidden = ?", serverID, false).
		Order("created_at DESC").
		Find(&crons).Error
	return crons, err
}

// UpdateCron updates a cron job
func (r *Repository) UpdateCron(ctx context.Context, cron *Cron) error {
	return r.db.WithContext(ctx).Save(cron).Error
}

// MarkCronInstalled marks a cron job as installed
func (r *Repository) MarkCronInstalled(ctx context.Context, id string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&Cron{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"installed_at":           now,
			"installation_failed_at": nil,
		}).Error
}

// DeleteCron deletes a cron job
func (r *Repository) DeleteCron(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&Cron{}, "id = ?", id).Error
}

// Daemon operations

// CreateDaemon creates a new daemon
func (r *Repository) CreateDaemon(ctx context.Context, daemon *Daemon) error {
	return r.db.WithContext(ctx).Create(daemon).Error
}

// FindDaemonByID finds a daemon by ID
func (r *Repository) FindDaemonByID(ctx context.Context, id string) (*Daemon, error) {
	var daemon Daemon
	err := r.db.WithContext(ctx).First(&daemon, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDaemonNotFound
		}
		return nil, err
	}
	return &daemon, nil
}

// FindDaemonByIDAndServer finds a daemon by ID and server ID
func (r *Repository) FindDaemonByIDAndServer(ctx context.Context, id, serverID string) (*Daemon, error) {
	var daemon Daemon
	err := r.db.WithContext(ctx).
		First(&daemon, "id = ? AND server_id = ?", id, serverID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDaemonNotFound
		}
		return nil, err
	}
	return &daemon, nil
}

// FindDaemonsByServer finds all daemons for a server
func (r *Repository) FindDaemonsByServer(ctx context.Context, serverID string) ([]Daemon, error) {
	var daemons []Daemon
	err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&daemons).Error
	return daemons, err
}

// UpdateDaemon updates a daemon
func (r *Repository) UpdateDaemon(ctx context.Context, daemon *Daemon) error {
	return r.db.WithContext(ctx).Save(daemon).Error
}

// UpdateDaemonStatus updates a daemon's running status
func (r *Repository) UpdateDaemonStatus(ctx context.Context, id string, running bool) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&Daemon{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"running":           running,
			"last_status_check": now,
		}).Error
}

// MarkDaemonInstalled marks a daemon as installed
func (r *Repository) MarkDaemonInstalled(ctx context.Context, id string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&Daemon{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"installed_at":           now,
			"installation_failed_at": nil,
		}).Error
}

// DeleteDaemon deletes a daemon
func (r *Repository) DeleteDaemon(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&Daemon{}, "id = ?", id).Error
}

// SSH Key operations

// CreateSshKey creates a new SSH key
func (r *Repository) CreateSshKey(ctx context.Context, key *SshKey) error {
	return r.db.WithContext(ctx).Create(key).Error
}

// FindSshKeyByID finds an SSH key by ID
func (r *Repository) FindSshKeyByID(ctx context.Context, id string) (*SshKey, error) {
	var key SshKey
	err := r.db.WithContext(ctx).First(&key, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSshKeyNotFound
		}
		return nil, err
	}
	return &key, nil
}

// FindSshKeysByTeam finds all SSH keys for a team
func (r *Repository) FindSshKeysByTeam(ctx context.Context, teamID string) ([]SshKey, error) {
	var keys []SshKey
	err := r.db.WithContext(ctx).
		Where("team_id = ? OR is_global = ?", teamID, true).
		Order("created_at DESC").
		Find(&keys).Error
	return keys, err
}

// FindSshKeysByServer finds all SSH keys attached to a server
func (r *Repository) FindSshKeysByServer(ctx context.Context, serverID string) ([]SshKey, error) {
	var keys []SshKey
	err := r.db.WithContext(ctx).
		Joins("JOIN server_ssh_keys ON server_ssh_keys.ssh_key_id = ssh_keys.id").
		Where("server_ssh_keys.server_id = ?", serverID).
		Find(&keys).Error
	return keys, err
}

// FindGlobalSshKeys finds all global SSH keys
func (r *Repository) FindGlobalSshKeys(ctx context.Context) ([]SshKey, error) {
	var keys []SshKey
	err := r.db.WithContext(ctx).
		Where("is_global = ?", true).
		Order("created_at DESC").
		Find(&keys).Error
	return keys, err
}

// UpdateSshKey updates an SSH key
func (r *Repository) UpdateSshKey(ctx context.Context, key *SshKey) error {
	return r.db.WithContext(ctx).Save(key).Error
}

// DeleteSshKey deletes an SSH key
func (r *Repository) DeleteSshKey(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&SshKey{}, "id = ?", id).Error
}

// AttachSshKeyToServer attaches an SSH key to a server
func (r *Repository) AttachSshKeyToServer(ctx context.Context, serverID, sshKeyID string) error {
	return r.db.WithContext(ctx).Create(&ServerSshKey{
		ServerID: serverID,
		SshKeyID: sshKeyID,
	}).Error
}

// DetachSshKeyFromServer detaches an SSH key from a server
func (r *Repository) DetachSshKeyFromServer(ctx context.Context, serverID, sshKeyID string) error {
	return r.db.WithContext(ctx).
		Where("server_id = ? AND ssh_key_id = ?", serverID, sshKeyID).
		Delete(&ServerSshKey{}).Error
}

// IsSshKeyAttachedToServer checks if an SSH key is attached to a server
func (r *Repository) IsSshKeyAttachedToServer(ctx context.Context, serverID, sshKeyID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&ServerSshKey{}).
		Where("server_id = ? AND ssh_key_id = ?", serverID, sshKeyID).
		Count(&count).Error
	return count > 0, err
}

// Task operations

// CreateTask creates a new task
func (r *Repository) CreateTask(ctx context.Context, task *Task) error {
	return r.db.WithContext(ctx).Create(task).Error
}

// FindTaskByID finds a task by ID
func (r *Repository) FindTaskByID(ctx context.Context, id string) (*Task, error) {
	var task Task
	err := r.db.WithContext(ctx).First(&task, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}
	return &task, nil
}

// FindTasksByServer finds all tasks for a server
func (r *Repository) FindTasksByServer(ctx context.Context, serverID string, limit int) ([]Task, error) {
	var tasks []Task
	query := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&tasks).Error
	return tasks, err
}

// FindLatestTaskByServer finds the latest task for a server
func (r *Repository) FindLatestTaskByServer(ctx context.Context, serverID string) (*Task, error) {
	var task Task
	err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("created_at DESC").
		First(&task).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &task, nil
}

// UpdateTask updates a task
func (r *Repository) UpdateTask(ctx context.Context, task *Task) error {
	return r.db.WithContext(ctx).Save(task).Error
}

// Metric operations

// CreateMetric creates a new metric
func (r *Repository) CreateMetric(ctx context.Context, metric *Metric) error {
	return r.db.WithContext(ctx).Create(metric).Error
}

// FindMetricsByServer finds metrics for a server with optional time range
func (r *Repository) FindMetricsByServer(ctx context.Context, serverID string, from, to *time.Time, limit int) ([]Metric, error) {
	var metrics []Metric
	query := r.db.WithContext(ctx).
		Where("server_id = ?", serverID)

	if from != nil {
		query = query.Where("recorded_at >= ?", from)
	}
	if to != nil {
		query = query.Where("recorded_at <= ?", to)
	}

	query = query.Order("recorded_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&metrics).Error
	return metrics, err
}

// FindLatestMetricByServer finds the latest metric for a server
func (r *Repository) FindLatestMetricByServer(ctx context.Context, serverID string) (*Metric, error) {
	var metric Metric
	err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("recorded_at DESC").
		First(&metric).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &metric, nil
}

// DeleteOldMetrics deletes metrics older than a certain time
func (r *Repository) DeleteOldMetrics(ctx context.Context, serverID string, before time.Time) error {
	return r.db.WithContext(ctx).
		Where("server_id = ? AND recorded_at < ?", serverID, before).
		Delete(&Metric{}).Error
}

// CountServersByTeam counts the total number of servers for a team
func (r *Repository) CountServersByTeam(ctx context.Context, teamID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&Server{}).
		Where("team_id = ?", teamID).
		Count(&count).Error
	return count, err
}

// ServerHasLaunchAgent checks if a server has the Launch Agent installed
func (r *Repository) ServerHasLaunchAgent(ctx context.Context, serverID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&InstalledService{}).
		Where("server_id = ? AND type = ?", serverID, ServiceTypeLaunchAgent).
		Count(&count).Error
	return count > 0, err
}

// Transaction executes a function within a database transaction
func (r *Repository) Transaction(ctx context.Context, fn func(tx *Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&Repository{db: tx})
	})
}
