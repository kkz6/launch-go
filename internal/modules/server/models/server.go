package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/config"
	"github.com/kkz6/launch-go/internal/modules/server/enums"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/pkg/signedurl"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// Server represents a managed server
type Server struct {
	basemodels.BaseModel
	ServerProviderID          *string                    `gorm:"column:server_provider_id;type:char(26);index" json:"server_provider_id,omitempty"`
	TeamID                    string                     `gorm:"column:team_id;type:char(26);not null;index" json:"team_id"`
	UserID                    string                     `gorm:"column:user_id;type:char(26);not null;index" json:"user_id"`
	Name                      string                     `gorm:"type:varchar(255);not null;index" json:"name"`
	Description               *string                    `gorm:"type:varchar(255)" json:"description,omitempty"`
	Provider                  enums.ServerProvider       `gorm:"type:varchar(255);not null" json:"provider"`
	ProviderData              basemodels.JSONMap         `gorm:"type:json" json:"-"`
	Type                      *string                    `gorm:"type:varchar(255)" json:"type,omitempty"`
	Connected                 bool                       `gorm:"type:tinyint(1);not null;default:0" json:"connected"`
	LaunchToken               string                     `gorm:"type:varchar(32);not null" json:"-"`
	MonitoringEnabled         bool                       `gorm:"column:monitoring_enabled;type:tinyint(1);not null;default:0" json:"monitoring_enabled"`
	CPUCores                  *int                       `gorm:"column:cpu_cores;type:int" json:"cpu_cores,omitempty"`
	MemoryInMB                *int                       `gorm:"column:memory_in_mb;type:int" json:"memory_in_mb,omitempty"`
	StorageInGB               *int                       `gorm:"column:storage_in_gb;type:int" json:"storage_in_gb,omitempty"`
	OperatingSystem           *string                    `gorm:"column:operating_system;type:varchar(255)" json:"operating_system,omitempty"`
	Status                    enums.ServerStatus         `gorm:"type:varchar(255);not null" json:"status"`
	PublicIPv4                *string                    `gorm:"column:public_ipv4;type:varchar(255)" json:"public_ipv4,omitempty"`
	PrivateIPv4               *string                    `gorm:"column:private_ipv4;type:varchar(255)" json:"-"`
	PublicKey                 basemodels.EncryptedString `gorm:"type:longtext" json:"-"`
	PrivateKey                basemodels.EncryptedString `gorm:"type:longtext" json:"-"`
	UserPublicKey             basemodels.EncryptedString `gorm:"column:user_public_key;type:longtext" json:"-"`
	Username                  *string                    `gorm:"type:varchar(255)" json:"username,omitempty"`
	Password                  basemodels.EncryptedString `gorm:"type:longtext" json:"-"`
	DatabasePassword          basemodels.EncryptedString `gorm:"column:database_password;type:longtext" json:"-"`
	SSHPort                   *int                       `gorm:"column:ssh_port;type:int" json:"ssh_port,omitempty"`
	WorkingDirectory          *string                    `gorm:"column:working_directory;type:varchar(255)" json:"-"`
	CompletedProvisionSteps   basemodels.JSONStringSlice `gorm:"column:completed_provision_steps;type:json" json:"-"`
	ProvisionedAt             *time.Time                 `gorm:"column:provisioned_at;type:timestamp null" json:"provisioned_at,omitempty"`
	UninstallationRequestedAt *time.Time                 `gorm:"column:uninstallation_requested_at;type:timestamp null" json:"-"`
	Updates                   bool                       `gorm:"type:tinyint(1);not null;default:0" json:"-"`
	AutoUpdate                bool                       `gorm:"column:auto_update;type:tinyint(1);not null;default:0" json:"auto_update"`
	AvailableUpdates          *int                       `gorm:"column:available_updates;type:int" json:"-"`
	SecurityUpdates           *int                       `gorm:"column:security_updates;type:int" json:"-"`
	Progress                  int                        `gorm:"type:int;not null;default:0" json:"progress"`
	ProgressStep              *string                    `gorm:"column:progress_step;type:varchar(255)" json:"progress_step,omitempty"`
	LastUpdateCheck           *time.Time                 `gorm:"column:last_update_check;type:timestamp null" json:"-"`
	LastConnectivityCheck     *time.Time                 `gorm:"column:last_connectivity_check;type:timestamp null" json:"last_connectivity_check,omitempty"`
	ArchivedAt                *time.Time                 `gorm:"column:archived_at;type:timestamp null" json:"archived_at,omitempty"`

	// Relations
	Services      []InstalledService `gorm:"foreignKey:ServerID;references:ID" json:"services,omitempty"`
	FirewallRules []FirewallRule     `gorm:"foreignKey:ServerID;references:ID" json:"firewall_rules,omitempty"`
	Crons         []Cron             `gorm:"foreignKey:ServerID;references:ID" json:"crons,omitempty"`
	Daemons       []Daemon           `gorm:"foreignKey:ServerID;references:ID" json:"daemons,omitempty"`
	SshKeys       []SshKey           `gorm:"many2many:server_ssh_keys" json:"ssh_keys,omitempty"`
	Tasks         []Task             `gorm:"foreignKey:ServerID;references:ID" json:"tasks,omitempty"`
	Metrics       []Metric           `gorm:"foreignKey:ServerID;references:ID" json:"metrics,omitempty"`
}

func (s *Server) BeforeCreate(tx *gorm.DB) error {
	if err := s.BaseModel.BeforeCreate(tx); err != nil {
		return err
	}

	if s.Status == "" {
		s.Status = enums.ServerStatusNew
	}

	if s.LaunchToken == "" {
		s.LaunchToken = utils.GenerateHexToken(32)
	}

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
	os := enums.OSUbuntu24
	if s.OperatingSystem != nil {
		os = enums.OperatingSystem(*s.OperatingSystem)
	}

	return s.Provider.GetDefaultUsername(os)
}

func (s *Server) GetProvisionCommand() string {
	return fmt.Sprintf("wget --no-verbose -O - '%s' | bash", s.GetProvisionScriptURL())
}

// GetProvisionScriptURL returns a signed URL for the provision script
// This uses a permanent signature since custom servers need to run this at any time
func (s *Server) GetProvisionScriptURL() string {
	path := fmt.Sprintf("/servers/%s/provision-script", s.ID)
	return signedurl.PermanentSign(path, nil)
}

func (s *Server) HasFeature(feature enums.ServerFeature) bool {
	if s.Type == nil {
		return enums.ServerTypePhp.HasFeature(feature)
	}

	return enums.ServerType(*s.Type).HasFeature(feature)
}

func (s *Server) GetFeatures() []enums.ServerFeature {
	if s.Type == nil {
		return enums.ServerTypePhp.GetFeatures()
	}

	return enums.ServerType(*s.Type).GetFeatures()
}

func (s *Server) GetProcessManager() enums.ProcessManager {
	if s.Type == nil {
		return enums.ServerTypePhp.GetProcessManager()
	}

	return enums.ServerType(*s.Type).GetProcessManager()
}

func (s *Server) GetUsername() string {
	if s.Username != nil && *s.Username != "" {
		return *s.Username
	}

	return config.DefaultUsername
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
	}
}

// GetTeamID returns the team ID for broadcasting
func (s *Server) GetTeamID() string {
	return s.TeamID
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
