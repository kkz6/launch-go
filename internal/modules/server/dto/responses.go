package dto

import (
	"time"

	serverconfig "github.com/kkz6/launch-go/internal/modules/server/config"
	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/models"
)

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
func ToServerResponse(server *models.Server) ServerResponse {
	// Handle Type field (now *string instead of enum)
	serverType := ""
	serverTypeLabel := ""
	if server.Type != nil && *server.Type != "" {
		st := enums.ServerType(*server.Type)
		serverType = st.String()
		serverTypeLabel = st.Label()
	}

	// Handle OperatingSystem field (now *string instead of enum)
	operatingSystem := ""
	operatingSystemLabel := ""
	if server.OperatingSystem != nil && *server.OperatingSystem != "" {
		os := enums.OperatingSystem(*server.OperatingSystem)
		operatingSystem = os.String()
		operatingSystemLabel = os.Label()
	}

	// Handle Username and SSHPort (now pointers)
	username := server.GetUsername()
	sshPort := server.GetSSHPort()

	// Handle timestamps
	createdAt := ""
	if server.CreatedAt != nil {
		createdAt = server.CreatedAt.Format(time.RFC3339)
	}
	updatedAt := ""
	if server.UpdatedAt != nil {
		updatedAt = server.UpdatedAt.Format(time.RFC3339)
	}

	// Progress is int, needs to be converted to *int for response
	var progress *int
	if server.Progress > 0 {
		progress = &server.Progress
	}

	resp := ServerResponse{
		ID:                   server.ID,
		TeamID:               server.TeamID,
		Name:                 server.Name,
		Description:          server.Description,
		Provider:             server.Provider.String(),
		ProviderLabel:        server.Provider.Label(),
		Type:                 serverType,
		TypeLabel:            serverTypeLabel,
		Connected:            server.Connected,
		MonitoringEnabled:    server.MonitoringEnabled,
		CPUCores:             server.CPUCores,
		MemoryInMB:           server.MemoryInMB,
		StorageInGB:          server.StorageInGB,
		OperatingSystem:      operatingSystem,
		OperatingSystemLabel: operatingSystemLabel,
		Status:               server.Status.String(),
		StatusLabel:          server.Status.Label(),
		PublicIPv4:           server.PublicIPv4,
		Username:             username,
		SSHPort:              sshPort,
		AutoUpdate:           server.AutoUpdate,
		Progress:             progress,
		ProgressStep:         server.ProgressStep,
		CreatedAt:            createdAt,
		UpdatedAt:            updatedAt,
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

	if server.Status == enums.ServerStatusNew {
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
func ToServiceResponse(service *models.InstalledService) ServiceResponse {
	// Handle timestamps
	createdAt := ""
	if service.CreatedAt != nil {
		createdAt = service.CreatedAt.Format(time.RFC3339)
	}
	updatedAt := ""
	if service.UpdatedAt != nil {
		updatedAt = service.UpdatedAt.Format(time.RFC3339)
	}

	// Handle Version (now string, convert to *string for response)
	var version *string
	if service.Version != "" {
		version = &service.Version
	}

	resp := ServiceResponse{
		ID:          service.ID,
		ServerID:    service.ServerID,
		Type:        service.Type.String(),
		TypeLabel:   service.Type.Label(),
		Name:        service.Name,
		Version:     version,
		Status:      service.Status.String(),
		StatusLabel: service.Status.Label(),
		IsDefault:   service.IsDefault,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}

	// Handle Software (now string, convert to enum for labels)
	if service.Software != "" {
		sw := enums.Software(service.Software)
		swStr := sw.String()
		resp.Software = &swStr
		label := sw.Label()
		resp.SoftwareLabel = &label
	}

	return resp
}

// FirewallRuleResponse represents the response for a firewall rule
type FirewallRuleResponse struct {
	ID          string  `json:"id"`
	ServerID    string  `json:"server_id"`
	Name        string  `json:"name"`
	Action      string  `json:"action"`
	ActionLabel string  `json:"action_label"`
	Port        *string `json:"port,omitempty"`
	FromIPv4    *string `json:"from_ipv4,omitempty"`
	Mask        *string `json:"mask,omitempty"`
	Note        *string `json:"note,omitempty"`
	IsInstalled bool    `json:"is_installed"`
	IsPending   bool    `json:"is_pending"`
	HasFailed   bool    `json:"has_failed"`
	InstalledAt *string `json:"installed_at,omitempty"`
	UfwRule     string  `json:"ufw_rule"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// ToFirewallRuleResponse converts a FirewallRule model to a FirewallRuleResponse DTO
func ToFirewallRuleResponse(rule *models.FirewallRule) FirewallRuleResponse {
	createdAt := ""
	if rule.CreatedAt != nil {
		createdAt = rule.CreatedAt.Format(time.RFC3339)
	}
	updatedAt := ""
	if rule.UpdatedAt != nil {
		updatedAt = rule.UpdatedAt.Format(time.RFC3339)
	}

	// Port is string, convert to *string for response
	var port *string
	if rule.Port != "" {
		port = &rule.Port
	}

	resp := FirewallRuleResponse{
		ID:          rule.ID,
		ServerID:    rule.ServerID,
		Name:        rule.Name,
		Action:      rule.Action.String(),
		ActionLabel: rule.Action.Label(),
		Port:        port,
		FromIPv4:    rule.FromIPv4,
		Mask:        rule.Mask,
		Note:        rule.Note,
		IsInstalled: rule.IsInstalled(),
		IsPending:   rule.IsPending(),
		HasFailed:   rule.HasFailed(),
		UfwRule:     rule.FormatAsUfwRule(),
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
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
func ToCronResponse(cron *models.Cron) CronResponse {
	createdAt := ""
	if cron.CreatedAt != nil {
		createdAt = cron.CreatedAt.Format(time.RFC3339)
	}
	updatedAt := ""
	if cron.UpdatedAt != nil {
		updatedAt = cron.UpdatedAt.Format(time.RFC3339)
	}

	// Frequency is string, convert to *string for response
	var frequency *string
	if cron.Frequency != "" {
		frequency = &cron.Frequency
	}

	resp := CronResponse{
		ID:          cron.ID,
		ServerID:    cron.ServerID,
		SiteID:      cron.SiteID,
		User:        cron.User,
		Expression:  cron.Expression,
		Command:     cron.Command.String(),
		Frequency:   frequency,
		Hidden:      cron.Hidden,
		IsInstalled: cron.IsInstalled(),
		Path:        cron.Path(),
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
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
func ToDaemonResponse(daemon *models.Daemon) DaemonResponse {
	createdAt := ""
	if daemon.CreatedAt != nil {
		createdAt = daemon.CreatedAt.Format(time.RFC3339)
	}
	updatedAt := ""
	if daemon.UpdatedAt != nil {
		updatedAt = daemon.UpdatedAt.Format(time.RFC3339)
	}

	// StopSignal is string, convert to *string for response
	var stopSignal *string
	if daemon.StopSignal != "" {
		stopSignal = &daemon.StopSignal
	}

	resp := DaemonResponse{
		ID:              daemon.ID,
		ServerID:        daemon.ServerID,
		User:            daemon.User,
		Directory:       daemon.Directory,
		Command:         daemon.Command,
		Processes:       daemon.Processes,
		StopWaitSeconds: daemon.StopWaitSeconds,
		StopSignal:      stopSignal,
		IsInstalled:     daemon.IsInstalled(),
		Running:         daemon.Running,
		Path:            daemon.Path(),
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
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
func ToSshKeyResponse(key *models.SshKey) SshKeyResponse {
	createdAt := ""
	if key.CreatedAt != nil {
		createdAt = key.CreatedAt.Format(time.RFC3339)
	}
	updatedAt := ""
	if key.UpdatedAt != nil {
		updatedAt = key.UpdatedAt.Format(time.RFC3339)
	}

	return SshKeyResponse{
		ID:          key.ID,
		Name:        key.Name,
		Fingerprint: key.GetFingerprint(),
		Description: key.Description,
		IsGlobal:    key.IsGlobal,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
}

// TaskResponse represents the response for a task
type TaskResponse struct {
	ID        string  `json:"id"`
	ServerID  string  `json:"server_id"`
	Type      string  `json:"type"`
	Status    string  `json:"status"`
	Name      string  `json:"name"`
	User      string  `json:"user"`
	Output    *string `json:"output,omitempty"`
	ExitCode  *int    `json:"exit_code,omitempty"`
	CreatedAt string  `json:"created_at"`
}

// ToTaskResponse converts a Task model to a TaskResponse DTO
func ToTaskResponse(task *models.Task) TaskResponse {
	var output *string
	if !task.Output.IsEmpty() {
		s := task.Output.String()
		output = &s
	}

	createdAt := ""
	if task.CreatedAt != nil {
		createdAt = task.CreatedAt.Format(time.RFC3339)
	}

	return TaskResponse{
		ID:        task.ID,
		ServerID:  task.ServerID,
		Type:      task.Type,
		Status:    task.Status,
		Name:      task.Name,
		User:      task.User,
		Output:    output,
		ExitCode:  task.ExitCode,
		CreatedAt: createdAt,
	}
}

// MetricResponse represents the response for server metrics
type MetricResponse struct {
	ID                 uint64  `json:"id"`
	ServerID           string  `json:"server_id"`
	Load               float64 `json:"load"`
	MemoryTotal        float64 `json:"memory_total"`
	MemoryUsed         float64 `json:"memory_used"`
	MemoryFree         float64 `json:"memory_free"`
	MemoryUsagePercent float64 `json:"memory_usage_percent"`
	DiskTotal          float64 `json:"disk_total"`
	DiskUsed           float64 `json:"disk_used"`
	DiskFree           float64 `json:"disk_free"`
	DiskUsagePercent   float64 `json:"disk_usage_percent"`
	CreatedAt          string  `json:"created_at"`
}

// ToMetricResponse converts a Metric model to a MetricResponse DTO
func ToMetricResponse(metric *models.Metric) MetricResponse {
	createdAt := ""
	if metric.CreatedAt != nil {
		createdAt = metric.CreatedAt.Format(time.RFC3339)
	}

	return MetricResponse{
		ID:                 metric.ID,
		ServerID:           metric.ServerID,
		Load:               metric.Load,
		MemoryTotal:        metric.MemoryTotal,
		MemoryUsed:         metric.MemoryUsed,
		MemoryFree:         metric.MemoryFree,
		MemoryUsagePercent: metric.MemoryUsagePercent(),
		DiskTotal:          metric.DiskTotal,
		DiskUsed:           metric.DiskUsed,
		DiskFree:           metric.DiskFree,
		DiskUsagePercent:   metric.DiskUsagePercent(),
		CreatedAt:          createdAt,
	}
}

// AvailableSoftwareResponse represents available software for installation
type AvailableSoftwareResponse struct {
	Group      string                    `json:"group"`
	Label      string                    `json:"label"`
	Type       string                    `json:"type"`
	ImagePath  string                    `json:"image_path"`
	Installed  bool                      `json:"installed"`
	Status     *string                   `json:"status,omitempty"`
	Versions   []SoftwareVersionResponse `json:"versions"`
	HasStart   bool                      `json:"has_start"`
	HasStop    bool                      `json:"has_stop"`
	HasRestart bool                      `json:"has_restart"`
	HasRemove  bool                      `json:"has_remove"`
	HasStatus  bool                      `json:"has_status"`
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
	providers := enums.AllServerProviders()
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
	types := enums.AllServerTypes()
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
	oses := enums.AllOperatingSystems()
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
	actions := enums.AllRuleActions()
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
	Server         ServerResponse         `json:"server"`
	Services       []ServiceResponse      `json:"services"`
	FirewallRules  []FirewallRuleResponse `json:"firewall_rules"`
	Crons          []CronResponse         `json:"crons"`
	Daemons        []DaemonResponse       `json:"daemons"`
	SshKeys        []SshKeyResponse       `json:"ssh_keys"`
	LatestTask     *TaskResponse          `json:"latest_task,omitempty"`
	LatestMetric   *MetricResponse        `json:"latest_metric,omitempty"`
	RuleActions    []ProviderResponse     `json:"rule_actions"`
	HasLaunchAgent bool                   `json:"has_launch_agent"`
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
func GetServerIndexPageData(servers []models.Server, total int64) ServerIndexPageData {
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

// CreateServerOptionsResponse represents the response for server creation options
type CreateServerOptionsResponse struct {
	PhpVersions      map[string]string                      `json:"phpVersions"`
	DatabaseTypes    map[string]string                      `json:"databaseTypes"`
	ServerTypes      map[string]string                      `json:"serverTypes"`
	OperatingSystems map[string]string                      `json:"operatingSystems"`
	Plans            map[string]serverconfig.ProviderConfig `json:"plans"`
	CanCreateServer  bool                                   `json:"canCreateServer"`
}

// GetCreateServerOptions builds the create server options response
func GetCreateServerOptions() CreateServerOptionsResponse {
	return CreateServerOptionsResponse{
		PhpVersions:      serverconfig.GetPhpVersions(),
		DatabaseTypes:    serverconfig.GetDatabaseTypes(),
		ServerTypes:      serverconfig.GetServerTypes(),
		OperatingSystems: serverconfig.GetOperatingSystems(),
		Plans:            serverconfig.GetProviderConfigs(),
		CanCreateServer:  true,
	}
}

// ServerProviderResponse represents a connected server provider
type ServerProviderResponse struct {
	ID        string `json:"id"`
	Profile   string `json:"profile"`
	Provider  string `json:"provider"`
	Connected bool   `json:"connected"`
	CreatedAt string `json:"created_at"`
}

// PhpVersionResponse represents a PHP version with installation status
type PhpVersionResponse struct {
	Key         string           `json:"key"`
	DisplayName string           `json:"display_name"`
	Version     string           `json:"version"`
	IsInstalled bool             `json:"is_installed"`
	IsDefault   bool             `json:"is_default"`
	Details     *ServiceResponse `json:"details,omitempty"`
}

// OpcacheStatusResponse represents the OPcache status
type OpcacheStatusResponse struct {
	Enabled          bool                    `json:"enabled"`
	CacheFull        bool                    `json:"cache_full"`
	RestartPending   bool                    `json:"restart_pending"`
	RestartInProgress bool                   `json:"restart_in_progress"`
	Memory           *OpcacheMemoryStatus    `json:"memory,omitempty"`
	Statistics       *OpcacheStatistics      `json:"statistics,omitempty"`
	InternedStrings  *OpcacheInternedStrings `json:"interned_strings,omitempty"`
	JIT              *OpcacheJITStatus       `json:"jit,omitempty"`
	Scripts          []OpcacheScript         `json:"scripts,omitempty"`
	Directives       map[string]interface{}  `json:"directives,omitempty"`
	Error            string                  `json:"error,omitempty"`
}

// OpcacheMemoryStatus represents OPcache memory usage
type OpcacheMemoryStatus struct {
	UsedMemory       int64   `json:"used_memory"`
	FreeMemory       int64   `json:"free_memory"`
	WastedMemory     int64   `json:"wasted_memory"`
	CurrentWastedPct float64 `json:"current_wasted_percentage"`
}

// OpcacheStatistics represents OPcache statistics
type OpcacheStatistics struct {
	NumCachedScripts   int     `json:"num_cached_scripts"`
	NumCachedKeys      int     `json:"num_cached_keys"`
	MaxCachedKeys      int     `json:"max_cached_keys"`
	Hits               int64   `json:"hits"`
	Misses             int64   `json:"misses"`
	BlacklistMisses    int64   `json:"blacklist_misses"`
	BlacklistMissRatio float64 `json:"blacklist_miss_ratio"`
	OomRestarts        int     `json:"oom_restarts"`
	HashRestarts       int     `json:"hash_restarts"`
	ManualRestarts     int     `json:"manual_restarts"`
	HitRate            float64 `json:"hit_rate"`
}

// OpcacheInternedStrings represents interned strings buffer info
type OpcacheInternedStrings struct {
	BufferSize      int64 `json:"buffer_size"`
	UsedMemory      int64 `json:"used_memory"`
	FreeMemory      int64 `json:"free_memory"`
	NumberOfStrings int   `json:"number_of_strings"`
}

// OpcacheJITStatus represents JIT status
type OpcacheJITStatus struct {
	Enabled    bool   `json:"enabled"`
	On         bool   `json:"on"`
	Kind       int    `json:"kind"`
	OptLevel   int    `json:"opt_level"`
	OptFlags   int    `json:"opt_flags"`
	BufferSize int64  `json:"buffer_size"`
	BufferFree int64  `json:"buffer_free"`
}

// OpcacheScript represents a cached script
type OpcacheScript struct {
	FullPath         string `json:"full_path"`
	Hits             int64  `json:"hits"`
	MemoryConsumption int64  `json:"memory_consumption"`
	LastUsedTimestamp int64  `json:"last_used_timestamp"`
}

// OpcacheDefaultsResponse represents the default OPcache settings
type OpcacheDefaultsResponse struct {
	Enabled               bool   `json:"enabled"`
	EnableCLI             bool   `json:"enable_cli"`
	MemoryConsumption     int    `json:"memory_consumption"`
	InternedStringsBuffer int    `json:"interned_strings_buffer"`
	MaxAcceleratedFiles   int    `json:"max_accelerated_files"`
	ValidateTimestamps    bool   `json:"validate_timestamps"`
	RevalidateFreq        int    `json:"revalidate_freq"`
	SaveComments          bool   `json:"save_comments"`
	JITEnabled            bool   `json:"jit_enabled"`
	JITBufferSize         string `json:"jit_buffer_size"`
	JITMode               string `json:"jit_mode"`
}

// GetDefaultOpcacheSettings returns the default OPcache settings
func GetDefaultOpcacheSettings() OpcacheDefaultsResponse {
	return OpcacheDefaultsResponse{
		Enabled:               true,
		EnableCLI:             false,
		MemoryConsumption:     128,
		InternedStringsBuffer: 16,
		MaxAcceleratedFiles:   10000,
		ValidateTimestamps:    true,
		RevalidateFreq:        2,
		SaveComments:          true,
		JITEnabled:            false,
		JITBufferSize:         "100M",
		JITMode:               "tracing",
	}
}

// ComposerAuthResponse represents the Composer auth.json configuration
type ComposerAuthResponse struct {
	Path     string      `json:"path"`
	Contents interface{} `json:"contents"`
}

// ToServerProviderResponse converts a ServerProvider model to response
func ToServerProviderResponse(sp *models.ServerProvider) ServerProviderResponse {
	createdAt := ""
	if sp.CreatedAt != nil {
		createdAt = sp.CreatedAt.Format(time.RFC3339)
	}

	profile := ""
	if sp.Profile != nil {
		profile = *sp.Profile
	}

	return ServerProviderResponse{
		ID:        sp.ID,
		Profile:   profile,
		Provider:  sp.Provider.String(),
		Connected: sp.Connected,
		CreatedAt: createdAt,
	}
}
