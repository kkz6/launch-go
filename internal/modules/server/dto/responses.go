package dto

import (
	"regexp"
	"strings"
	"time"

	serverconfig "github.com/kkz6/launch-go/internal/modules/server/config"
	"github.com/kkz6/launch-go/internal/modules/server/formatter"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	pkgdto "github.com/kkz6/launch-go/internal/pkg/dto"
)

// DatabaseResponse represents the response for a database
type DatabaseResponse struct {
	Name      string `json:"name"`
	CreatedAt string `json:"created_at,omitempty"`
}

// ServerUsersResponse represents the users available for SSH connections
type ServerUsersResponse struct {
	Root  string `json:"root"`
	Local string `json:"local"`
}

// ServerResponse represents the response for a server
type ServerResponse struct {
	ID                   string  `json:"id"`
	TeamID               string  `json:"team_id"`
	Name                 string  `json:"name"`
	Description          *string `json:"description,omitempty"`
	Provider             string  `json:"provider"`
	ProviderLabel        string  `json:"provider_label"`
	Type                 string  `json:"type"`
	TypeLabel            string  `json:"type_label"`
	Connected            bool    `json:"connected"`
	MonitoringEnabled    bool    `json:"monitoring_enabled"`
	CPUCores             *int    `json:"cpu_cores,omitempty"`
	MemoryInMB           *int    `json:"memory_in_mb,omitempty"`
	StorageInGB          *int    `json:"storage_in_gb,omitempty"`
	OperatingSystem      string  `json:"operating_system"`
	OperatingSystemLabel string  `json:"operating_system_label"`
	// Detected* mirror the server's runtime-detected facts (populated
	// by the detect_os provision step from /etc/os-release + uname).
	// Distinct from OperatingSystem above (which is the user's
	// dashboard choice). UI surfaces both so any mismatch is visible.
	DetectedOSID              *string              `json:"detected_os_id,omitempty"`
	DetectedOSVersion         *string              `json:"detected_os_version,omitempty"`
	DetectedOSVersionCodename *string              `json:"detected_os_version_codename,omitempty"`
	DetectedArch              *string              `json:"detected_arch,omitempty"`
	DetectedKernel            *string              `json:"detected_kernel,omitempty"`
	DetectedAt                *string              `json:"detected_at,omitempty"`
	Status                    string               `json:"status"`
	StatusLabel               string               `json:"status_label"`
	PublicIPv4                *string              `json:"public_ipv4,omitempty"`
	Username                  string               `json:"username"`
	Users                     *ServerUsersResponse `json:"users,omitempty"`
	SSHPort                   int                  `json:"ssh_port"`
	AutoUpdate                bool                 `json:"auto_update"`
	Progress                  *int                 `json:"progress,omitempty"`
	ProgressStep              *string              `json:"progress_step,omitempty"`
	ProvisionedAt             *string              `json:"provisioned_at,omitempty"`
	LastConnectivityCheck     *string              `json:"last_connectivity_check,omitempty"`
	ArchivedAt                *string              `json:"archived_at,omitempty"`
	CreatedAt                 string               `json:"created_at"`
	UpdatedAt                 string               `json:"updated_at"`
	ProvisionCommand          string               `json:"provision_command,omitempty"`
	Features                  []string             `json:"features,omitempty"`
	SitesCount                int                  `json:"sites_count,omitempty"`
	ServicesCount             int                  `json:"services_count,omitempty"`
	UpstreamsCount            int                  `json:"upstreams_count,omitempty"`
	// ProjectsCount is the live docker_projects count on this server.
	// Populated from server_repository.go via a SELECT subquery; always
	// returned (0 for PHP servers) so the frontend can disable the
	// Delete button on docker servers that still have projects.
	ProjectsCount int `json:"projects_count"`
	// WorkloadsCount sums live docker_applications + docker_composes
	// + docker_databases. Used by the Servers list card to show
	// "X workloads" on docker servers (where SitesCount is always 0
	// since the sites table is Laravel-style PHP only). 0 on
	// non-docker servers.
	WorkloadsCount int `json:"workloads_count"`
}

// ToServerResponse converts a Server model to a ServerResponse DTO
func ToServerResponse(server *models.Server) ServerResponse {
	// Handle Type field (now *string instead of enum)
	serverType := ""
	serverTypeLabel := ""
	if server.Type != nil && *server.Type != "" {
		st := types.ServerType(*server.Type)
		serverType = st.String()
		serverTypeLabel = st.Label()
	}

	// Handle OperatingSystem field (now *string instead of enum)
	operatingSystem := ""
	operatingSystemLabel := ""
	if server.OperatingSystem != nil && *server.OperatingSystem != "" {
		os := types.OperatingSystem(*server.OperatingSystem)
		operatingSystem = os.String()
		operatingSystemLabel = os.Label()
	}

	// Handle Username and SSHPort (now pointers)
	username := server.GetUsername()
	sshPort := server.GetSSHPort()

	// Handle timestamps using pkg/dto helpers
	createdAt := pkgdto.FormatTimeOrEmpty(server.CreatedAt)
	updatedAt := pkgdto.FormatTimeOrEmpty(server.UpdatedAt)

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
		Users: &ServerUsersResponse{
			Root:  server.RootUsername(),
			Local: username,
		},
		SSHPort:        sshPort,
		AutoUpdate:     server.AutoUpdate,
		Progress:       progress,
		ProgressStep:   server.ProgressStep,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
		SitesCount:     int(server.SitesCount),
		ServicesCount:  len(server.Services),
		UpstreamsCount: int(server.UpstreamsCount),
		ProjectsCount:  int(server.ProjectsCount),
		WorkloadsCount: int(server.WorkloadsCount),
	}

	resp.ProvisionedAt = pkgdto.FormatTime(server.ProvisionedAt)
	resp.LastConnectivityCheck = pkgdto.FormatTime(server.LastConnectivityCheck)
	resp.ArchivedAt = pkgdto.FormatTime(server.ArchivedAt)
	// Detected* fields pass through as nullable pointers so the frontend
	// can render "Detected: <…>" only when the detect_os step has
	// actually run. Legacy servers (provisioned before the step
	// existed) come back with nil and the UI hides the row.
	resp.DetectedOSID = server.DetectedOSID
	resp.DetectedOSVersion = server.DetectedOSVersion
	resp.DetectedOSVersionCodename = server.DetectedOSVersionCodename
	resp.DetectedArch = server.DetectedArch
	resp.DetectedKernel = server.DetectedKernel
	resp.DetectedAt = pkgdto.FormatTime(server.DetectedAt)

	// Include provision command for custom servers that need manual provisioning
	// (status: new, starting, or failed - before successful provisioning)
	if server.Provider == types.ProviderCustom && server.ProvisionedAt == nil {
		resp.ProvisionCommand = server.GetProvisionCommand()
	}

	features := server.GetFeatures()
	resp.Features = pkgdto.MapSlice(features, func(f *types.ServerFeature) string { return f.String() })

	return resp
}

// ServerListResponse represents a list of servers
type ServerListResponse struct {
	Servers []ServerResponse `json:"servers"`
	Total   int64            `json:"total"`
}

// ServiceResponse represents the response for a service
type ServiceResponse struct {
	ID              string              `json:"id"`
	ServerID        string              `json:"server_id"`
	Type            string              `json:"type"`
	TypeLabel       string              `json:"type_label"`
	Name            string              `json:"name"`
	Version         *string             `json:"version,omitempty"`
	Status          string              `json:"status"`
	StatusLabel     string              `json:"status_label"`
	IsDefault       bool                `json:"is_default"`
	DefaultPending  bool                `json:"default_change_pending"`
	Software        *string             `json:"software,omitempty"`
	SoftwareLabel   *string             `json:"software_label,omitempty"`
	LastStatusCheck *string             `json:"last_status_check,omitempty"`
	StatusDetails   map[string]any      `json:"status_details,omitempty"`
	StatusOutput    *string             `json:"status_output,omitempty"`
	TaskID          *string             `json:"task_id,omitempty"`
	PatchStatus     *string             `json:"patch_status,omitempty"`
	PatchError      *string             `json:"patch_error,omitempty"`
	Extensions      []ExtensionResponse `json:"extensions,omitempty"`
	Opcache         map[string]any      `json:"opcache,omitempty"`
	CreatedAt       string              `json:"created_at"`
	UpdatedAt       string              `json:"updated_at"`
}

// ExtensionResponse represents a PHP extension with status
type ExtensionResponse struct {
	Value       string  `json:"value"`
	Label       string  `json:"label"`
	Description string  `json:"description,omitempty"`
	Status      *string `json:"status,omitempty"`
	IsInstalled bool    `json:"is_installed"`
	IsPending   bool    `json:"is_pending"`
}

// ToServiceResponse converts an InstalledService model to a ServiceResponse DTO
func ToServiceResponse(service *models.InstalledService) ServiceResponse {
	createdAt := pkgdto.FormatTimeOrEmpty(service.CreatedAt)
	updatedAt := pkgdto.FormatTimeOrEmpty(service.UpdatedAt)

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
		TaskID:      service.TaskID,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}

	// Handle Software (now string, convert to enum for labels)
	if service.Software != "" {
		sw := types.Software(service.Software)
		swStr := sw.String()
		resp.Software = &swStr
		label := sw.Label()
		resp.SoftwareLabel = &label
	}

	// Extract status details from TypeData
	if service.TypeData != nil {
		if lastStatusCheck, ok := service.TypeData["last_status_check"].(string); ok {
			resp.LastStatusCheck = &lastStatusCheck
		}
		if statusDetails, ok := service.TypeData["status_details"].(map[string]any); ok {
			resp.StatusDetails = statusDetails
		}
		if statusOutput, ok := service.TypeData["status_output"].(string); ok {
			resp.StatusOutput = &statusOutput
		}
		if patchStatus, ok := service.TypeData["patch_status"].(string); ok {
			resp.PatchStatus = &patchStatus
		}
		if patchError, ok := service.TypeData["patch_error"].(string); ok {
			resp.PatchError = &patchError
		}
		// Extract OPcache settings (PHP only)
		if opcache, ok := service.TypeData["opcache"].(map[string]any); ok {
			resp.Opcache = opcache
		}
	}

	// For PHP services, always include available extensions
	if service.Type == types.ServiceTypePhp {
		var installedExtensions map[string]any
		if service.TypeData != nil {
			installedExtensions, _ = service.TypeData["extensions"].(map[string]any)
		}
		resp.Extensions = convertExtensionsToResponse(installedExtensions)
	}

	return resp
}

// convertExtensionsToResponse converts extensions map to response slice
// It includes all available extensions, marking installed ones appropriately
func convertExtensionsToResponse(installedExtensions map[string]any) []ExtensionResponse {
	// Get all available extensions
	availableExtensions := serverconfig.GetAvailablePhpExtensions()

	// Create a map of installed extensions for quick lookup
	// Extensions can be stored as either a string status or a map with "status" key
	installedMap := make(map[string]string)
	for name, value := range installedExtensions {
		var status string
		switch v := value.(type) {
		case string:
			status = v
		case map[string]any:
			if s, ok := v["status"].(string); ok {
				status = s
			}
		}
		if status != "" {
			installedMap[name] = status
		}
	}

	// Build the response with all available extensions
	result := make([]ExtensionResponse, 0, len(availableExtensions))
	for _, available := range availableExtensions {
		ext := ExtensionResponse{
			Value:       available.Value,
			Label:       available.Label,
			Description: available.Description,
			IsInstalled: false,
			IsPending:   false,
		}

		// Check if this extension is installed
		if status, exists := installedMap[available.Value]; exists {
			ext.IsInstalled = status == "installed"
			ext.IsPending = status == "installing" || status == "removing"
			if status != "" {
				ext.Status = &status
			}
			// Remove from map so we can handle any extra installed extensions not in our list
			delete(installedMap, available.Value)
		}

		result = append(result, ext)
	}

	// Add any installed extensions that weren't in our available list
	for name, status := range installedMap {
		ext := ExtensionResponse{
			Value:       name,
			Label:       serverconfig.GetPhpExtensionLabel(name),
			IsInstalled: status == "installed",
			IsPending:   status == "installing" || status == "removing",
		}
		if status != "" {
			ext.Status = &status
		}
		result = append(result, ext)
	}

	return result
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
	// Port is string, convert to *string for response
	var port *string
	if rule.Port != "" {
		port = &rule.Port
	}

	return FirewallRuleResponse{
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
		InstalledAt: pkgdto.FormatTime(rule.InstalledAt),
		UfwRule:     formatter.UfwRule(rule),
		CreatedAt:   pkgdto.FormatTimeOrEmpty(rule.CreatedAt),
		UpdatedAt:   pkgdto.FormatTimeOrEmpty(rule.UpdatedAt),
	}
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
	// Frequency is string, convert to *string for response
	var frequency *string
	if cron.Frequency != "" {
		frequency = &cron.Frequency
	}

	return CronResponse{
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
		InstalledAt: pkgdto.FormatTime(cron.InstalledAt),
		CreatedAt:   pkgdto.FormatTimeOrEmpty(cron.CreatedAt),
		UpdatedAt:   pkgdto.FormatTimeOrEmpty(cron.UpdatedAt),
	}
}

// DaemonResponse represents the response for a daemon
type DaemonResponse struct {
	ID              string         `json:"id"`
	ServerID        string         `json:"server_id"`
	User            string         `json:"user"`
	Directory       *string        `json:"directory,omitempty"`
	Command         string         `json:"command"`
	Processes       int            `json:"processes"`
	StopWaitSeconds int            `json:"stop_wait_seconds"`
	StopSignal      *string        `json:"stop_signal,omitempty"`
	IsInstalled     bool           `json:"is_installed"`
	Running         bool           `json:"running"`
	Info            map[string]any `json:"info,omitempty"`
	Path            string         `json:"path"`
	InstalledAt     *string        `json:"installed_at,omitempty"`
	LastStatusCheck *string        `json:"last_status_check,omitempty"`
	CreatedAt       string         `json:"created_at"`
	UpdatedAt       string         `json:"updated_at"`
}

// ToDaemonResponse converts a Daemon model to a DaemonResponse DTO
func ToDaemonResponse(daemon *models.Daemon) DaemonResponse {
	// StopSignal is string, convert to *string for response
	var stopSignal *string
	if daemon.StopSignal != "" {
		stopSignal = &daemon.StopSignal
	}

	return DaemonResponse{
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
		Info:            daemon.GetInfo(),
		Path:            daemon.Path(),
		InstalledAt:     pkgdto.FormatTime(daemon.InstalledAt),
		LastStatusCheck: pkgdto.FormatTime(daemon.LastStatusCheck),
		CreatedAt:       pkgdto.FormatTimeOrEmpty(daemon.CreatedAt),
		UpdatedAt:       pkgdto.FormatTimeOrEmpty(daemon.UpdatedAt),
	}
}

// SSHKeyResponse represents the response for an SSH key
type SSHKeyResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Fingerprint string  `json:"fingerprint"`
	Description *string `json:"description,omitempty"`
	IsGlobal    bool    `json:"is_global"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// ToSSHKeyResponse converts an SSHKey model to an SSHKeyResponse DTO
func ToSSHKeyResponse(key *models.SSHKey) SSHKeyResponse {
	return SSHKeyResponse{
		ID:          key.ID,
		Name:        key.Name,
		Fingerprint: key.GetFingerprint(),
		Description: key.Description,
		IsGlobal:    key.IsGlobal,
		CreatedAt:   pkgdto.FormatTimeOrEmpty(key.CreatedAt),
		UpdatedAt:   pkgdto.FormatTimeOrEmpty(key.UpdatedAt),
	}
}

// GenerateSSHKeyResponse represents the response for generating an SSH key pair
type GenerateSSHKeyResponse struct {
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
}

// ProvisionStatusStep represents a step in the provisioning process
type ProvisionStatusStep struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"` // "completed", "current", "pending"
}

// ProvisionStatusResponse represents the provision status for a server
type ProvisionStatusResponse struct {
	Steps        []ProvisionStatusStep `json:"steps"`
	CurrentStep  *ProvisionStatusStep  `json:"current_step,omitempty"`
	LatestTask   *TaskResponse         `json:"latest_task,omitempty"`
	Failed       bool                  `json:"failed"`
	ErrorMessage string                `json:"error_message,omitempty"`
}

// BuildProvisionStatus builds the provision status for the UI's progress
// timeline. Two behaviours that bit us in the past:
//   - Step list must match the server's actual provisioning type. A docker
//     server runs ForDockerServer() (base hardening + docker stack); a PHP
//     server runs ForFreshServer(). Hardcoding one type caused docker
//     provisioning to display PHP-stack steps and a spinner glued to the
//     wrong row.
//   - When the server's status is `failed`, no step should be marked
//     `current` — the failure usually happened *before* the next step
//     would have run, so the UI must render a failed banner instead of a
//     spinner that never resolves.
func BuildProvisionStatus(server *models.Server, latestTask *models.Task) ProvisionStatusResponse {
	completedSteps := make(map[string]bool)
	for _, step := range server.CompletedProvisionSteps {
		completedSteps[step] = true
	}

	serverFailed := server.Status == types.ServerStatusFailed

	// Build steps list
	var steps []ProvisionStatusStep
	var currentStep *ProvisionStatusStep

	// First step: connecting to server. Only "current" while the server is
	// actively starting AND hasn't failed.
	connectingStatus := "pending"
	switch {
	case latestTask != nil:
		connectingStatus = "completed"
	case !serverFailed && server.Status == types.ServerStatusStarting:
		connectingStatus = "current"
	}
	connectingStep := ProvisionStatusStep{
		Name:        "connecting_server",
		Description: "Waiting for server to connect",
		Status:      connectingStatus,
	}
	steps = append(steps, connectingStep)
	if connectingStatus == "current" {
		currentStep = &connectingStep
	}

	// Provision steps — choose the right list for the server's type. Docker
	// servers expose the base hardening steps plus the docker-stack steps.
	var provisionSteps []types.ProvisionStep
	if server.Type != nil && *server.Type == string(types.ServerTypeDocker) {
		provisionSteps = types.ForDockerServer()
	} else {
		provisionSteps = types.ForFreshServer()
	}

	foundCurrent := false
	for _, ps := range provisionSteps {
		status := "pending"
		switch {
		case completedSteps[ps.String()]:
			status = "completed"
		case !serverFailed && !foundCurrent && connectingStatus == "completed":
			status = "current"
			foundCurrent = true
		}

		step := ProvisionStatusStep{
			Name:        ps.String(),
			Description: ps.Description(),
			Status:      status,
		}
		steps = append(steps, step)
		if status == "current" {
			currentStep = &step
		}
	}

	// Service installation steps. A service is only allowed to be `current`
	// once the connecting step has completed AND no earlier step is already
	// marked `current`. Without this, the UI shows two spinners at once —
	// "Waiting for server to connect" and "Installing Docker" — because
	// service.Status can be set to `installing` independently of overall
	// progress.
	for _, service := range server.Services {
		status := "pending"
		desc := "Installing " + service.Name
		canBeCurrent := !serverFailed && !foundCurrent && connectingStatus == "completed"
		switch {
		case service.Status.IsActive():
			status = "completed"
			desc = "Installed " + service.Name
		case canBeCurrent && service.Status == types.ServiceStatusInstalling:
			status = "current"
			foundCurrent = true
		case canBeCurrent:
			status = "current"
			foundCurrent = true
		}

		step := ProvisionStatusStep{
			Name:        service.Name,
			Description: desc,
			Status:      status,
		}
		steps = append(steps, step)
		if status == "current" {
			currentStep = &step
		}
	}

	resp := ProvisionStatusResponse{
		Steps:       steps,
		CurrentStep: currentStep,
		Failed:      serverFailed,
	}
	if serverFailed {
		resp.ErrorMessage = provisionErrorMessage(server, latestTask)
	}

	if latestTask != nil {
		taskResp := ToTaskResponse(latestTask)
		resp.LatestTask = &taskResp
	}

	return resp
}

// provisionErrorMessage picks the most informative error string we have for
// a failed server. Order of preference:
//  1. server.provision_error — friendly, classified message set by the failing
//     job/task callbacks (see providers.FriendlyError).
//  2. trailing output of the latest failed task, **classified into a
//     short user-friendly sentence** (see classifyTaskFailure). Returning
//     the raw tail of bash output here once leaked our internal
//     `::LAUNCH::status::...` markers and dpkg lock noise straight to the
//     customer-facing failure banner.
//  3. a generic fallback the product team has approved for end-user display.
//
// Never surface raw HTTP bodies / stack traces / shell output from here —
// those belong in the worker logs and Sentry, not the SaaS UI.
func provisionErrorMessage(server *models.Server, latestTask *models.Task) string {
	if server != nil && server.ProvisionError != nil && *server.ProvisionError != "" {
		return *server.ProvisionError
	}
	if latestTask != nil && latestTask.Status == string(types.TaskStatusFailed) {
		if out := latestTask.Output.String(); out != "" {
			return classifyTaskFailure(out)
		}
	}
	return "We couldn't finish provisioning this server. Please try again, or contact support if it keeps happening."
}

// stripLaunchMarkers removes our internal `::LAUNCH::<type>::<value>` lines
// from script output. Without this, the customer-facing failure banner
// echoes things like "::LAUNCH::step_completed::configure_firewall
// ::LAUNCH::progress::15" which looks like a stack trace to a non-technical
// user and exposes our internal protocol.
var launchMarkerLine = regexp.MustCompile(`::LAUNCH::[^\s]+`)

func stripLaunchMarkers(s string) string {
	return strings.TrimSpace(launchMarkerLine.ReplaceAllString(s, ""))
}

// classifyTaskFailure maps raw bash/apt output into a single user-friendly
// sentence. Keep these branches narrow (specific signal in the output) and
// in priority order — first match wins. When in doubt, fall through to the
// generic copy rather than echoing a confusing tail of shell output.
//
// The patterns here are all motivated by actual production failures we've
// seen. Don't add speculative ones — if a class of failure isn't surfacing
// in tickets, leaving it on the generic fallback is fine.
func classifyTaskFailure(rawOutput string) string {
	out := stripLaunchMarkers(rawOutput)
	low := strings.ToLower(out)

	switch {
	case strings.Contains(low, "could not get lock /var/lib/dpkg") ||
		strings.Contains(low, "unable to acquire the dpkg frontend lock"):
		return "Another package manager is still running on the server. This usually clears up within a few minutes — please try again shortly."

	case strings.Contains(low, "unable to locate package") ||
		strings.Contains(low, "has no installation candidate"):
		return "A required package isn't available in the server's repositories. Please contact support so we can investigate."

	case strings.Contains(low, "permission denied") && strings.Contains(low, "sudo"):
		return "We don't have permission to run setup commands on this server. Please verify the SSH user has sudo access without a password prompt."

	case strings.Contains(low, "no space left on device"):
		return "The server ran out of disk space during setup. Please resize it or use a larger plan and try again."

	case strings.Contains(low, "temporary failure resolving") ||
		strings.Contains(low, "could not resolve host"):
		return "The server couldn't reach the internet to download packages. Please check the server's DNS and outbound network access, then retry."

	case strings.Contains(low, "connection timed out") ||
		strings.Contains(low, "connection refused"):
		return "We couldn't reach the server while installing required software. Please verify the server is online and reachable, then retry."

	case strings.Contains(low, "ssl certificate problem") ||
		strings.Contains(low, "certificate has expired"):
		return "An SSL certificate verification error stopped the installation. This is usually a server-side clock issue — set the server's time correctly and retry."

	case strings.Contains(low, "killed") && strings.Contains(low, "out of memory"):
		return "The server ran out of memory during setup. Please use a plan with more RAM and try again."
	}

	return "Setup didn't complete on this server. Please try again, or contact support if it keeps happening."
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

	createdAt := pkgdto.FormatTimeOrEmpty(task.CreatedAt)

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
		CreatedAt:          pkgdto.FormatTimeOrEmpty(metric.CreatedAt),
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
	providers := types.AllServerProviders()
	return pkgdto.MapSlice(providers, func(p *types.ServerProvider) ProviderResponse {
		return ProviderResponse{
			Value: p.String(),
			Label: p.Label(),
		}
	})
}

// GetAllServerTypes returns all available server types
func GetAllServerTypes() []ProviderResponse {
	serverTypes := types.AllServerTypes()
	return pkgdto.MapSlice(serverTypes, func(t *types.ServerType) ProviderResponse {
		return ProviderResponse{
			Value: t.String(),
			Label: t.Label(),
		}
	})
}

// GetAllOperatingSystems returns all available operating systems
func GetAllOperatingSystems() []ProviderResponse {
	oses := types.AllOperatingSystems()
	return pkgdto.MapSlice(oses, func(os *types.OperatingSystem) ProviderResponse {
		return ProviderResponse{
			Value: os.String(),
			Label: os.Label(),
		}
	})
}

// GetAllRuleActions returns all available firewall rule actions
func GetAllRuleActions() []ProviderResponse {
	actions := types.AllRuleActions()
	return pkgdto.MapSlice(actions, func(a *types.RuleAction) ProviderResponse {
		return ProviderResponse{
			Value: a.String(),
			Label: a.Label(),
		}
	})
}

// ServerShowPageData represents the data for the server show page
type ServerShowPageData struct {
	Server         ServerResponse         `json:"server"`
	Services       []ServiceResponse      `json:"services"`
	FirewallRules  []FirewallRuleResponse `json:"firewall_rules"`
	Crons          []CronResponse         `json:"crons"`
	Daemons        []DaemonResponse       `json:"daemons"`
	SSHKeys        []SSHKeyResponse       `json:"ssh_keys"`
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
	return ServerIndexPageData{
		Servers:          pkgdto.TransformSlice(servers, ToServerResponse),
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

// InstalledPhpVersionResponse represents a simple installed PHP version for dropdowns
type InstalledPhpVersionResponse struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	DisplayName string `json:"display_name"`
	Version     string `json:"version"`
	IsDefault   bool   `json:"is_default"`
}

// OpcacheStatusResponse represents the OPcache status
type OpcacheStatusResponse struct {
	Enabled           bool                    `json:"enabled"`
	CacheFull         bool                    `json:"cache_full"`
	RestartPending    bool                    `json:"restart_pending"`
	RestartInProgress bool                    `json:"restart_in_progress"`
	Memory            *OpcacheMemoryStatus    `json:"memory,omitempty"`
	Statistics        *OpcacheStatistics      `json:"statistics,omitempty"`
	InternedStrings   *OpcacheInternedStrings `json:"interned_strings,omitempty"`
	JIT               *OpcacheJITStatus       `json:"jit,omitempty"`
	Scripts           []OpcacheScript         `json:"scripts,omitempty"`
	Directives        map[string]any          `json:"directives,omitempty"`
	Error             string                  `json:"error,omitempty"`
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
	Enabled    bool  `json:"enabled"`
	On         bool  `json:"on"`
	Kind       int   `json:"kind"`
	OptLevel   int   `json:"opt_level"`
	OptFlags   int   `json:"opt_flags"`
	BufferSize int64 `json:"buffer_size"`
	BufferFree int64 `json:"buffer_free"`
}

// OpcacheScript represents a cached script
type OpcacheScript struct {
	FullPath          string `json:"full_path"`
	Hits              int64  `json:"hits"`
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

// LoadBalancerUpstreamResponse represents the response for a load balancer upstream
type LoadBalancerUpstreamResponse struct {
	ID                  string                        `json:"id"`
	ServerID            string                        `json:"server_id"`
	TeamID              string                        `json:"team_id"`
	Name                string                        `json:"name"`
	Address             string                        `json:"address"`
	Port                int                           `json:"port"`
	TLSSetting          string                        `json:"tls_setting"`
	LBPolicy            string                        `json:"lb_policy"`
	LBPolicyLabel       string                        `json:"lb_policy_label"`
	HealthCheckPath     string                        `json:"health_check_path"`
	HealthCheckInterval string                        `json:"health_check_interval"`
	HealthCheckTimeout  string                        `json:"health_check_timeout"`
	InstalledAt         *string                       `json:"installed_at,omitempty"`
	CreatedAt           string                        `json:"created_at"`
	UpdatedAt           string                        `json:"updated_at"`
	Backends            []LoadBalancerBackendResponse `json:"backends,omitempty"`
}

// LoadBalancerBackendResponse represents the response for a load balancer backend
type LoadBalancerBackendResponse struct {
	ID                string  `json:"id"`
	UpstreamID        string  `json:"upstream_id"`
	SiteID            string  `json:"site_id"`
	ServerID          string  `json:"server_id"`
	Port              int     `json:"port"`
	IsDown            bool    `json:"is_down"`
	HealthStatus      string  `json:"health_status"`
	LastHealthCheckAt *string `json:"last_health_check_at,omitempty"`
	CreatedAt         string  `json:"created_at"`
	UpdatedAt         string  `json:"updated_at"`
}

// CheckDomainResponse represents the response for domain conflict detection
type CheckDomainResponse struct {
	Address string            `json:"address"`
	Exists  bool              `json:"exists"`
	Sites   []DomainCheckSite `json:"sites,omitempty"`
	Warning string            `json:"warning,omitempty"`
}

// DomainCheckSite represents a site found during domain conflict check
type DomainCheckSite struct {
	ID       string `json:"id"`
	ServerID string `json:"server_id"`
	Address  string `json:"address"`
	Type     string `json:"type"`
}

// ToLoadBalancerUpstreamResponse converts a LoadBalancerUpstream model to response
func ToLoadBalancerUpstreamResponse(upstream *models.LoadBalancerUpstream) LoadBalancerUpstreamResponse {
	resp := LoadBalancerUpstreamResponse{
		ID:                  upstream.ID,
		ServerID:            upstream.ServerID,
		TeamID:              upstream.TeamID,
		Name:                upstream.Name,
		Address:             upstream.Address,
		Port:                upstream.Port,
		TLSSetting:          upstream.TLSSetting,
		LBPolicy:            upstream.LBPolicy.String(),
		LBPolicyLabel:       upstream.LBPolicyLabel(),
		HealthCheckPath:     upstream.HealthCheckPath,
		HealthCheckInterval: upstream.HealthCheckInterval,
		HealthCheckTimeout:  upstream.HealthCheckTimeout,
		InstalledAt:         pkgdto.FormatTime(upstream.InstalledAt),
		CreatedAt:           pkgdto.FormatTimeOrEmpty(upstream.CreatedAt),
		UpdatedAt:           pkgdto.FormatTimeOrEmpty(upstream.UpdatedAt),
	}

	if upstream.Backends != nil {
		resp.Backends = pkgdto.TransformSlice(upstream.Backends, ToLoadBalancerBackendResponse)
	}

	return resp
}

// ToLoadBalancerBackendResponse converts a LoadBalancerBackend model to response
func ToLoadBalancerBackendResponse(backend *models.LoadBalancerBackend) LoadBalancerBackendResponse {
	return LoadBalancerBackendResponse{
		ID:                backend.ID,
		UpstreamID:        backend.UpstreamID,
		SiteID:            backend.SiteID,
		ServerID:          backend.ServerID,
		Port:              backend.Port,
		IsDown:            backend.IsDown,
		HealthStatus:      string(backend.HealthStatus),
		LastHealthCheckAt: pkgdto.FormatTime(backend.LastHealthCheckAt),
		CreatedAt:         pkgdto.FormatTimeOrEmpty(backend.CreatedAt),
		UpdatedAt:         pkgdto.FormatTimeOrEmpty(backend.UpdatedAt),
	}
}

// UpstreamHealthResponse represents the health status of all backends in an upstream
type UpstreamHealthResponse struct {
	UpstreamID      string                `json:"upstream_id"`
	Address         string                `json:"address"`
	TotalBackends   int                   `json:"total_backends"`
	HealthyBackends int                   `json:"healthy_backends"`
	Backends        []BackendHealthStatus `json:"backends"`
}

// BackendHealthStatus represents the health status of a single backend
type BackendHealthStatus struct {
	BackendID         string     `json:"backend_id"`
	ServerID          string     `json:"server_id"`
	SiteID            string     `json:"site_id"`
	Port              int        `json:"port"`
	IsDown            bool       `json:"is_down"`
	HealthStatus      string     `json:"health_status"`
	LastHealthCheckAt *time.Time `json:"last_health_check_at,omitempty"`
}

// AgentVersionResponse tells the UI whether a server's Launch Agent has
// an update available, driving the update banner + "Update" action.
type AgentVersionResponse struct {
	// ServiceID is the installed launch_agent service row id (empty if
	// the agent isn't installed on this server).
	ServiceID string `json:"service_id"`
	// Installed is the version the server currently reports (no leading
	// "v"); may be empty or a legacy placeholder ("latest"/"master").
	Installed string `json:"installed"`
	// Latest is the newest published agent version (no leading "v"), or
	// empty if the GitHub lookup is unavailable.
	Latest string `json:"latest"`
	// UpdateAvailable is true when the agent is installed and a newer
	// (or unknown-vs-known) version is available.
	UpdateAvailable bool `json:"update_available"`
}

// ToServerProviderResponse converts a ServerProvider model to response
func ToServerProviderResponse(sp *models.ServerProvider) ServerProviderResponse {
	profile := ""
	if sp.Profile != nil {
		profile = *sp.Profile
	}

	return ServerProviderResponse{
		ID:        sp.ID,
		Profile:   profile,
		Provider:  sp.Provider.String(),
		Connected: sp.Connected,
		CreatedAt: pkgdto.FormatTimeOrEmpty(sp.CreatedAt),
	}
}
