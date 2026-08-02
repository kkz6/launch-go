package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/pkg/signedurl"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// Compile-time check that Server implements ServerConnection
var _ taskrunner.ServerConnection = (*Server)(nil)

// Server represents a managed server
type Server struct {
	basemodels.BaseModel
	basemodels.TeamScoped
	basemodels.UserScoped
	ServerProviderID  *string                `gorm:"column:server_provider_id;type:char(26);index" json:"server_provider_id,omitempty"`
	Name              string                 `gorm:"type:varchar(255);not null;index" json:"name"`
	Description       *string                `gorm:"type:varchar(255)" json:"description,omitempty"`
	Provider          types.ServerProvider   `gorm:"type:varchar(255);not null" json:"provider"`
	ProviderData      dbtype.JSONMap         `gorm:"type:json" json:"-"`
	Type              *string                `gorm:"type:varchar(255)" json:"type,omitempty"`
	Connected         bool                   `gorm:"type:tinyint(1);not null;default:0" json:"connected"`
	LaunchToken       string                 `gorm:"type:varchar(32);not null" json:"-"`
	MonitoringEnabled bool                   `gorm:"column:monitoring_enabled;type:tinyint(1);not null;default:0" json:"monitoring_enabled"`
	CPUCores          *int                   `gorm:"column:cpu_cores;type:int" json:"cpu_cores,omitempty"`
	MemoryInMB        *int                   `gorm:"column:memory_in_mb;type:int" json:"memory_in_mb,omitempty"`
	StorageInGB       *int                   `gorm:"column:storage_in_gb;type:int" json:"storage_in_gb,omitempty"`
	OperatingSystem   *string                `gorm:"column:operating_system;type:varchar(255)" json:"operating_system,omitempty"`
	Status            types.ServerStatus     `gorm:"type:varchar(255);not null" json:"status"`
	PublicIPv4        *string                `gorm:"column:public_ipv4;type:varchar(255)" json:"public_ipv4,omitempty"`
	PrivateIPv4       *string                `gorm:"column:private_ipv4;type:varchar(255)" json:"-"`
	PublicKey         dbtype.EncryptedString `gorm:"type:longtext" json:"-"`
	PrivateKey        dbtype.EncryptedString `gorm:"type:longtext" json:"-"`
	// HostKey is the SSH host key (base64-encoded wire format) pinned
	// on the first connection. Empty until the first connect — then
	// every subsequent connection compares the live key against this
	// stored value and refuses to authenticate on mismatch. Public
	// data by definition, so this is plain text rather than the
	// EncryptedString that wraps the private key on line 42.
	HostKey                    string                 `gorm:"column:host_key;type:text" json:"-"`
	UserPublicKey              dbtype.EncryptedString `gorm:"column:user_public_key;type:longtext" json:"-"`
	Username                   *string                `gorm:"type:varchar(255)" json:"username,omitempty"`
	Password                   dbtype.EncryptedString `gorm:"type:longtext" json:"-"`
	DatabasePassword           dbtype.EncryptedString `gorm:"column:database_password;type:longtext" json:"-"`
	SSHPort                    *int                   `gorm:"column:ssh_port;type:int" json:"ssh_port,omitempty"`
	WorkingDirectory           *string                `gorm:"column:working_directory;type:varchar(255)" json:"-"`
	CompletedProvisionSteps    dbtype.JSONStringSlice `gorm:"column:completed_provision_steps;type:json" json:"-"`
	ProvisionedAt              *time.Time             `gorm:"column:provisioned_at;type:timestamp null" json:"provisioned_at,omitempty"`
	UninstallationRequestedAt  *time.Time             `gorm:"column:uninstallation_requested_at;type:timestamp null" json:"-"`
	Updates                    bool                   `gorm:"type:tinyint(1);not null;default:0" json:"-"`
	AutoUpdate                 bool                   `gorm:"column:auto_update;type:tinyint(1);not null;default:0" json:"auto_update"`
	AvailableUpdates           *int                   `gorm:"column:available_updates;type:int" json:"-"`
	SecurityUpdates            *int                   `gorm:"column:security_updates;type:int" json:"-"`
	Progress                   int                    `gorm:"type:int;not null;default:0" json:"progress"`
	ProgressStep               *string                `gorm:"column:progress_step;type:varchar(255)" json:"progress_step,omitempty"`
	ProvisionError             *string                `gorm:"column:provision_error;type:text" json:"provision_error,omitempty"`
	PendingDefaultPHPServiceID *string                `gorm:"column:pending_default_php_service_id;type:char(26)" json:"pending_default_php_service_id,omitempty"`
	LastUpdateCheck            *time.Time             `gorm:"column:last_update_check;type:timestamp null" json:"-"`
	LastConnectivityCheck      *time.Time             `gorm:"column:last_connectivity_check;type:timestamp null" json:"last_connectivity_check,omitempty"`
	ArchivedAt                 *time.Time             `gorm:"column:archived_at;type:timestamp null" json:"archived_at,omitempty"`
	// Detected runtime facts about the box, populated by the detect_os
	// provision step from /etc/os-release + uname. Distinct from
	// OperatingSystem above (which is what the user picked in the
	// dropdown) — downstream scripts trust these, the UI surfaces them
	// so any mismatch is visible.
	DetectedOSID              *string    `gorm:"column:detected_os_id;type:varchar(64)" json:"detected_os_id,omitempty"`
	DetectedOSVersion         *string    `gorm:"column:detected_os_version;type:varchar(64)" json:"detected_os_version,omitempty"`
	DetectedOSVersionCodename *string    `gorm:"column:detected_os_version_codename;type:varchar(64)" json:"detected_os_version_codename,omitempty"`
	DetectedArch              *string    `gorm:"column:detected_arch;type:varchar(32)" json:"detected_arch,omitempty"`
	DetectedKernel            *string    `gorm:"column:detected_kernel;type:varchar(128)" json:"detected_kernel,omitempty"`
	DetectedAt                *time.Time `gorm:"column:detected_at;type:timestamptz" json:"detected_at,omitempty"`

	// Relations
	Services      []InstalledService `gorm:"foreignKey:ServerID;references:ID" json:"services,omitempty"`
	FirewallRules []FirewallRule     `gorm:"foreignKey:ServerID;references:ID" json:"firewall_rules,omitempty"`
	Crons         []Cron             `gorm:"foreignKey:ServerID;references:ID" json:"crons,omitempty"`
	Daemons       []Daemon           `gorm:"foreignKey:ServerID;references:ID" json:"daemons,omitempty"`
	SSHKeys       []SSHKey           `gorm:"many2many:server_ssh_keys" json:"ssh_keys,omitempty"`
	Tasks         []Task             `gorm:"foreignKey:ServerID;references:ID" json:"tasks,omitempty"`
	Metrics       []Metric           `gorm:"foreignKey:ServerID;references:ID" json:"metrics,omitempty"`

	// Computed fields (read-only, not stored in DB)
	SitesCount     int64 `gorm:"column:sites_count;->" json:"sites_count"`
	UpstreamsCount int64 `gorm:"-" json:"upstreams_count,omitempty"`
	// ProjectsCount mirrors the SitesCount pattern but counts live
	// docker_projects rows for the server. The server module doesn't
	// import the docker module (docker imports server, not the other
	// way), so the subquery references the table by name. Populated
	// from server_repository.go via Select("(?) as projects_count").
	ProjectsCount int64 `gorm:"column:projects_count;->" json:"projects_count"`
	// WorkloadsCount sums docker_applications + docker_composes +
	// docker_databases for the server — what the Servers list card
	// shows on docker servers since SitesCount is always 0 there
	// (the `sites` table is Laravel-style PHP only). Same cross-
	// module reference pattern as ProjectsCount: docker tables are
	// referenced by name from server_repository.go, never imported
	// (docker depends on server, not the reverse).
	WorkloadsCount int64 `gorm:"column:workloads_count;->" json:"workloads_count"`
}

func (s *Server) BeforeCreate(tx *gorm.DB) error {
	if err := s.BaseModel.BeforeCreate(tx); err != nil {
		return err
	}

	basemodels.SetDefaultStatus(&s.Status, types.ServerStatusNew)
	basemodels.SetDefaultToken(&s.LaunchToken, 16)

	return nil
}

func (Server) TableName() string {
	return "servers"
}

func (s *Server) IsProvisioned() bool {
	return s.ProvisionedAt != nil
}

func (s *Server) IsConnected() bool {
	return s.Connected
}

func (s *Server) IsArchived() bool {
	return s.ArchivedAt != nil
}

func (s *Server) RootUsername() string {
	os := types.OSUbuntu24
	if s.OperatingSystem != nil {
		os = types.OperatingSystem(*s.OperatingSystem)
	}

	return s.Provider.GetDefaultUsername(os)
}

// GetProvisionCommand returns the one-liner the customer pastes into
// their server. Pipes to `sudo bash` (not bare `bash`) because the
// script must run as root for the SSH public key to land in
// /root/.ssh/authorized_keys where Launch's backend looks for it.
// Most cloud images default to a non-root user (ubuntu/admin/ec2-user)
// with passwordless sudo, so this just works for them; if the user is
// already root, sudo is a no-op. See generateAuthorizeKeyScript for
// the matching root-check at the top of the script itself.
func (s *Server) GetProvisionCommand() string {
	return fmt.Sprintf("wget --no-verbose -O - '%s' | sudo bash", s.GetProvisionScriptURL())
}

// GetProvisionScriptURL returns a signed URL for the provision script.
// Lives on a dedicated `/provision/:id` path on the frontend domain — kept
// out of the `/servers/*` namespace so it doesn't collide with the
// authenticated server routes (which are registered as a Fiber group with
// auth middleware applied as a `/servers/*` wildcard).
func (s *Server) GetProvisionScriptURL() string {
	return signedurl.PublicPermanentSign(fmt.Sprintf("/provision/%s", s.ID), nil)
}

func (s *Server) HasFeature(feature types.ServerFeature) bool {
	if s.Type == nil {
		return types.ServerTypePhp.HasFeature(feature)
	}

	return types.ServerType(*s.Type).HasFeature(feature)
}

func (s *Server) GetFeatures() []types.ServerFeature {
	if s.Type == nil {
		return types.ServerTypePhp.GetFeatures()
	}

	return types.ServerType(*s.Type).GetFeatures()
}

func (s *Server) GetProcessManager() types.ProcessManager {
	if s.Type == nil {
		return types.ServerTypePhp.GetProcessManager()
	}

	return types.ServerType(*s.Type).GetProcessManager()
}

func (s *Server) GetUsername() string {
	if s.Username != nil && *s.Username != "" {
		return *s.Username
	}

	return config.ServerDefaults().Username
}

func (s *Server) GetSSHPort() int {
	if s.SSHPort != nil {
		return *s.SSHPort
	}

	return 22
}

// ConnectionAsRoot returns an SSH connection configured to connect as the root user
func (s *Server) ConnectionAsRoot() *taskrunner.Connection {
	host := ""
	if s.PublicIPv4 != nil {
		host = *s.PublicIPv4
	}

	return &taskrunner.Connection{
		Host:       host,
		Port:       s.GetSSHPort(),
		User:       s.RootUsername(),
		PrivateKey: s.PrivateKey.String(),
		ScriptPath: s.GetScriptPath(s.RootUsername()),
		ServerID:   s.ID,
		HostKey:    s.HostKey,
	}
}

// ConnectionAsUser returns an SSH connection configured to connect as the specified user
// If no username is provided, uses the default server username
func (s *Server) ConnectionAsUser(username ...string) *taskrunner.Connection {
	user := s.GetUsername()
	if len(username) > 0 && username[0] != "" {
		user = username[0]
	}

	host := ""
	if s.PublicIPv4 != nil {
		host = *s.PublicIPv4
	}

	return &taskrunner.Connection{
		Host:       host,
		Port:       s.GetSSHPort(),
		User:       user,
		PrivateKey: s.PrivateKey.String(),
		ScriptPath: s.GetScriptPath(user),
		ServerID:   s.ID,
		HostKey:    s.HostKey,
	}
}

// GetScriptPath returns the script/task directory path for a given user
func (s *Server) GetScriptPath(user string) string {
	// Use server's working directory, fall back to config default
	workingDir := config.ServerDefaults().WorkingDirectory
	if s.WorkingDirectory != nil && *s.WorkingDirectory != "" {
		workingDir = *s.WorkingDirectory
	}

	var homeDir string
	switch user {
	case "root":
		homeDir = "/root"
	case "ubuntu":
		homeDir = "/home/ubuntu"
	default:
		homeDir = "/home/" + user
	}

	return homeDir + "/" + workingDir
}

// GetID returns the server's unique identifier
func (s *Server) GetID() string {
	return s.ID
}

// GetTeamID returns the team ID for broadcasting
func (s *Server) GetTeamID() string {
	return s.TeamID
}

// GetIPAddress returns the server's public IP address
func (s *Server) GetIPAddress() string {
	if s.PublicIPv4 != nil {
		return *s.PublicIPv4
	}
	return ""
}

// GetPrivateKey returns the SSH private key
func (s *Server) GetPrivateKey() string {
	return s.PrivateKey.String()
}

// GetSudoPassword returns the password for sudo operations
func (s *Server) GetSudoPassword() string {
	return s.Password.String()
}

// BroadcastName returns the model name for broadcasting
func (s *Server) BroadcastName() string {
	return "server"
}

// BroadcastPayload returns the data to broadcast
func (s *Server) BroadcastPayload() map[string]interface{} {
	return map[string]interface{}{
		"id":     s.ID,
		"name":   s.Name,
		"status": s.Status,
	}
}
