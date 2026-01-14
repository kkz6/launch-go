package models

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/site/enums"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// Site represents a web application deployed on a server
type Site struct {
	basemodels.BaseModel
	basemodels.InstallableModel
	ServerID                     string           `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
	UserID                       string           `gorm:"column:user_id;type:char(26);not null;index" json:"user_id"`
	SourceControlID              *string          `gorm:"column:source_control_id;type:char(26);index" json:"source_control_id,omitempty"`
	Address                      string           `gorm:"type:varchar(255);not null" json:"address"`
	Type                         enums.SiteType   `gorm:"type:varchar(255);not null;index" json:"type"`
	TypeData                     *string          `gorm:"column:type_data;type:json" json:"type_data,omitempty"`
	VcsData                      *string          `gorm:"column:vcs_data;type:json" json:"vcs_data,omitempty"`
	Aliases                      basemodels.JSONStringSlice `gorm:"type:json" json:"aliases,omitempty"`
	TlsSetting                   enums.TlsSetting           `gorm:"column:tls_setting;type:varchar(255);not null;index" json:"tls_setting"`
	ZeroDowntimeDeployment       bool             `gorm:"column:zero_downtime_deployment" json:"zero_downtime_deployment"`
	DeploymentReleasesRetention  int              `gorm:"column:deployment_releases_retention;default:10" json:"deployment_releases_retention"`
	AutoDeployment               bool             `gorm:"column:auto_deployment;default:false" json:"auto_deployment"`
	QueueDeployments             bool             `gorm:"column:queue_deployments;default:false" json:"queue_deployments"`
	AutoRestartQueue             bool             `gorm:"column:auto_restart_queue;default:false" json:"auto_restart_queue"`
	Features                     basemodels.JSONStringSlice `gorm:"type:json" json:"features,omitempty"`
	SourceControlRepositoriesID  *uint64          `gorm:"column:source_control_repositories_id;index" json:"source_control_repositories_id,omitempty"`
	RepositoryBranch             *string          `gorm:"column:repository_branch;type:varchar(255)" json:"repository_branch,omitempty"`
	DeployToken                  *string          `gorm:"column:deploy_token;type:varchar(32)" json:"-"`
	DeployNotificationEmail      *string          `gorm:"column:deploy_notification_email;type:varchar(255)" json:"deploy_notification_email,omitempty"`
	DeployKeyPublic              *string                    `gorm:"column:deploy_key_public;type:longtext" json:"-"`
	DeployKeyPrivate             basemodels.EncryptedString `gorm:"column:deploy_key_private;type:longtext" json:"-"`
	User                         string           `gorm:"type:varchar(255);not null" json:"user"`
	Path                         string           `gorm:"type:varchar(255);not null" json:"path"`
	WebFolder                    string           `gorm:"column:web_folder;type:varchar(255);not null" json:"web_folder"`
	PhpVersion                   *string          `gorm:"column:php_version;type:varchar(255);index" json:"php_version,omitempty"`
	PendingTlsUpdateSince        *time.Time       `gorm:"column:pending_tls_update_since;type:timestamp null" json:"pending_tls_update_since,omitempty"`
	PendingCaddyfileUpdateSince  *time.Time       `gorm:"column:pending_caddyfile_update_since;type:timestamp null" json:"pending_caddyfile_update_since,omitempty"`
	SharedDirectories            basemodels.JSONStringSlice `gorm:"column:shared_directories;type:json" json:"shared_directories"`
	WriteableDirectories         basemodels.JSONStringSlice `gorm:"column:writeable_directories;type:json" json:"writeable_directories"`
	SharedFiles                  basemodels.JSONStringSlice `gorm:"column:shared_files;type:json" json:"shared_files"`
	Port                         *int             `gorm:"type:int" json:"port,omitempty"`
	Progress                     *int             `gorm:"default:0" json:"progress,omitempty"`
	HookBeforeUpdatingRepository *string          `gorm:"column:hook_before_updating_repository;type:longtext" json:"hook_before_updating_repository,omitempty"`
	HookAfterUpdatingRepository  *string          `gorm:"column:hook_after_updating_repository;type:longtext" json:"hook_after_updating_repository,omitempty"`
	HookBeforeMakingCurrent      *string          `gorm:"column:hook_before_making_current;type:longtext" json:"hook_before_making_current,omitempty"`
	HookAfterMakingCurrent       *string          `gorm:"column:hook_after_making_current;type:longtext" json:"hook_after_making_current,omitempty"`

	// Relations
	Deployments      []Deployment  `gorm:"foreignKey:SiteID;references:ID" json:"deployments,omitempty"`
	LatestDeployment *Deployment   `gorm:"-" json:"latest_deployment,omitempty"`
	Certificates     []Certificate `gorm:"foreignKey:SiteID;references:ID" json:"certificates,omitempty"`
	Queues           []Queue       `gorm:"foreignKey:SiteID;references:ID" json:"queues,omitempty"`
	Commands         []Command     `gorm:"foreignKey:SiteID;references:ID" json:"commands,omitempty"`
	Redirects        []Redirect    `gorm:"foreignKey:SiteID;references:ID" json:"redirects,omitempty"`
}

func (Site) TableName() string {
	return "sites"
}

func (s *Site) BeforeCreate(tx *gorm.DB) error {
	if err := s.BaseModel.BeforeCreate(tx); err != nil {
		return err
	}

	if s.DeployToken == nil || *s.DeployToken == "" {
		token := GenerateRandomToken(32)
		s.DeployToken = &token
	}

	return nil
}

// GetURL returns the full URL for the site
func (s *Site) GetURL() string {
	return fmt.Sprintf("%s://%s", s.TlsSetting.GetProtocol(), s.Address)
}

// GetPort returns the HTTP port based on TLS setting
func (s *Site) GetPort() int {
	if s.Port != nil {
		return *s.Port
	}

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

// HasFeature checks if the site has a specific feature
func (s *Site) HasFeature(feature string) bool {
	return slices.Contains(s.Features, feature)
}

// GetRepositoryBranch returns the repository branch or default
func (s *Site) GetRepositoryBranch() string {
	if s.RepositoryBranch != nil && *s.RepositoryBranch != "" {
		return *s.RepositoryBranch
	}

	return "main"
}
