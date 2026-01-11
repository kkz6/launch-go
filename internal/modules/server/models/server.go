package models

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// generateRandomToken generates a random hex string of specified length
func generateRandomToken(length int) string {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}

	return hex.EncodeToString(bytes)
}

// Server represents a managed server
type Server struct {
	ID                        string               `gorm:"type:char(26);primaryKey" json:"id"`
	ServerProviderID          *string              `gorm:"column:server_provider_id;type:char(26);index" json:"server_provider_id,omitempty"`
	TeamID                    string               `gorm:"column:team_id;type:char(26);not null;index" json:"team_id"`
	UserID                    string               `gorm:"column:user_id;type:char(26);not null;index" json:"user_id"`
	Name                      string               `gorm:"type:varchar(255);not null;index" json:"name"`
	Description               *string              `gorm:"type:varchar(255)" json:"description,omitempty"`
	Provider                  enums.ServerProvider `gorm:"type:varchar(255);not null" json:"provider"`
	ProviderData              *string              `gorm:"type:json" json:"-"`
	Type                      *string              `gorm:"type:varchar(255)" json:"type,omitempty"`
	Connected                 bool                 `gorm:"type:tinyint(1);not null;default:0" json:"connected"`
	LaunchToken               string               `gorm:"type:varchar(32);not null" json:"-"`
	MonitoringEnabled         bool                 `gorm:"column:monitoring_enabled;type:tinyint(1);not null;default:0" json:"monitoring_enabled"`
	CPUCores                  *int                 `gorm:"column:cpu_cores;type:int" json:"cpu_cores,omitempty"`
	MemoryInMB                *int                 `gorm:"column:memory_in_mb;type:int" json:"memory_in_mb,omitempty"`
	StorageInGB               *int                 `gorm:"column:storage_in_gb;type:int" json:"storage_in_gb,omitempty"`
	OperatingSystem           *string              `gorm:"column:operating_system;type:varchar(255)" json:"operating_system,omitempty"`
	Status                    enums.ServerStatus   `gorm:"type:varchar(255);not null" json:"status"`
	PublicIPv4                *string              `gorm:"column:public_ipv4;type:varchar(255)" json:"public_ipv4,omitempty"`
	PrivateIPv4               *string              `gorm:"column:private_ipv4;type:varchar(255)" json:"-"`
	PublicKey                 *string              `gorm:"type:longtext;serializer:encrypted" json:"-"`
	PrivateKey                *string              `gorm:"type:longtext;serializer:encrypted" json:"-"`
	UserPublicKey             *string              `gorm:"column:user_public_key;type:longtext;serializer:encrypted" json:"-"`
	Username                  *string              `gorm:"type:varchar(255)" json:"username,omitempty"`
	Password                  *string              `gorm:"type:longtext;serializer:encrypted" json:"-"`
	DatabasePassword          *string              `gorm:"column:database_password;type:longtext;serializer:encrypted" json:"-"`
	SSHPort                   *int                 `gorm:"column:ssh_port;type:int" json:"ssh_port,omitempty"`
	WorkingDirectory          *string              `gorm:"column:working_directory;type:varchar(255)" json:"-"`
	CompletedProvisionSteps   *string              `gorm:"column:completed_provision_steps;type:json" json:"-"`
	ProvisionedAt             *time.Time           `gorm:"column:provisioned_at;type:timestamp null" json:"provisioned_at,omitempty"`
	UninstallationRequestedAt *time.Time           `gorm:"column:uninstallation_requested_at;type:timestamp null" json:"-"`
	Updates                   bool                 `gorm:"type:tinyint(1);not null;default:0" json:"-"`
	AutoUpdate                bool                 `gorm:"column:auto_update;type:tinyint(1);not null;default:0" json:"auto_update"`
	AvailableUpdates          *int                 `gorm:"column:available_updates;type:int" json:"-"`
	SecurityUpdates           *int                 `gorm:"column:security_updates;type:int" json:"-"`
	Progress                  int                  `gorm:"type:int;not null;default:0" json:"progress"`
	ProgressStep              *string              `gorm:"column:progress_step;type:varchar(255)" json:"progress_step,omitempty"`
	LastUpdateCheck           *time.Time           `gorm:"column:last_update_check;type:timestamp null" json:"-"`
	LastConnectivityCheck     *time.Time           `gorm:"column:last_connectivity_check;type:timestamp null" json:"last_connectivity_check,omitempty"`
	ArchivedAt                *time.Time           `gorm:"column:archived_at;type:timestamp null" json:"archived_at,omitempty"`
	CreatedAt                 *time.Time           `gorm:"type:timestamp null" json:"created_at,omitempty"`
	UpdatedAt                 *time.Time           `gorm:"type:timestamp null" json:"updated_at,omitempty"`

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
	if s.ID == "" {
		s.ID = utils.NewULID()
	}

	if s.Status == "" {
		s.Status = enums.ServerStatusNew
	}

	if s.LaunchToken == "" {
		s.LaunchToken = generateRandomToken(32)
	}

	return nil
}

func (s *Server) TableName() string {
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
	return fmt.Sprintf("wget --no-verbose -O - %s | bash", s.GetProvisionScriptURL())
}

func (s *Server) GetProvisionScriptURL() string {
	return fmt.Sprintf("/servers/%s/provision-script", s.ID)
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

	return "launch"
}

func (s *Server) GetSSHPort() int {
	if s.SSHPort != nil {
		return *s.SSHPort
	}

	return 22
}

func (s *Server) GetCompletedSteps() []string {
	if s.CompletedProvisionSteps == nil {
		return nil
	}

	var steps []string
	if err := json.Unmarshal([]byte(*s.CompletedProvisionSteps), &steps); err != nil {
		return nil
	}

	return steps
}

func (s *Server) SetCompletedSteps(steps []string) error {
	data, err := json.Marshal(steps)
	if err != nil {
		return err
	}

	str := string(data)
	s.CompletedProvisionSteps = &str

	return nil
}

func (s *Server) GetProviderData() map[string]interface{} {
	if s.ProviderData == nil {
		return nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(*s.ProviderData), &data); err != nil {
		return nil
	}

	return data
}

func (s *Server) SetProviderData(data map[string]interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	str := string(jsonData)
	s.ProviderData = &str

	return nil
}
