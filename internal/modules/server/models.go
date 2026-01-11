package server

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

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
	ID                       string          `gorm:"primaryKey;size:26" json:"id"`
	TeamID                   string          `gorm:"size:26;not null;index" json:"team_id"`
	UserID                   string          `gorm:"size:26;not null;index" json:"user_id"`
	ServerProviderID         *string         `gorm:"size:26;index" json:"server_provider_id,omitempty"`
	Name                     string          `gorm:"size:255;not null" json:"name"`
	Description              *string         `gorm:"type:text" json:"description,omitempty"`
	Provider                 ServerProvider  `gorm:"size:50;not null" json:"provider"`
	ProviderData             *string         `gorm:"type:json" json:"-"`
	Type                     ServerType      `gorm:"size:50;not null;default:'php'" json:"type"`
	Connected                bool            `gorm:"default:false" json:"connected"`
	LaunchToken              string          `gorm:"size:64" json:"-"`
	MonitoringEnabled        bool            `gorm:"default:false" json:"monitoring_enabled"`
	CPUCores                 *int            `json:"cpu_cores,omitempty"`
	MemoryInMB               *int            `json:"memory_in_mb,omitempty"`
	StorageInGB              *int            `json:"storage_in_gb,omitempty"`
	OperatingSystem          OperatingSystem `gorm:"size:50;default:'ubuntu_24'" json:"operating_system"`
	Status                   ServerStatus    `gorm:"size:50;default:'new'" json:"status"`
	PublicIPv4               *string         `gorm:"size:45" json:"public_ipv4,omitempty"`
	PrivateIPv4              *string         `gorm:"size:45" json:"-"`
	PublicKey                *string         `gorm:"type:text" json:"-"`
	PrivateKey               *string         `gorm:"type:text" json:"-"`
	UserPublicKey            *string         `gorm:"type:text" json:"-"`
	Username                 string          `gorm:"size:100;default:'launch'" json:"username"`
	Password                 *string         `gorm:"type:text" json:"-"`
	DatabasePassword         *string         `gorm:"type:text" json:"-"`
	SSHPort                  int             `gorm:"default:22" json:"ssh_port"`
	WorkingDirectory         *string         `gorm:"size:255" json:"-"`
	CompletedProvisionSteps  *string         `gorm:"type:json" json:"-"`
	ProvisionedAt            *time.Time      `json:"provisioned_at,omitempty"`
	UninstallationRequestedAt *time.Time     `json:"-"`
	Updates                  *string         `gorm:"type:json" json:"-"`
	AutoUpdate               bool            `gorm:"default:false" json:"auto_update"`
	AvailableUpdates         int             `gorm:"default:0" json:"-"`
	SecurityUpdates          int             `gorm:"default:0" json:"-"`
	LastUpdateCheck          *time.Time      `json:"-"`
	LastConnectivityCheck    *time.Time      `json:"last_connectivity_check,omitempty"`
	Progress                 *int            `json:"progress,omitempty"`
	ProgressStep             *string         `gorm:"size:255" json:"progress_step,omitempty"`
	ArchivedAt               *time.Time      `json:"archived_at,omitempty"`
	CreatedAt                time.Time       `json:"created_at"`
	UpdatedAt                time.Time       `json:"updated_at"`
	DeletedAt                gorm.DeletedAt  `gorm:"index" json:"-"`

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
		s.Status = ServerStatusNew
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
	// This would be configured based on the application URL
	return fmt.Sprintf("/servers/%s/provision-script", s.ID)
}

func (s *Server) HasFeature(feature ServerFeature) bool {
	return s.Type.HasFeature(feature)
}

func (s *Server) GetFeatures() []ServerFeature {
	return s.Type.GetFeatures()
}

func (s *Server) GetProcessManager() ProcessManager {
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

// InstalledService represents an installed service on a server
type InstalledService struct {
	ID        string        `gorm:"primaryKey;size:26" json:"id"`
	ServerID  string        `gorm:"size:26;not null;index" json:"server_id"`
	Type      ServiceType   `gorm:"size:50;not null" json:"type"`
	TypeData  *string       `gorm:"type:json" json:"-"`
	Name      string        `gorm:"size:255;not null" json:"name"`
	Version   *string       `gorm:"size:50" json:"version,omitempty"`
	Status    ServiceStatus `gorm:"size:50;default:'pending'" json:"status"`
	IsDefault bool          `gorm:"default:false" json:"is_default"`
	Unit      *string       `gorm:"size:255" json:"unit,omitempty"`
	Software  *Software     `gorm:"size:50" json:"software,omitempty"`
	TaskID    *string       `gorm:"size:26" json:"task_id,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`

	// Relations
	Server *Server `gorm:"foreignKey:ServerID" json:"server,omitempty"`
}

func (s *InstalledService) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = utils.NewULID()
	}
	if s.Status == "" {
		s.Status = ServiceStatusPending
	}
	return nil
}

func (s *InstalledService) TableName() string {
	return "services"
}

func (s *InstalledService) GetFormattedVersion() string {
	if s.Software != nil {
		return s.Software.GetVersion()
	}
	if s.Version != nil {
		return *s.Version
	}
	return ""
}

func (s *InstalledService) GetTypeData() map[string]interface{} {
	if s.TypeData == nil {
		return nil
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(*s.TypeData), &data); err != nil {
		return nil
	}
	return data
}

func (s *InstalledService) SetTypeData(data map[string]interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	str := string(jsonData)
	s.TypeData = &str
	return nil
}

// FirewallRule represents a firewall rule on a server
type FirewallRule struct {
	ID                        string     `gorm:"primaryKey;size:26" json:"id"`
	ServerID                  string     `gorm:"size:26;not null;index" json:"server_id"`
	Name                      string     `gorm:"size:255;not null" json:"name"`
	Action                    RuleAction `gorm:"size:50;not null;default:'allow'" json:"action"`
	Port                      *string    `gorm:"size:50" json:"port,omitempty"`
	FromIPv4                  *string    `gorm:"size:45" json:"from_ipv4,omitempty"`
	Mask                      *string    `gorm:"size:10" json:"mask,omitempty"`
	Note                      *string    `gorm:"type:text" json:"note,omitempty"`
	InstalledAt               *time.Time `json:"installed_at,omitempty"`
	InstallationFailedAt      *time.Time `json:"installation_failed_at,omitempty"`
	UninstallationRequestedAt *time.Time `json:"-"`
	UninstallationFailedAt    *time.Time `json:"-"`
	CreatedAt                 time.Time  `json:"created_at"`
	UpdatedAt                 time.Time  `json:"updated_at"`

	// Relations
	Server *Server `gorm:"foreignKey:ServerID" json:"server,omitempty"`
}

func (f *FirewallRule) BeforeCreate(tx *gorm.DB) error {
	if f.ID == "" {
		f.ID = utils.NewULID()
	}
	if f.Action == "" {
		f.Action = RuleActionAllow
	}
	return nil
}

func (f *FirewallRule) TableName() string {
	return "firewall_rules"
}

func (f *FirewallRule) IsInstalled() bool {
	return f.InstalledAt != nil
}

func (f *FirewallRule) IsPending() bool {
	return f.InstalledAt == nil && f.InstallationFailedAt == nil
}

func (f *FirewallRule) HasFailed() bool {
	return f.InstallationFailedAt != nil
}

func (f *FirewallRule) FormatAsUfwRule() string {
	parts := []string{f.Action.String()}

	if f.FromIPv4 != nil && *f.FromIPv4 != "" {
		parts = append(parts, fmt.Sprintf("from %s to any port", *f.FromIPv4))
	}

	if f.Port != nil && *f.Port != "" {
		parts = append(parts, *f.Port)
	}

	return strings.Join(parts, " ")
}

// Cron represents a scheduled cron job on a server
type Cron struct {
	ID                        string     `gorm:"primaryKey;size:26" json:"id"`
	ServerID                  string     `gorm:"size:26;not null;index" json:"server_id"`
	SiteID                    *string    `gorm:"size:26;index" json:"site_id,omitempty"`
	User                      string     `gorm:"size:100;not null;default:'root'" json:"user"`
	Expression                string     `gorm:"size:100;not null" json:"expression"`
	Command                   string     `gorm:"type:text;not null" json:"command"`
	Frequency                 *string    `gorm:"size:100" json:"frequency,omitempty"`
	Hidden                    bool       `gorm:"default:false" json:"hidden"`
	InstalledAt               *time.Time `json:"installed_at,omitempty"`
	InstallationFailedAt      *time.Time `json:"installation_failed_at,omitempty"`
	UninstallationRequestedAt *time.Time `json:"-"`
	UninstallationFailedAt    *time.Time `json:"-"`
	CreatedAt                 time.Time  `json:"created_at"`
	UpdatedAt                 time.Time  `json:"updated_at"`

	// Relations
	Server *Server `gorm:"foreignKey:ServerID" json:"server,omitempty"`
}

func (c *Cron) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = utils.NewULID()
	}
	if c.User == "" {
		c.User = "root"
	}
	return nil
}

func (c *Cron) TableName() string {
	return "crons"
}

func (c *Cron) IsInstalled() bool {
	return c.InstalledAt != nil
}

func (c *Cron) Path() string {
	return fmt.Sprintf("/etc/cron.d/cron-%s", c.ID)
}

func (c *Cron) LogPath(workingDirectory string) string {
	if c.User == "root" {
		return fmt.Sprintf("/root/%s/cron-%s.log", workingDirectory, c.ID)
	}
	return fmt.Sprintf("/home/%s/%s/cron-%s.log", c.User, workingDirectory, c.ID)
}

// Daemon represents a background process managed by supervisor
type Daemon struct {
	ID                        string     `gorm:"primaryKey;size:26" json:"id"`
	ServerID                  string     `gorm:"size:26;not null;index" json:"server_id"`
	User                      string     `gorm:"size:100;not null;default:'root'" json:"user"`
	Directory                 *string    `gorm:"size:255" json:"directory,omitempty"`
	Command                   string     `gorm:"type:text;not null" json:"command"`
	Processes                 int        `gorm:"default:1" json:"processes"`
	StopWaitSeconds           int        `gorm:"default:10" json:"stop_wait_seconds"`
	StopSignal                *string    `gorm:"size:20" json:"stop_signal,omitempty"`
	InstalledAt               *time.Time `json:"installed_at,omitempty"`
	InstallationFailedAt      *time.Time `json:"installation_failed_at,omitempty"`
	UninstallationRequestedAt *time.Time `json:"-"`
	UninstallationFailedAt    *time.Time `json:"-"`
	LastStatusCheck           *time.Time `json:"last_status_check,omitempty"`
	Running                   bool       `gorm:"default:false" json:"running"`
	Info                      *string    `gorm:"type:json" json:"-"`
	CreatedAt                 time.Time  `json:"created_at"`
	UpdatedAt                 time.Time  `json:"updated_at"`

	// Relations
	Server *Server `gorm:"foreignKey:ServerID" json:"server,omitempty"`
}

func (d *Daemon) BeforeCreate(tx *gorm.DB) error {
	if d.ID == "" {
		d.ID = utils.NewULID()
	}
	if d.User == "" {
		d.User = "root"
	}
	if d.Processes == 0 {
		d.Processes = 1
	}
	if d.StopWaitSeconds == 0 {
		d.StopWaitSeconds = 10
	}
	return nil
}

func (d *Daemon) TableName() string {
	return "daemons"
}

func (d *Daemon) IsInstalled() bool {
	return d.InstalledAt != nil
}

func (d *Daemon) Path() string {
	return fmt.Sprintf("/etc/supervisor/conf.d/daemon-%s.conf", d.ID)
}

func (d *Daemon) GetInfo() map[string]interface{} {
	if d.Info == nil {
		return nil
	}
	var info map[string]interface{}
	if err := json.Unmarshal([]byte(*d.Info), &info); err != nil {
		return nil
	}
	return info
}

func (d *Daemon) SetInfo(info map[string]interface{}) error {
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	str := string(data)
	d.Info = &str
	return nil
}

// SshKey represents an SSH public key
type SshKey struct {
	ID          string     `gorm:"primaryKey;size:26" json:"id"`
	UserID      *string    `gorm:"size:26;index" json:"user_id,omitempty"`
	TeamID      *string    `gorm:"size:26;index" json:"team_id,omitempty"`
	IsGlobal    bool       `gorm:"default:false" json:"is_global"`
	PublicKey   string     `gorm:"type:text;not null" json:"-"`
	Name        string     `gorm:"size:255;not null" json:"name"`
	Description *string    `gorm:"type:text" json:"description,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`

	// Relations
	Servers []Server `gorm:"many2many:server_ssh_keys" json:"servers,omitempty"`
}

func (k *SshKey) BeforeCreate(tx *gorm.DB) error {
	if k.ID == "" {
		k.ID = utils.NewULID()
	}
	return nil
}

func (k *SshKey) TableName() string {
	return "ssh_keys"
}

func (k *SshKey) GetFingerprint() string {
	return GenerateSSHFingerprint(k.PublicKey, FingerprintAlgorithmMD5)
}

// ServerSshKey represents the many-to-many relationship between servers and SSH keys
type ServerSshKey struct {
	ServerID  string    `gorm:"primaryKey;size:26" json:"server_id"`
	SshKeyID  string    `gorm:"primaryKey;size:26" json:"ssh_key_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *ServerSshKey) TableName() string {
	return "server_ssh_keys"
}

// FingerprintAlgorithm represents the algorithm used for SSH fingerprints
type FingerprintAlgorithm string

const (
	FingerprintAlgorithmMD5    FingerprintAlgorithm = "md5"
	FingerprintAlgorithmSHA256 FingerprintAlgorithm = "sha256"
)

// GenerateSSHFingerprint generates a fingerprint for an SSH public key
func GenerateSSHFingerprint(publicKey string, algorithm FingerprintAlgorithm) string {
	if !strings.HasPrefix(publicKey, "ssh-") {
		return ""
	}

	parts := strings.SplitN(publicKey, " ", 3)
	if len(parts) < 2 {
		return ""
	}

	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}

	switch algorithm {
	case FingerprintAlgorithmMD5:
		hash := md5.Sum(decoded)
		hexParts := make([]string, len(hash))
		for i, b := range hash {
			hexParts[i] = fmt.Sprintf("%02x", b)
		}
		return strings.Join(hexParts, ":")
	case FingerprintAlgorithmSHA256:
		// SHA256 fingerprints are typically base64 encoded
		return base64.StdEncoding.EncodeToString(decoded)
	}

	return ""
}

// Task represents a task execution record
type Task struct {
	ID          string     `gorm:"primaryKey;size:26" json:"id"`
	ServerID    string     `gorm:"size:26;not null;index" json:"server_id"`
	Type        string     `gorm:"size:255;not null" json:"type"`
	Status      string     `gorm:"size:50;default:'pending'" json:"status"`
	Name        *string    `gorm:"size:255" json:"name,omitempty"`
	User        *string    `gorm:"size:100" json:"user,omitempty"`
	Script      *string    `gorm:"type:text" json:"-"`
	Output      *string    `gorm:"type:longtext" json:"output,omitempty"`
	ExitCode    *int       `json:"exit_code,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`

	// Relations
	Server *Server `gorm:"foreignKey:ServerID" json:"server,omitempty"`
}

func (t *Task) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = utils.NewULID()
	}
	if t.Status == "" {
		t.Status = "pending"
	}
	return nil
}

func (t *Task) TableName() string {
	return "tasks"
}

func (t *Task) IsSuccessful() bool {
	return t.ExitCode != nil && *t.ExitCode == 0
}

func (t *Task) Duration() time.Duration {
	if t.StartedAt == nil || t.FinishedAt == nil {
		return 0
	}
	return t.FinishedAt.Sub(*t.StartedAt)
}

// Metric represents server performance metrics
type Metric struct {
	ID           string    `gorm:"primaryKey;size:26" json:"id"`
	ServerID     string    `gorm:"size:26;not null;index" json:"server_id"`
	CPUUsage     float64   `json:"cpu_usage"`
	MemoryUsage  float64   `json:"memory_usage"`
	DiskUsage    float64   `json:"disk_usage"`
	LoadAverage1 float64   `json:"load_average_1"`
	LoadAverage5 float64   `json:"load_average_5"`
	LoadAverage15 float64  `json:"load_average_15"`
	RecordedAt   time.Time `json:"recorded_at"`
	CreatedAt    time.Time `json:"created_at"`

	// Relations
	Server *Server `gorm:"foreignKey:ServerID" json:"server,omitempty"`
}

func (m *Metric) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = utils.NewULID()
	}
	return nil
}

func (m *Metric) TableName() string {
	return "metrics"
}

// AllModels returns all models for migration
func AllModels() []interface{} {
	return []interface{}{
		&Server{},
		&InstalledService{},
		&FirewallRule{},
		&Cron{},
		&Daemon{},
		&SshKey{},
		&ServerSshKey{},
		&Task{},
		&Metric{},
	}
}
