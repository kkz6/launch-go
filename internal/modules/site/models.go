package site

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// Site represents a web application deployed on a server
type Site struct {
	ID                            string         `gorm:"primaryKey;size:26" json:"id"`
	ServerID                      string         `gorm:"size:26;not null;index" json:"server_id"`
	UserID                        string         `gorm:"size:26;not null;index" json:"user_id"`
	SourceControlID               *string        `gorm:"size:26;index" json:"source_control_id,omitempty"`
	SourceControlRepositoriesID   *string        `gorm:"size:26;index" json:"source_control_repositories_id,omitempty"`
	ConnectedDomainID             *string        `gorm:"size:26;index" json:"connected_domain_id,omitempty"`
	Address                       string         `gorm:"size:255;not null" json:"address"`
	Type                          SiteType       `gorm:"size:50;not null;default:'laravel'" json:"type"`
	TypeData                      string         `gorm:"type:json" json:"type_data,omitempty"`
	VcsData                       string         `gorm:"type:json" json:"vcs_data,omitempty"`
	Aliases                       string         `gorm:"type:json" json:"aliases,omitempty"`
	TlsSetting                    TlsSetting     `gorm:"size:50;default:'auto'" json:"tls_setting"`
	PendingTlsUpdateSince         *time.Time     `json:"pending_tls_update_since,omitempty"`
	ZeroDowntimeDeployment        bool           `gorm:"default:true" json:"zero_downtime_deployment"`
	DeploymentReleasesRetention   int            `gorm:"default:5" json:"deployment_releases_retention"`
	RepositoryBranch              string         `gorm:"size:255;default:'main'" json:"repository_branch"`
	DeployToken                   string         `gorm:"size:64" json:"-"`
	DeployNotificationEmail       *string        `gorm:"size:255" json:"deploy_notification_email,omitempty"`
	User                          string         `gorm:"size:100" json:"user"`
	Path                          string         `gorm:"size:500" json:"path"`
	WebFolder                     string         `gorm:"size:255;default:'public'" json:"web_folder"`
	PhpVersion                    string         `gorm:"size:10" json:"php_version"`
	PendingCaddyfileUpdateSince   *time.Time     `json:"pending_caddyfile_update_since,omitempty"`
	SharedDirectories             string         `gorm:"type:json" json:"shared_directories,omitempty"`
	WriteableDirectories          string         `gorm:"type:json" json:"writeable_directories,omitempty"`
	SharedFiles                   string         `gorm:"type:json" json:"shared_files,omitempty"`
	Progress                      *string        `gorm:"size:255" json:"progress,omitempty"`
	AutoDeployment                bool           `gorm:"default:false" json:"auto_deployment"`
	QueueDeployments              bool           `gorm:"default:false" json:"queue_deployments"`
	AutoRestartQueue              bool           `gorm:"default:false" json:"auto_restart_queue"`
	HookBeforeUpdatingRepository  string         `gorm:"type:text" json:"hook_before_updating_repository,omitempty"`
	HookAfterUpdatingRepository   string         `gorm:"type:text" json:"hook_after_updating_repository,omitempty"`
	HookBeforeMakingCurrent       string         `gorm:"type:text" json:"hook_before_making_current,omitempty"`
	HookAfterMakingCurrent        string         `gorm:"type:text" json:"hook_after_making_current,omitempty"`
	InstalledAt                   *time.Time     `json:"installed_at,omitempty"`
	InstallationFailedAt          *time.Time     `json:"installation_failed_at,omitempty"`
	UninstallationRequestedAt     *time.Time     `json:"uninstallation_requested_at,omitempty"`
	UninstallationFailedAt        *time.Time     `json:"uninstallation_failed_at,omitempty"`
	Features                      string         `gorm:"type:json" json:"features,omitempty"`
	EnabledFeatures               string         `gorm:"type:json" json:"enabled_features,omitempty"`
	PendingFeatures               string         `gorm:"type:json" json:"pending_features,omitempty"`
	CreatedAt                     time.Time      `json:"created_at"`
	UpdatedAt                     time.Time      `json:"updated_at"`
	DeletedAt                     gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Deployments      []Deployment   `gorm:"foreignKey:SiteID" json:"deployments,omitempty"`
	LatestDeployment *Deployment    `gorm:"-" json:"latest_deployment,omitempty"`
	Certificates     []Certificate  `gorm:"foreignKey:SiteID" json:"certificates,omitempty"`
	Queues           []Queue        `gorm:"foreignKey:SiteID" json:"queues,omitempty"`
	Commands         []Command      `gorm:"foreignKey:SiteID" json:"commands,omitempty"`
	Redirects        []Redirect     `gorm:"foreignKey:SiteID" json:"redirects,omitempty"`
}

func (s *Site) TableName() string {
	return "sites"
}

func (s *Site) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = utils.NewULID()
	}
	if s.DeployToken == "" {
		s.DeployToken = generateRandomToken(32)
	}
	if s.SharedDirectories == "" {
		s.SharedDirectories = "[]"
	}
	if s.WriteableDirectories == "" {
		s.WriteableDirectories = "[]"
	}
	if s.SharedFiles == "" {
		s.SharedFiles = "[]"
	}
	return nil
}

// GetURL returns the full URL for the site
func (s *Site) GetURL() string {
	return fmt.Sprintf("%s://%s", s.TlsSetting.GetProtocol(), s.Address)
}

// GetPort returns the HTTP port based on TLS setting
func (s *Site) GetPort() int {
	return s.TlsSetting.GetPort()
}

// StartsWithWww checks if the address starts with www.
func (s *Site) StartsWithWww() bool {
	return strings.HasPrefix(s.Address, "www.")
}

// GetLogsDirectory returns the logs directory path
func (s *Site) GetLogsDirectory() string {
	return fmt.Sprintf("%s/logs", s.Path)
}

// GetApplicationDirectory returns the current application directory
func (s *Site) GetApplicationDirectory() string {
	if s.ZeroDowntimeDeployment {
		return fmt.Sprintf("%s/current", s.Path)
	}
	return fmt.Sprintf("%s/repository", s.Path)
}

// GetWebDirectory returns the web directory path
func (s *Site) GetWebDirectory() string {
	return s.GenerateWebDirectory(s.WebFolder)
}

// GenerateWebDirectory generates the web directory path for a given folder
func (s *Site) GenerateWebDirectory(folder string) string {
	folder = strings.Trim(folder, "/")
	var path string
	if s.ZeroDowntimeDeployment {
		path = fmt.Sprintf("%s/current/%s", s.Path, folder)
	} else {
		path = fmt.Sprintf("%s/repository/%s", s.Path, folder)
	}
	return strings.TrimRight(path, "/")
}

// GetAliases returns the site aliases as a slice
func (s *Site) GetAliases() []string {
	if s.Aliases == "" || s.Aliases == "null" {
		return []string{}
	}
	var aliases []string
	if err := json.Unmarshal([]byte(s.Aliases), &aliases); err != nil {
		return []string{}
	}
	return aliases
}

// SetAliases sets the site aliases from a slice
func (s *Site) SetAliases(aliases []string) error {
	data, err := json.Marshal(aliases)
	if err != nil {
		return err
	}
	s.Aliases = string(data)
	return nil
}

// GetSharedDirectories returns shared directories as a slice
func (s *Site) GetSharedDirectories() []string {
	if s.SharedDirectories == "" || s.SharedDirectories == "null" {
		return []string{}
	}
	var dirs []string
	if err := json.Unmarshal([]byte(s.SharedDirectories), &dirs); err != nil {
		return []string{}
	}
	return dirs
}

// SetSharedDirectories sets shared directories from a slice
func (s *Site) SetSharedDirectories(dirs []string) error {
	data, err := json.Marshal(dirs)
	if err != nil {
		return err
	}
	s.SharedDirectories = string(data)
	return nil
}

// GetWriteableDirectories returns writeable directories as a slice
func (s *Site) GetWriteableDirectories() []string {
	if s.WriteableDirectories == "" || s.WriteableDirectories == "null" {
		return []string{}
	}
	var dirs []string
	if err := json.Unmarshal([]byte(s.WriteableDirectories), &dirs); err != nil {
		return []string{}
	}
	return dirs
}

// SetWriteableDirectories sets writeable directories from a slice
func (s *Site) SetWriteableDirectories(dirs []string) error {
	data, err := json.Marshal(dirs)
	if err != nil {
		return err
	}
	s.WriteableDirectories = string(data)
	return nil
}

// GetSharedFiles returns shared files as a slice
func (s *Site) GetSharedFiles() []string {
	if s.SharedFiles == "" || s.SharedFiles == "null" {
		return []string{}
	}
	var files []string
	if err := json.Unmarshal([]byte(s.SharedFiles), &files); err != nil {
		return []string{}
	}
	return files
}

// SetSharedFiles sets shared files from a slice
func (s *Site) SetSharedFiles(files []string) error {
	data, err := json.Marshal(files)
	if err != nil {
		return err
	}
	s.SharedFiles = string(data)
	return nil
}

// IsInstalled returns true if the site is installed
func (s *Site) IsInstalled() bool {
	return s.InstalledAt != nil
}

// HasFeature checks if the site has a specific feature
func (s *Site) HasFeature(feature string) bool {
	features := s.GetFeatures()
	for _, f := range features {
		if f == feature {
			return true
		}
	}
	return false
}

// GetFeatures returns features as a slice
func (s *Site) GetFeatures() []string {
	if s.Features == "" || s.Features == "null" {
		return []string{}
	}
	var features []string
	if err := json.Unmarshal([]byte(s.Features), &features); err != nil {
		return []string{}
	}
	return features
}

// Deployment represents a site deployment
type Deployment struct {
	ID             string           `gorm:"primaryKey;size:26" json:"id"`
	SiteID         string           `gorm:"size:26;not null;index" json:"site_id"`
	UserID         *string          `gorm:"size:26;index" json:"user_id,omitempty"`
	TaskID         *string          `gorm:"size:26;index" json:"task_id,omitempty"`
	Status         DeploymentStatus `gorm:"size:50;default:'pending'" json:"status"`
	GitHash        *string          `gorm:"size:40" json:"git_hash,omitempty"`
	CommitData     string           `gorm:"type:json" json:"commit_data,omitempty"`
	VcsData        string           `gorm:"type:json" json:"vcs_data,omitempty"`
	UserNotifiedAt *time.Time       `json:"user_notified_at,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`

	// Relations
	Site *Site `gorm:"foreignKey:SiteID" json:"site,omitempty"`
}

func (d *Deployment) TableName() string {
	return "deployments"
}

func (d *Deployment) BeforeCreate(tx *gorm.DB) error {
	if d.ID == "" {
		d.ID = utils.NewULID()
	}
	return nil
}

// GetShortGitHash returns the first 7 characters of the git hash
func (d *Deployment) GetShortGitHash() string {
	if d.GitHash == nil {
		return ""
	}
	if len(*d.GitHash) < 7 {
		return *d.GitHash
	}
	return (*d.GitHash)[:7]
}

// GetCommitData returns commit data as a map
func (d *Deployment) GetCommitData() map[string]interface{} {
	if d.CommitData == "" || d.CommitData == "null" {
		return map[string]interface{}{}
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(d.CommitData), &data); err != nil {
		return map[string]interface{}{}
	}
	return data
}

// SetCommitData sets commit data from a map
func (d *Deployment) SetCommitData(data map[string]interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	d.CommitData = string(jsonData)
	return nil
}

// IsRollback checks if the deployment is a rollback
func (d *Deployment) IsRollback() bool {
	data := d.GetCommitData()
	_, hasRollbackFrom := data["rollback_from"]
	_, hasRollbackTo := data["rollback_to"]
	return hasRollbackFrom && hasRollbackTo
}

// Certificate represents an SSL certificate for a site
type Certificate struct {
	ID          string          `gorm:"primaryKey;size:26" json:"id"`
	SiteID      string          `gorm:"size:26;not null;index" json:"site_id"`
	Type        CertificateType `gorm:"size:50" json:"type"`
	Domains     string          `gorm:"type:json" json:"domains,omitempty"`
	CSR         *string         `gorm:"type:text" json:"csr,omitempty"`
	PublicKey   *string         `gorm:"type:text" json:"public_key,omitempty"`
	PrivateKey  *string         `gorm:"type:text" json:"-"`
	Certificate *string         `gorm:"type:text" json:"certificate,omitempty"`
	UploadedAt  *time.Time      `json:"uploaded_at,omitempty"`
	IsActive    bool            `gorm:"default:false" json:"is_active"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`

	// Relations
	Site *Site `gorm:"foreignKey:SiteID" json:"site,omitempty"`
}

func (c *Certificate) TableName() string {
	return "certificates"
}

func (c *Certificate) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = utils.NewULID()
	}
	return nil
}

// SiteDirectory returns the directory path for the certificate
func (c *Certificate) SiteDirectory(sitePath string) string {
	return fmt.Sprintf("%s/certificates/%s", sitePath, c.ID)
}

// CertificatePath returns the path to the certificate file
func (c *Certificate) CertificatePath(sitePath string) string {
	return fmt.Sprintf("%s/certificate.cert", c.SiteDirectory(sitePath))
}

// PrivateKeyPath returns the path to the private key file
func (c *Certificate) PrivateKeyPath(sitePath string) string {
	return fmt.Sprintf("%s/private.key", c.SiteDirectory(sitePath))
}

// GetDomains returns domains as a slice
func (c *Certificate) GetDomains() []string {
	if c.Domains == "" || c.Domains == "null" {
		return []string{}
	}
	var domains []string
	if err := json.Unmarshal([]byte(c.Domains), &domains); err != nil {
		return []string{}
	}
	return domains
}

// SetDomains sets domains from a slice
func (c *Certificate) SetDomains(domains []string) error {
	data, err := json.Marshal(domains)
	if err != nil {
		return err
	}
	c.Domains = string(data)
	return nil
}

// Queue represents a supervisor queue worker for a site
type Queue struct {
	ID                      string         `gorm:"primaryKey;size:26" json:"id"`
	SiteID                  string         `gorm:"size:26;not null;index" json:"site_id"`
	ServerID                string         `gorm:"size:26;not null;index" json:"server_id"`
	UserID                  string         `gorm:"size:26;not null;index" json:"user_id"`
	Name                    string         `gorm:"size:255" json:"name"`
	Directory               string         `gorm:"size:500" json:"directory"`
	Command                 string         `gorm:"type:text" json:"command"`
	User                    string         `gorm:"size:100" json:"user"`
	AutoStart               bool           `gorm:"default:true" json:"auto_start"`
	AutoRestart             bool           `gorm:"default:true" json:"auto_restart"`
	NumProcs                int            `gorm:"default:1" json:"numprocs"`
	RedirectStderr          bool           `gorm:"default:true" json:"redirect_stderr"`
	StopWaitSeconds         int            `gorm:"default:10" json:"stop_wait_seconds"`
	StopSignal              string         `gorm:"size:20;default:'TERM'" json:"stop_signal"`
	QueueConnection         string         `gorm:"size:50;default:'database'" json:"queue_connection"`
	Environment             *string        `gorm:"size:100" json:"environment,omitempty"`
	QueueName               string         `gorm:"size:100;default:'default'" json:"queue"`
	MaxSecondsPerJob        int            `gorm:"default:60" json:"max_seconds_per_job"`
	MaxTries                int            `gorm:"default:3" json:"max_tries"`
	RestSecondsOnEmpty      int            `gorm:"default:10" json:"rest_seconds_on_empty"`
	FailedJobDelaySeconds   int            `gorm:"default:3" json:"failed_job_delay_seconds"`
	MaxMemory               int            `gorm:"default:128" json:"max_memory"`
	RunOnMaintenance        bool           `gorm:"default:false" json:"run_on_maintenance"`
	RunWithListen           bool           `gorm:"default:false" json:"run_with_listen"`
	Running                 bool           `gorm:"default:false" json:"running"`
	Info                    string         `gorm:"type:json" json:"info,omitempty"`
	LastStatusCheck         *time.Time     `json:"last_status_check,omitempty"`
	InstalledAt             *time.Time     `json:"installed_at,omitempty"`
	InstallationFailedAt    *time.Time     `json:"installation_failed_at,omitempty"`
	UninstallationRequestedAt *time.Time   `json:"uninstallation_requested_at,omitempty"`
	UninstallationFailedAt  *time.Time     `json:"uninstallation_failed_at,omitempty"`
	CreatedAt               time.Time      `json:"created_at"`
	UpdatedAt               time.Time      `json:"updated_at"`
	DeletedAt               gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Site *Site `gorm:"foreignKey:SiteID" json:"site,omitempty"`
}

func (q *Queue) TableName() string {
	return "queues"
}

func (q *Queue) BeforeCreate(tx *gorm.DB) error {
	if q.ID == "" {
		q.ID = utils.NewULID()
	}
	return nil
}

// GetPath returns the path to the supervisor config file
func (q *Queue) GetPath() string {
	return fmt.Sprintf("/etc/supervisor/conf.d/daemon-%s.conf", q.ID)
}

// ErrorLogPath returns the path to the error log file
func (q *Queue) ErrorLogPath(workingDirectory string) string {
	if q.User == "root" {
		return fmt.Sprintf("/root/%s/daemon-%s.err", workingDirectory, q.ID)
	}
	return fmt.Sprintf("/home/%s/%s/daemon-%s.err", q.User, workingDirectory, q.ID)
}

// OutputLogPath returns the path to the output log file
func (q *Queue) OutputLogPath(workingDirectory string) string {
	if q.User == "root" {
		return fmt.Sprintf("/root/%s/daemon-%s.log", workingDirectory, q.ID)
	}
	return fmt.Sprintf("/home/%s/%s/daemon-%s.log", q.User, workingDirectory, q.ID)
}

// BuildCommand builds the artisan queue command
func (q *Queue) BuildCommand() string {
	run := "work"
	if q.RunWithListen {
		run = "listen"
	}

	cmd := fmt.Sprintf("php artisan queue:%s %s --queue=%s", run, q.QueueConnection, q.QueueName)

	if q.MaxTries > 0 {
		cmd += fmt.Sprintf(" --tries=%d", q.MaxTries)
	}
	if q.Environment != nil && *q.Environment != "" {
		cmd += fmt.Sprintf(" --env=%s", *q.Environment)
	}
	cmd += fmt.Sprintf(" --sleep=%d", q.RestSecondsOnEmpty)
	cmd += fmt.Sprintf(" --timeout=%d", q.MaxSecondsPerJob)
	if q.FailedJobDelaySeconds > 0 {
		cmd += fmt.Sprintf(" --backoff=%d", q.FailedJobDelaySeconds)
	}
	if q.RunOnMaintenance {
		cmd += " --force"
	}
	if q.MaxMemory > 0 {
		cmd += fmt.Sprintf(" --memory=%d", q.MaxMemory)
	}

	return cmd
}

// IsInstalled returns true if the queue is installed
func (q *Queue) IsInstalled() bool {
	return q.InstalledAt != nil
}

// Command represents a command executed on a site
type Command struct {
	ID        string        `gorm:"primaryKey;size:26" json:"id"`
	SiteID    string        `gorm:"size:26;not null;index" json:"site_id"`
	UserID    string        `gorm:"size:26;not null;index" json:"user_id"`
	Command   string        `gorm:"type:text;not null" json:"command"`
	Status    CommandStatus `gorm:"size:50;default:'pending'" json:"status"`
	Output    *string       `gorm:"type:text" json:"output,omitempty"`
	ExitCode  *int          `json:"exit_code,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`

	// Relations
	Site *Site `gorm:"foreignKey:SiteID" json:"site,omitempty"`
}

func (c *Command) TableName() string {
	return "commands"
}

func (c *Command) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = utils.NewULID()
	}
	return nil
}

// Redirect represents an HTTP redirect rule for a site
type Redirect struct {
	ID        string         `gorm:"primaryKey;size:26" json:"id"`
	SiteID    string         `gorm:"size:26;not null;index" json:"site_id"`
	UserID    string         `gorm:"size:26;not null;index" json:"user_id"`
	Mode      RedirectMode   `gorm:"not null" json:"mode"`
	From      string         `gorm:"size:500;not null" json:"from"`
	To        string         `gorm:"size:500;not null" json:"to"`
	Status    string         `gorm:"size:50" json:"status"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Site *Site `gorm:"foreignKey:SiteID" json:"site,omitempty"`
}

func (r *Redirect) TableName() string {
	return "redirects"
}

func (r *Redirect) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = utils.NewULID()
	}
	return nil
}

// Release represents a deployment release for zero-downtime deployments
type Release struct {
	ID         string         `gorm:"primaryKey;size:26" json:"id"`
	SiteID     string         `gorm:"size:26;not null;index" json:"site_id"`
	Path       string         `gorm:"size:500;not null" json:"path"`
	CommitHash *string        `gorm:"size:40" json:"commit_hash,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Site *Site `gorm:"foreignKey:SiteID" json:"site,omitempty"`
}

func (r *Release) TableName() string {
	return "releases"
}

func (r *Release) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = utils.NewULID()
	}
	return nil
}

// Helper functions

func generateRandomToken(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length]
}

// GenerateAppKey generates a Laravel-style application key
func GenerateAppKey() string {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return ""
	}
	return "base64:" + base64.StdEncoding.EncodeToString(key)
}
