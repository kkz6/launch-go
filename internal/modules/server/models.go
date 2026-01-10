package server

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

type ServerStatus string

const (
	ServerStatusPending      ServerStatus = "pending"
	ServerStatusProvisioning ServerStatus = "provisioning"
	ServerStatusActive       ServerStatus = "active"
	ServerStatusFailed       ServerStatus = "failed"
	ServerStatusDeleting     ServerStatus = "deleting"
)

type ServerProvider string

const (
	ProviderDigitalOcean ServerProvider = "digitalocean"
	ProviderHetzner      ServerProvider = "hetzner"
	ProviderAWS          ServerProvider = "aws"
	ProviderLinode       ServerProvider = "linode"
	ProviderVultr        ServerProvider = "vultr"
	ProviderCustom       ServerProvider = "custom"
)

type Server struct {
	ID                string         `gorm:"primaryKey;size:26" json:"id"`
	TeamID            string         `gorm:"size:26;not null;index" json:"team_id"`
	Name              string         `gorm:"size:255;not null" json:"name"`
	Provider          ServerProvider `gorm:"size:50;not null" json:"provider"`
	ProviderServerID  *string        `gorm:"size:255" json:"provider_server_id,omitempty"`
	IPAddress         *string        `gorm:"size:45" json:"ip_address,omitempty"`
	PrivateIPAddress  *string        `gorm:"size:45" json:"private_ip_address,omitempty"`
	Region            string         `gorm:"size:100" json:"region"`
	Size              string         `gorm:"size:100" json:"size"`
	Status            ServerStatus   `gorm:"size:50;default:'pending'" json:"status"`
	SSHPort           int            `gorm:"default:22" json:"ssh_port"`
	SSHUser           string         `gorm:"size:100;default:'root'" json:"ssh_user"`
	PrivateKey        string         `gorm:"type:text" json:"-"`
	PublicKey         string         `gorm:"type:text" json:"public_key,omitempty"`
	PHPVersion        string         `gorm:"size:10;default:'8.3'" json:"php_version"`
	DatabaseType      *string        `gorm:"size:50" json:"database_type,omitempty"`
	WebServer         string         `gorm:"size:50;default:'caddy'" json:"web_server"`
	ConnectedAt       *time.Time     `json:"connected_at,omitempty"`
	LastCheckedAt     *time.Time     `json:"last_checked_at,omitempty"`
	ProviderData      string         `gorm:"type:json" json:"-"`
	Features          string         `gorm:"type:json" json:"features,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Sites         []Site         `gorm:"foreignKey:ServerID" json:"sites,omitempty"`
	Databases     []Database     `gorm:"foreignKey:ServerID" json:"databases,omitempty"`
	SSHKeys       []SSHKey       `gorm:"foreignKey:ServerID" json:"ssh_keys,omitempty"`
	FirewallRules []FirewallRule `gorm:"foreignKey:ServerID" json:"firewall_rules,omitempty"`
	CronJobs      []CronJob      `gorm:"foreignKey:ServerID" json:"cron_jobs,omitempty"`
}

func (s *Server) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = utils.NewULID()
	}
	return nil
}

type Site struct {
	ID        string         `gorm:"primaryKey;size:26" json:"id"`
	ServerID  string         `gorm:"size:26;not null;index" json:"server_id"`
	TeamID    string         `gorm:"size:26;not null;index" json:"team_id"`
	Name      string         `gorm:"size:255;not null" json:"name"`
	Domain    string         `gorm:"size:255;not null" json:"domain"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (s *Site) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = utils.NewULID()
	}
	return nil
}

type Database struct {
	ID        string         `gorm:"primaryKey;size:26" json:"id"`
	ServerID  string         `gorm:"size:26;not null;index" json:"server_id"`
	Name      string         `gorm:"size:255;not null" json:"name"`
	Type      string         `gorm:"size:50;not null" json:"type"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (d *Database) BeforeCreate(tx *gorm.DB) error {
	if d.ID == "" {
		d.ID = utils.NewULID()
	}
	return nil
}

type SSHKey struct {
	ID          string         `gorm:"primaryKey;size:26" json:"id"`
	ServerID    string         `gorm:"size:26;not null;index" json:"server_id"`
	Name        string         `gorm:"size:255;not null" json:"name"`
	PublicKey   string         `gorm:"type:text;not null" json:"public_key"`
	Fingerprint string         `gorm:"size:255" json:"fingerprint"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (k *SSHKey) BeforeCreate(tx *gorm.DB) error {
	if k.ID == "" {
		k.ID = utils.NewULID()
	}
	return nil
}

type FirewallRule struct {
	ID        string         `gorm:"primaryKey;size:26" json:"id"`
	ServerID  string         `gorm:"size:26;not null;index" json:"server_id"`
	Name      string         `gorm:"size:255" json:"name"`
	Port      int            `gorm:"not null" json:"port"`
	Protocol  string         `gorm:"size:10;default:'tcp'" json:"protocol"`
	FromIP    string         `gorm:"size:45" json:"from_ip"`
	Status    string         `gorm:"size:50;default:'active'" json:"status"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (r *FirewallRule) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = utils.NewULID()
	}
	return nil
}

type CronJob struct {
	ID        string         `gorm:"primaryKey;size:26" json:"id"`
	ServerID  string         `gorm:"size:26;not null;index" json:"server_id"`
	SiteID    *string        `gorm:"size:26;index" json:"site_id,omitempty"`
	Command   string         `gorm:"type:text;not null" json:"command"`
	Schedule  string         `gorm:"size:100;not null" json:"schedule"`
	User      string         `gorm:"size:100;default:'root'" json:"user"`
	Status    string         `gorm:"size:50;default:'active'" json:"status"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (j *CronJob) BeforeCreate(tx *gorm.DB) error {
	if j.ID == "" {
		j.ID = utils.NewULID()
	}
	return nil
}
