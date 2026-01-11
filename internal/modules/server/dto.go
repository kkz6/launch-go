package server

import (
	"time"
)

// CreateServerRequest represents the request body for creating a server
type CreateServerRequest struct {
	Name            string  `json:"name" validate:"required,min=2,max=255"`
	Description     *string `json:"description" validate:"omitempty,max=1000"`
	Provider        string  `json:"provider" validate:"required,oneof=digitalocean hetzner linode vultr aws custom_server"`
	Type            string  `json:"type" validate:"required,oneof=php database"`
	OperatingSystem string  `json:"operating_system" validate:"omitempty,oneof=ubuntu_20 ubuntu_22 ubuntu_24"`
	Region          string  `json:"region" validate:"required_unless=Provider custom_server"`
	Size            string  `json:"size" validate:"required_unless=Provider custom_server"`
	PHPVersion      string  `json:"php_version" validate:"omitempty,oneof=5.6 7.0 7.1 7.2 7.3 7.4 8.0 8.1 8.2 8.3 8.4"`
	DatabaseType    string  `json:"database_type" validate:"omitempty,oneof=mysql80 postgresql16"`
	CredentialID    string  `json:"credential_id" validate:"required_unless=Provider custom_server,omitempty,ulid"`

	// For custom servers
	IPAddress  string `json:"ip_address" validate:"required_if=Provider custom_server,omitempty,ip"`
	SSHPort    int    `json:"ssh_port" validate:"omitempty,min=1,max=65535"`
	SSHUser    string `json:"ssh_user" validate:"omitempty,max=100"`
	PrivateKey string `json:"private_key" validate:"required_if=Provider custom_server"`
}

// UpdateServerRequest represents the request body for updating a server
type UpdateServerRequest struct {
	Name              *string `json:"name" validate:"omitempty,min=2,max=255"`
	Description       *string `json:"description" validate:"omitempty,max=1000"`
	MonitoringEnabled *bool   `json:"monitoring_enabled"`
	AutoUpdate        *bool   `json:"auto_update"`
}

// ArchiveServerRequest represents the request body for archiving a server
type ArchiveServerRequest struct {
	Confirm bool `json:"confirm" validate:"required,eq=true"`
}

// CreateServiceRequest represents the request body for installing a service
type CreateServiceRequest struct {
	Software string `json:"software" validate:"required"`
}

// ServiceOperationRequest represents the request body for service operations
type ServiceOperationRequest struct {
	Operation string `json:"operation" validate:"required,oneof=start stop restart remove status"`
}

// CreateFirewallRuleRequest represents the request body for creating a firewall rule
type CreateFirewallRuleRequest struct {
	Name     string  `json:"name" validate:"required,min=1,max=255"`
	Action   string  `json:"action" validate:"required,oneof=allow deny reject"`
	Port     string  `json:"port" validate:"required,max=50"`
	FromIPv4 *string `json:"from_ipv4" validate:"omitempty,ip|cidr"`
	Mask     *string `json:"mask" validate:"omitempty,max=10"`
	Note     *string `json:"note" validate:"omitempty,max=500"`
}

// UpdateFirewallRuleRequest represents the request body for updating a firewall rule
type UpdateFirewallRuleRequest struct {
	Name     *string `json:"name" validate:"omitempty,min=1,max=255"`
	Action   *string `json:"action" validate:"omitempty,oneof=allow deny reject"`
	Port     *string `json:"port" validate:"omitempty,max=50"`
	FromIPv4 *string `json:"from_ipv4" validate:"omitempty,ip|cidr"`
	Mask     *string `json:"mask" validate:"omitempty,max=10"`
	Note     *string `json:"note" validate:"omitempty,max=500"`
}

// CreateCronRequest represents the request body for creating a cron job
type CreateCronRequest struct {
	User       string  `json:"user" validate:"omitempty,max=100"`
	Expression string  `json:"expression" validate:"required,max=100"`
	Command    string  `json:"command" validate:"required"`
	Frequency  *string `json:"frequency" validate:"omitempty,max=100"`
	SiteID     *string `json:"site_id" validate:"omitempty,ulid"`
}

// UpdateCronRequest represents the request body for updating a cron job
type UpdateCronRequest struct {
	User       *string `json:"user" validate:"omitempty,max=100"`
	Expression *string `json:"expression" validate:"omitempty,max=100"`
	Command    *string `json:"command"`
	Frequency  *string `json:"frequency" validate:"omitempty,max=100"`
}

// CreateDaemonRequest represents the request body for creating a daemon
type CreateDaemonRequest struct {
	User            string  `json:"user" validate:"omitempty,max=100"`
	Directory       *string `json:"directory" validate:"omitempty,max=255"`
	Command         string  `json:"command" validate:"required"`
	Processes       int     `json:"processes" validate:"omitempty,min=1,max=100"`
	StopWaitSeconds int     `json:"stop_wait_seconds" validate:"omitempty,min=1,max=600"`
	StopSignal      *string `json:"stop_signal" validate:"omitempty,max=20"`
}

// UpdateDaemonRequest represents the request body for updating a daemon
type UpdateDaemonRequest struct {
	User            *string `json:"user" validate:"omitempty,max=100"`
	Directory       *string `json:"directory" validate:"omitempty,max=255"`
	Command         *string `json:"command"`
	Processes       *int    `json:"processes" validate:"omitempty,min=1,max=100"`
	StopWaitSeconds *int    `json:"stop_wait_seconds" validate:"omitempty,min=1,max=600"`
	StopSignal      *string `json:"stop_signal" validate:"omitempty,max=20"`
}

// CreateSshKeyRequest represents the request body for creating an SSH key
type CreateSshKeyRequest struct {
	Name        string  `json:"name" validate:"required,min=1,max=255"`
	PublicKey   string  `json:"public_key" validate:"required"`
	Description *string `json:"description" validate:"omitempty,max=500"`
	IsGlobal    bool    `json:"is_global"`
}

// AttachSshKeyRequest represents the request body for attaching an SSH key to a server
type AttachSshKeyRequest struct {
	SshKeyID string `json:"ssh_key_id" validate:"required,ulid"`
}

// RunCommandRequest represents the request body for running a command on a server
type RunCommandRequest struct {
	Command string `json:"command" validate:"required"`
	User    string `json:"user" validate:"omitempty,max=100"`
}

// CreateDatabaseRequest represents the request body for creating a database
type CreateDatabaseRequest struct {
	Name string `json:"name" validate:"required,min=1,max=64"`
}

// DatabaseResponse represents the response for a database
type DatabaseResponse struct {
	Name      string `json:"name"`
	CreatedAt string `json:"created_at,omitempty"`
}

// ServerResponse represents the response for a server
type ServerResponse struct {
	ID                    string   `json:"id"`
	TeamID                string   `json:"team_id"`
	Name                  string   `json:"name"`
	Description           *string  `json:"description,omitempty"`
	Provider              string   `json:"provider"`
	ProviderLabel         string   `json:"provider_label"`
	Type                  string   `json:"type"`
	TypeLabel             string   `json:"type_label"`
	Connected             bool     `json:"connected"`
	MonitoringEnabled     bool     `json:"monitoring_enabled"`
	CPUCores              *int     `json:"cpu_cores,omitempty"`
	MemoryInMB            *int     `json:"memory_in_mb,omitempty"`
	StorageInGB           *int     `json:"storage_in_gb,omitempty"`
	OperatingSystem       string   `json:"operating_system"`
	OperatingSystemLabel  string   `json:"operating_system_label"`
	Status                string   `json:"status"`
	StatusLabel           string   `json:"status_label"`
	PublicIPv4            *string  `json:"public_ipv4,omitempty"`
	Username              string   `json:"username"`
	SSHPort               int      `json:"ssh_port"`
	AutoUpdate            bool     `json:"auto_update"`
	Progress              *int     `json:"progress,omitempty"`
	ProgressStep          *string  `json:"progress_step,omitempty"`
	ProvisionedAt         *string  `json:"provisioned_at,omitempty"`
	LastConnectivityCheck *string  `json:"last_connectivity_check,omitempty"`
	ArchivedAt            *string  `json:"archived_at,omitempty"`
	CreatedAt             string   `json:"created_at"`
	UpdatedAt             string   `json:"updated_at"`
	ProvisionCommand      string   `json:"provision_command,omitempty"`
	Features              []string `json:"features,omitempty"`
	SitesCount            int      `json:"sites_count,omitempty"`
	ServicesCount         int      `json:"services_count,omitempty"`
}

// ToServerResponse converts a Server model to a ServerResponse DTO
func ToServerResponse(server *Server) ServerResponse {
	resp := ServerResponse{
		ID:                   server.ID,
		TeamID:               server.TeamID,
		Name:                 server.Name,
		Description:          server.Description,
		Provider:             server.Provider.String(),
		ProviderLabel:        server.Provider.Label(),
		Type:                 server.Type.String(),
		TypeLabel:            server.Type.Label(),
		Connected:            server.Connected,
		MonitoringEnabled:    server.MonitoringEnabled,
		CPUCores:             server.CPUCores,
		MemoryInMB:           server.MemoryInMB,
		StorageInGB:          server.StorageInGB,
		OperatingSystem:      server.OperatingSystem.String(),
		OperatingSystemLabel: server.OperatingSystem.Label(),
		Status:               server.Status.String(),
		StatusLabel:          server.Status.Label(),
		PublicIPv4:           server.PublicIPv4,
		Username:             server.Username,
		SSHPort:              server.SSHPort,
		AutoUpdate:           server.AutoUpdate,
		Progress:             server.Progress,
		ProgressStep:         server.ProgressStep,
		CreatedAt:            server.CreatedAt.Format(time.RFC3339),
		UpdatedAt:            server.UpdatedAt.Format(time.RFC3339),
		ServicesCount:        len(server.Services),
	}

	if server.ProvisionedAt != nil {
		formatted := server.ProvisionedAt.Format(time.RFC3339)
		resp.ProvisionedAt = &formatted
	}

	if server.LastConnectivityCheck != nil {
		formatted := server.LastConnectivityCheck.Format(time.RFC3339)
		resp.LastConnectivityCheck = &formatted
	}

	if server.ArchivedAt != nil {
		formatted := server.ArchivedAt.Format(time.RFC3339)
		resp.ArchivedAt = &formatted
	}

	if server.Status == ServerStatusNew {
		resp.ProvisionCommand = server.GetProvisionCommand()
	}

	features := server.GetFeatures()
	resp.Features = make([]string, len(features))
	for i, f := range features {
		resp.Features[i] = f.String()
	}

	return resp
}

// ServerListResponse represents a list of servers
type ServerListResponse struct {
	Servers []ServerResponse `json:"servers"`
	Total   int64            `json:"total"`
}

// ServiceResponse represents the response for a service
type ServiceResponse struct {
	ID            string  `json:"id"`
	ServerID      string  `json:"server_id"`
	Type          string  `json:"type"`
	TypeLabel     string  `json:"type_label"`
	Name          string  `json:"name"`
	Version       *string `json:"version,omitempty"`
	Status        string  `json:"status"`
	StatusLabel   string  `json:"status_label"`
	IsDefault     bool    `json:"is_default"`
	Software      *string `json:"software,omitempty"`
	SoftwareLabel *string `json:"software_label,omitempty"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

// ToServiceResponse converts an InstalledService model to a ServiceResponse DTO
func ToServiceResponse(service *InstalledService) ServiceResponse {
	resp := ServiceResponse{
		ID:          service.ID,
		ServerID:    service.ServerID,
		Type:        service.Type.String(),
		TypeLabel:   service.Type.Label(),
		Name:        service.Name,
		Version:     service.Version,
		Status:      service.Status.String(),
		StatusLabel: service.Status.Label(),
		IsDefault:   service.IsDefault,
		CreatedAt:   service.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   service.UpdatedAt.Format(time.RFC3339),
	}

	if service.Software != nil {
		sw := service.Software.String()
		resp.Software = &sw
		label := service.Software.Label()
		resp.SoftwareLabel = &label
	}

	return resp
}

// FirewallRuleResponse represents the response for a firewall rule
type FirewallRuleResponse struct {
	ID           string  `json:"id"`
	ServerID     string  `json:"server_id"`
	Name         string  `json:"name"`
	Action       string  `json:"action"`
	ActionLabel  string  `json:"action_label"`
	Port         *string `json:"port,omitempty"`
	FromIPv4     *string `json:"from_ipv4,omitempty"`
	Mask         *string `json:"mask,omitempty"`
	Note         *string `json:"note,omitempty"`
	IsInstalled  bool    `json:"is_installed"`
	IsPending    bool    `json:"is_pending"`
	HasFailed    bool    `json:"has_failed"`
	InstalledAt  *string `json:"installed_at,omitempty"`
	UfwRule      string  `json:"ufw_rule"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

// ToFirewallRuleResponse converts a FirewallRule model to a FirewallRuleResponse DTO
func ToFirewallRuleResponse(rule *FirewallRule) FirewallRuleResponse {
	resp := FirewallRuleResponse{
		ID:          rule.ID,
		ServerID:    rule.ServerID,
		Name:        rule.Name,
		Action:      rule.Action.String(),
		ActionLabel: rule.Action.Label(),
		Port:        rule.Port,
		FromIPv4:    rule.FromIPv4,
		Mask:        rule.Mask,
		Note:        rule.Note,
		IsInstalled: rule.IsInstalled(),
		IsPending:   rule.IsPending(),
		HasFailed:   rule.HasFailed(),
		UfwRule:     rule.FormatAsUfwRule(),
		CreatedAt:   rule.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   rule.UpdatedAt.Format(time.RFC3339),
	}

	if rule.InstalledAt != nil {
		formatted := rule.InstalledAt.Format(time.RFC3339)
		resp.InstalledAt = &formatted
	}

	return resp
}

// CronResponse represents the response for a cron job
type CronResponse struct {
	ID          string  `json:"id"`
	ServerID    string  `json:"server_id"`
	SiteID      *string `json:"site_id,omitempty"`
	User        string  `json:"user"`
	Expression  string  `json:"expression"`
	Command     string  `json:"command"`
	Frequency   *string `json:"frequency,omitempty"`
	Hidden      bool    `json:"hidden"`
	IsInstalled bool    `json:"is_installed"`
	Path        string  `json:"path"`
	InstalledAt *string `json:"installed_at,omitempty"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// ToCronResponse converts a Cron model to a CronResponse DTO
func ToCronResponse(cron *Cron) CronResponse {
	resp := CronResponse{
		ID:          cron.ID,
		ServerID:    cron.ServerID,
		SiteID:      cron.SiteID,
		User:        cron.User,
		Expression:  cron.Expression,
		Command:     cron.Command,
		Frequency:   cron.Frequency,
		Hidden:      cron.Hidden,
		IsInstalled: cron.IsInstalled(),
		Path:        cron.Path(),
		CreatedAt:   cron.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   cron.UpdatedAt.Format(time.RFC3339),
	}

	if cron.InstalledAt != nil {
		formatted := cron.InstalledAt.Format(time.RFC3339)
		resp.InstalledAt = &formatted
	}

	return resp
}

// DaemonResponse represents the response for a daemon
type DaemonResponse struct {
	ID              string  `json:"id"`
	ServerID        string  `json:"server_id"`
	User            string  `json:"user"`
	Directory       *string `json:"directory,omitempty"`
	Command         string  `json:"command"`
	Processes       int     `json:"processes"`
	StopWaitSeconds int     `json:"stop_wait_seconds"`
	StopSignal      *string `json:"stop_signal,omitempty"`
	IsInstalled     bool    `json:"is_installed"`
	Running         bool    `json:"running"`
	Path            string  `json:"path"`
	InstalledAt     *string `json:"installed_at,omitempty"`
	LastStatusCheck *string `json:"last_status_check,omitempty"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

// ToDaemonResponse converts a Daemon model to a DaemonResponse DTO
func ToDaemonResponse(daemon *Daemon) DaemonResponse {
	resp := DaemonResponse{
		ID:              daemon.ID,
		ServerID:        daemon.ServerID,
		User:            daemon.User,
		Directory:       daemon.Directory,
		Command:         daemon.Command,
		Processes:       daemon.Processes,
		StopWaitSeconds: daemon.StopWaitSeconds,
		StopSignal:      daemon.StopSignal,
		IsInstalled:     daemon.IsInstalled(),
		Running:         daemon.Running,
		Path:            daemon.Path(),
		CreatedAt:       daemon.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       daemon.UpdatedAt.Format(time.RFC3339),
	}

	if daemon.InstalledAt != nil {
		formatted := daemon.InstalledAt.Format(time.RFC3339)
		resp.InstalledAt = &formatted
	}

	if daemon.LastStatusCheck != nil {
		formatted := daemon.LastStatusCheck.Format(time.RFC3339)
		resp.LastStatusCheck = &formatted
	}

	return resp
}

// SshKeyResponse represents the response for an SSH key
type SshKeyResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Fingerprint string  `json:"fingerprint"`
	Description *string `json:"description,omitempty"`
	IsGlobal    bool    `json:"is_global"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// ToSshKeyResponse converts an SshKey model to an SshKeyResponse DTO
func ToSshKeyResponse(key *SshKey) SshKeyResponse {
	return SshKeyResponse{
		ID:          key.ID,
		Name:        key.Name,
		Fingerprint: key.GetFingerprint(),
		Description: key.Description,
		IsGlobal:    key.IsGlobal,
		CreatedAt:   key.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   key.UpdatedAt.Format(time.RFC3339),
	}
}

// TaskResponse represents the response for a task
type TaskResponse struct {
	ID         string  `json:"id"`
	ServerID   string  `json:"server_id"`
	Type       string  `json:"type"`
	Status     string  `json:"status"`
	Name       *string `json:"name,omitempty"`
	User       *string `json:"user,omitempty"`
	Output     *string `json:"output,omitempty"`
	ExitCode   *int    `json:"exit_code,omitempty"`
	Duration   *string `json:"duration,omitempty"`
	StartedAt  *string `json:"started_at,omitempty"`
	FinishedAt *string `json:"finished_at,omitempty"`
	CreatedAt  string  `json:"created_at"`
}

// ToTaskResponse converts a Task model to a TaskResponse DTO
func ToTaskResponse(task *Task) TaskResponse {
	resp := TaskResponse{
		ID:        task.ID,
		ServerID:  task.ServerID,
		Type:      task.Type,
		Status:    task.Status,
		Name:      task.Name,
		User:      task.User,
		Output:    task.Output,
		ExitCode:  task.ExitCode,
		CreatedAt: task.CreatedAt.Format(time.RFC3339),
	}

	if task.StartedAt != nil {
		formatted := task.StartedAt.Format(time.RFC3339)
		resp.StartedAt = &formatted
	}

	if task.FinishedAt != nil {
		formatted := task.FinishedAt.Format(time.RFC3339)
		resp.FinishedAt = &formatted
	}

	if duration := task.Duration(); duration > 0 {
		durationStr := duration.String()
		resp.Duration = &durationStr
	}

	return resp
}

// MetricResponse represents the response for server metrics
type MetricResponse struct {
	ID            string  `json:"id"`
	ServerID      string  `json:"server_id"`
	CPUUsage      float64 `json:"cpu_usage"`
	MemoryUsage   float64 `json:"memory_usage"`
	DiskUsage     float64 `json:"disk_usage"`
	LoadAverage1  float64 `json:"load_average_1"`
	LoadAverage5  float64 `json:"load_average_5"`
	LoadAverage15 float64 `json:"load_average_15"`
	RecordedAt    string  `json:"recorded_at"`
}

// ToMetricResponse converts a Metric model to a MetricResponse DTO
func ToMetricResponse(metric *Metric) MetricResponse {
	return MetricResponse{
		ID:            metric.ID,
		ServerID:      metric.ServerID,
		CPUUsage:      metric.CPUUsage,
		MemoryUsage:   metric.MemoryUsage,
		DiskUsage:     metric.DiskUsage,
		LoadAverage1:  metric.LoadAverage1,
		LoadAverage5:  metric.LoadAverage5,
		LoadAverage15: metric.LoadAverage15,
		RecordedAt:    metric.RecordedAt.Format(time.RFC3339),
	}
}

// AvailableSoftwareResponse represents available software for installation
type AvailableSoftwareResponse struct {
	Group        string                    `json:"group"`
	Label        string                    `json:"label"`
	Type         string                    `json:"type"`
	ImagePath    string                    `json:"image_path"`
	Installed    bool                      `json:"installed"`
	Status       *string                   `json:"status,omitempty"`
	Versions     []SoftwareVersionResponse `json:"versions"`
	HasStart     bool                      `json:"has_start"`
	HasStop      bool                      `json:"has_stop"`
	HasRestart   bool                      `json:"has_restart"`
	HasRemove    bool                      `json:"has_remove"`
	HasStatus    bool                      `json:"has_status"`
}

// SoftwareVersionResponse represents a specific software version
type SoftwareVersionResponse struct {
	Software  string  `json:"software"`
	Label     string  `json:"label"`
	Version   string  `json:"version"`
	Installed bool    `json:"installed"`
	Status    *string `json:"status,omitempty"`
}

// ProviderResponse represents cloud provider options
type ProviderResponse struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// GetAllProviders returns all available server providers
func GetAllProviders() []ProviderResponse {
	providers := AllServerProviders()
	result := make([]ProviderResponse, len(providers))
	for i, p := range providers {
		result[i] = ProviderResponse{
			Value: p.String(),
			Label: p.Label(),
		}
	}
	return result
}

// GetAllServerTypes returns all available server types
func GetAllServerTypes() []ProviderResponse {
	types := AllServerTypes()
	result := make([]ProviderResponse, len(types))
	for i, t := range types {
		result[i] = ProviderResponse{
			Value: t.String(),
			Label: t.Label(),
		}
	}
	return result
}

// GetAllOperatingSystems returns all available operating systems
func GetAllOperatingSystems() []ProviderResponse {
	oses := AllOperatingSystems()
	result := make([]ProviderResponse, len(oses))
	for i, os := range oses {
		result[i] = ProviderResponse{
			Value: os.String(),
			Label: os.Label(),
		}
	}
	return result
}

// GetAllRuleActions returns all available firewall rule actions
func GetAllRuleActions() []ProviderResponse {
	actions := AllRuleActions()
	result := make([]ProviderResponse, len(actions))
	for i, a := range actions {
		result[i] = ProviderResponse{
			Value: a.String(),
			Label: a.Label(),
		}
	}
	return result
}

// ServerShowPageData represents the data for the server show page
type ServerShowPageData struct {
	Server         ServerResponse           `json:"server"`
	Services       []ServiceResponse        `json:"services"`
	FirewallRules  []FirewallRuleResponse   `json:"firewall_rules"`
	Crons          []CronResponse           `json:"crons"`
	Daemons        []DaemonResponse         `json:"daemons"`
	SshKeys        []SshKeyResponse         `json:"ssh_keys"`
	LatestTask     *TaskResponse            `json:"latest_task,omitempty"`
	LatestMetric   *MetricResponse          `json:"latest_metric,omitempty"`
	RuleActions    []ProviderResponse       `json:"rule_actions"`
	HasLaunchAgent bool                     `json:"has_launch_agent"`
}

// ServerIndexPageData represents the data for the server index page
type ServerIndexPageData struct {
	Servers          []ServerResponse   `json:"servers"`
	Total            int64              `json:"total"`
	Providers        []ProviderResponse `json:"providers"`
	ServerTypes      []ProviderResponse `json:"server_types"`
	OperatingSystems []ProviderResponse `json:"operating_systems"`
}

// GetServerIndexPageData builds the server index page data
func GetServerIndexPageData(servers []Server, total int64) ServerIndexPageData {
	serverResponses := make([]ServerResponse, len(servers))
	for i, s := range servers {
		serverResponses[i] = ToServerResponse(&s)
	}

	return ServerIndexPageData{
		Servers:          serverResponses,
		Total:            total,
		Providers:        GetAllProviders(),
		ServerTypes:      GetAllServerTypes(),
		OperatingSystems: GetAllOperatingSystems(),
	}
}
