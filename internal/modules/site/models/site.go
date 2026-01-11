package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// Site represents a web application deployed on a server
type Site struct {
	ID                            string           `gorm:"primaryKey;size:26" json:"id"`
	ServerID                      string           `gorm:"size:26;not null;index" json:"server_id"`
	UserID                        string           `gorm:"size:26;not null;index" json:"user_id"`
	SourceControlID               *string          `gorm:"size:26;index" json:"source_control_id,omitempty"`
	SourceControlRepositoriesID   *string          `gorm:"size:26;index" json:"source_control_repositories_id,omitempty"`
	ConnectedDomainID             *string          `gorm:"size:26;index" json:"connected_domain_id,omitempty"`
	Address                       string           `gorm:"size:255;not null" json:"address"`
	Type                          enums.SiteType   `gorm:"size:50;not null;default:'laravel'" json:"type"`
	TypeData                      string           `gorm:"type:json" json:"type_data,omitempty"`
	VcsData                       string           `gorm:"type:json" json:"vcs_data,omitempty"`
	Aliases                       string           `gorm:"type:json" json:"aliases,omitempty"`
	TlsSetting                    enums.TlsSetting `gorm:"size:50;default:'auto'" json:"tls_setting"`
	PendingTlsUpdateSince         *time.Time       `json:"pending_tls_update_since,omitempty"`
	ZeroDowntimeDeployment        bool             `gorm:"default:true" json:"zero_downtime_deployment"`
	DeploymentReleasesRetention   int              `gorm:"default:5" json:"deployment_releases_retention"`
	RepositoryBranch              string           `gorm:"size:255;default:'main'" json:"repository_branch"`
	DeployToken                   string           `gorm:"size:64" json:"-"`
	DeployNotificationEmail       *string          `gorm:"size:255" json:"deploy_notification_email,omitempty"`
	User                          string           `gorm:"size:100" json:"user"`
	Path                          string           `gorm:"size:500" json:"path"`
	WebFolder                     string           `gorm:"size:255;default:'public'" json:"web_folder"`
	PhpVersion                    string           `gorm:"size:10" json:"php_version"`
	PendingCaddyfileUpdateSince   *time.Time       `json:"pending_caddyfile_update_since,omitempty"`
	SharedDirectories             string           `gorm:"type:json" json:"shared_directories,omitempty"`
	WriteableDirectories          string           `gorm:"type:json" json:"writeable_directories,omitempty"`
	SharedFiles                   string           `gorm:"type:json" json:"shared_files,omitempty"`
	Progress                      *string          `gorm:"size:255" json:"progress,omitempty"`
	AutoDeployment                bool             `gorm:"default:false" json:"auto_deployment"`
	QueueDeployments              bool             `gorm:"default:false" json:"queue_deployments"`
	AutoRestartQueue              bool             `gorm:"default:false" json:"auto_restart_queue"`
	HookBeforeUpdatingRepository  string           `gorm:"type:text" json:"hook_before_updating_repository,omitempty"`
	HookAfterUpdatingRepository   string           `gorm:"type:text" json:"hook_after_updating_repository,omitempty"`
	HookBeforeMakingCurrent       string           `gorm:"type:text" json:"hook_before_making_current,omitempty"`
	HookAfterMakingCurrent        string           `gorm:"type:text" json:"hook_after_making_current,omitempty"`
	InstalledAt                   *time.Time       `json:"installed_at,omitempty"`
	InstallationFailedAt          *time.Time       `json:"installation_failed_at,omitempty"`
	UninstallationRequestedAt     *time.Time       `json:"uninstallation_requested_at,omitempty"`
	UninstallationFailedAt        *time.Time       `json:"uninstallation_failed_at,omitempty"`
	Features                      string           `gorm:"type:json" json:"features,omitempty"`
	EnabledFeatures               string           `gorm:"type:json" json:"enabled_features,omitempty"`
	PendingFeatures               string           `gorm:"type:json" json:"pending_features,omitempty"`
	CreatedAt                     time.Time        `json:"created_at"`
	UpdatedAt                     time.Time        `json:"updated_at"`
	DeletedAt                     gorm.DeletedAt   `gorm:"index" json:"-"`

	// Relations
	Deployments      []Deployment  `gorm:"foreignKey:SiteID" json:"deployments,omitempty"`
	LatestDeployment *Deployment   `gorm:"-" json:"latest_deployment,omitempty"`
	Certificates     []Certificate `gorm:"foreignKey:SiteID" json:"certificates,omitempty"`
	Queues           []Queue       `gorm:"foreignKey:SiteID" json:"queues,omitempty"`
	Commands         []Command     `gorm:"foreignKey:SiteID" json:"commands,omitempty"`
	Redirects        []Redirect    `gorm:"foreignKey:SiteID" json:"redirects,omitempty"`
}

func (s *Site) TableName() string {
	return "sites"
}

func (s *Site) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = utils.NewULID()
	}

	if s.DeployToken == "" {
		s.DeployToken = GenerateRandomToken(32)
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
