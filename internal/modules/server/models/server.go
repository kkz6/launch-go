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
	ID                        string                `gorm:"primaryKey;size:26" json:"id"`
	TeamID                    string                `gorm:"size:26;not null;index" json:"team_id"`
	UserID                    string                `gorm:"size:26;not null;index" json:"user_id"`
	ServerProviderID          *string               `gorm:"size:26;index" json:"server_provider_id,omitempty"`
	Name                      string                `gorm:"size:255;not null" json:"name"`
	Description               *string               `gorm:"type:text" json:"description,omitempty"`
	Provider                  enums.ServerProvider  `gorm:"size:50;not null" json:"provider"`
	ProviderData              *string               `gorm:"type:json" json:"-"`
	Type                      enums.ServerType      `gorm:"size:50;not null;default:'php'" json:"type"`
	Connected                 bool                  `gorm:"default:false" json:"connected"`
	LaunchToken               string                `gorm:"size:64" json:"-"`
	MonitoringEnabled         bool                  `gorm:"default:false" json:"monitoring_enabled"`
	CPUCores                  *int                  `json:"cpu_cores,omitempty"`
	MemoryInMB                *int                  `json:"memory_in_mb,omitempty"`
	StorageInGB               *int                  `json:"storage_in_gb,omitempty"`
	OperatingSystem           enums.OperatingSystem `gorm:"size:50;default:'ubuntu_24'" json:"operating_system"`
	Status                    enums.ServerStatus    `gorm:"size:50;default:'new'" json:"status"`
	PublicIPv4                *string               `gorm:"size:45" json:"public_ipv4,omitempty"`
	PrivateIPv4               *string               `gorm:"size:45" json:"-"`
	PublicKey                 *string               `gorm:"type:text" json:"-"`
	PrivateKey                *string               `gorm:"type:text" json:"-"`
	UserPublicKey             *string               `gorm:"type:text" json:"-"`
	Username                  string                `gorm:"size:100;default:'launch'" json:"username"`
	Password                  *string               `gorm:"type:text" json:"-"`
	DatabasePassword          *string               `gorm:"type:text" json:"-"`
	SSHPort                   int                   `gorm:"default:22" json:"ssh_port"`
	WorkingDirectory          *string               `gorm:"size:255" json:"-"`
	CompletedProvisionSteps   *string               `gorm:"type:json" json:"-"`
	ProvisionedAt             *time.Time            `json:"provisioned_at,omitempty"`
	UninstallationRequestedAt *time.Time            `json:"-"`
	Updates                   *string               `gorm:"type:json" json:"-"`
	AutoUpdate                bool                  `gorm:"default:false" json:"auto_update"`
	AvailableUpdates          int                   `gorm:"default:0" json:"-"`
	SecurityUpdates           int                   `gorm:"default:0" json:"-"`
	LastUpdateCheck           *time.Time            `json:"-"`
	LastConnectivityCheck     *time.Time            `json:"last_connectivity_check,omitempty"`
	Progress                  *int                  `json:"progress,omitempty"`
	ProgressStep              *string               `gorm:"size:255" json:"progress_step,omitempty"`
	ArchivedAt                *time.Time            `json:"archived_at,omitempty"`
	CreatedAt                 time.Time             `json:"created_at"`
	UpdatedAt                 time.Time             `json:"updated_at"`
	DeletedAt                 gorm.DeletedAt        `gorm:"index" json:"-"`

	// Relations
	Services      []InstalledService `gorm:"foreignKey:ServerID" json:"services,omitempty"`
	FirewallRules []FirewallRule     `gorm:"foreignKey:ServerID" json:"firewall_rules,omitempty"`
	Crons         []Cron             `gorm:"foreignKey:ServerID" json:"crons,omitempty"`
	Daemons       []Daemon           `gorm:"foreignKey:ServerID" json:"daemons,omitempty"`
	SshKeys       []SshKey           `gorm:"many2many:server_ssh_keys" json:"ssh_keys,omitempty"`
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
	return s.Provider.GetDefaultUsername(s.OperatingSystem)
}

func (s *Server) GetProvisionCommand() string {
	return fmt.Sprintf("wget --no-verbose -O - %s | bash", s.GetProvisionScriptURL())
}

func (s *Server) GetProvisionScriptURL() string {
	return fmt.Sprintf("/servers/%s/provision-script", s.ID)
}

func (s *Server) HasFeature(feature enums.ServerFeature) bool {
	return s.Type.HasFeature(feature)
}

func (s *Server) GetFeatures() []enums.ServerFeature {
	return s.Type.GetFeatures()
}

func (s *Server) GetProcessManager() enums.ProcessManager {
	return s.Type.GetProcessManager()
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
