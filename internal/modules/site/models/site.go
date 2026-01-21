package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	mrand "math/rand"
	"slices"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/site/enums"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/pkg/token"
	"github.com/kkz6/launch-go/internal/pkg/xutil"
)

// Site represents a web application deployed on a server
type Site struct {
	basemodels.BaseModel
	basemodels.InstallableModel
	basemodels.ServerScopedModel
	basemodels.TeamScopedModel
	basemodels.UserScopedModel
	SourceControlID              *string                    `gorm:"column:source_control_id;type:char(26);index" json:"source_control_id,omitempty"`
	Address                      string                     `gorm:"type:varchar(255);not null" json:"address"`
	Type                         enums.SiteType             `gorm:"type:varchar(255);not null;index" json:"type"`
	TypeData                     *string                    `gorm:"column:type_data;type:json" json:"type_data,omitempty"`
	VcsData                      *string                    `gorm:"column:vcs_data;type:json" json:"vcs_data,omitempty"`
	Aliases                      basemodels.JSONStringSlice `gorm:"type:json;serializer:json" json:"aliases,omitempty"`
	TLSSetting                   enums.TLSSetting           `gorm:"column:tls_setting;type:varchar(255);not null;index" json:"tls_setting"`
	ZeroDowntimeDeployment       bool                       `gorm:"column:zero_downtime_deployment" json:"zero_downtime_deployment"`
	DeploymentReleasesRetention  int                        `gorm:"column:deployment_releases_retention;default:10" json:"deployment_releases_retention"`
	AutoDeployment               bool                       `gorm:"column:auto_deployment;default:false" json:"auto_deployment"`
	QueueDeployments             bool                       `gorm:"column:queue_deployments;default:false" json:"queue_deployments"`
	AutoRestartQueue             bool                       `gorm:"column:auto_restart_queue;default:false" json:"auto_restart_queue"`
	Features                     basemodels.JSONStringSlice `gorm:"type:json;serializer:json" json:"features,omitempty"`
	EnabledFeatures              EnabledFeaturesSlice       `gorm:"column:enabled_features;type:json;serializer:json" json:"enabled_features,omitempty"`
	PendingFeatures              basemodels.JSONStringSlice `gorm:"column:pending_features;type:json;serializer:json" json:"pending_features,omitempty"`
	SourceControlRepositoriesID  *uint64                    `gorm:"column:source_control_repositories_id;index" json:"source_control_repositories_id,omitempty"`
	RepositoryBranch             *string                    `gorm:"column:repository_branch;type:varchar(255)" json:"repository_branch,omitempty"`
	DeployToken                  *string                    `gorm:"column:deploy_token;type:varchar(32)" json:"-"`
	DeployNotificationEmail      *string                    `gorm:"column:deploy_notification_email;type:varchar(255)" json:"deploy_notification_email,omitempty"`
	DeployKeyPublic              *string                    `gorm:"column:deploy_key_public;type:longtext" json:"-"`
	DeployKeyPrivate             basemodels.EncryptedString `gorm:"column:deploy_key_private;type:longtext" json:"-"`
	User                         string                     `gorm:"type:varchar(255);not null" json:"user"`
	Path                         string                     `gorm:"type:varchar(255);not null" json:"path"`
	WebFolder                    string                     `gorm:"column:web_folder;type:varchar(255);not null" json:"web_folder"`
	PhpVersion                   *enums.PhpVersion          `gorm:"column:php_version;type:varchar(255);index" json:"php_version,omitempty"`
	PendingTLSUpdateSince        *time.Time                 `gorm:"column:pending_tls_update_since;type:timestamp null" json:"pending_tls_update_since,omitempty"`
	PendingCaddyfileUpdateSince  *time.Time                 `gorm:"column:pending_caddyfile_update_since;type:timestamp null" json:"pending_caddyfile_update_since,omitempty"`
	SharedDirectories            basemodels.JSONStringSlice `gorm:"column:shared_directories;type:json;serializer:json" json:"shared_directories"`
	WriteableDirectories         basemodels.JSONStringSlice `gorm:"column:writeable_directories;type:json;serializer:json" json:"writeable_directories"`
	SharedFiles                  basemodels.JSONStringSlice `gorm:"column:shared_files;type:json;serializer:json" json:"shared_files"`
	Port                         *int                       `gorm:"type:int" json:"port,omitempty"`
	Progress                     *int                       `gorm:"default:0" json:"progress,omitempty"`
	HookBeforeUpdatingRepository *string                    `gorm:"column:hook_before_updating_repository;type:longtext" json:"hook_before_updating_repository,omitempty"`
	HookAfterUpdatingRepository  *string                    `gorm:"column:hook_after_updating_repository;type:longtext" json:"hook_after_updating_repository,omitempty"`
	HookBeforeMakingCurrent      *string                    `gorm:"column:hook_before_making_current;type:longtext" json:"hook_before_making_current,omitempty"`
	HookAfterMakingCurrent       *string                    `gorm:"column:hook_after_making_current;type:longtext" json:"hook_after_making_current,omitempty"`
	ConnectedDomainID            *string                    `gorm:"column:connected_domain_id;type:char(26);index" json:"connected_domain_id,omitempty"`

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
		deployToken := token.New(32).WithEncoding(token.Base64URL).MustGenerate()
		s.DeployToken = &deployToken
	}

	return nil
}

// GetURL returns the full URL for the site
func (s *Site) GetURL() string {
	return fmt.Sprintf("%s://%s", s.TLSSetting.GetProtocol(), s.Address)
}

// GetPort returns the HTTP port based on TLS setting
func (s *Site) GetPort() int {
	if s.Port != nil {
		return *s.Port
	}

	return s.TLSSetting.GetPort()
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

// GetPhpBinary returns the PHP binary path based on the site's PHP version
func (s *Site) GetPhpBinary() string {
	if s.PhpVersion == nil {
		return "php"
	}
	return s.PhpVersion.BinaryPath()
}

// GenerateEnvironmentVariables generates framework-specific environment variables
func (s *Site) GenerateEnvironmentVariables() map[string]string {
	variables := make(map[string]string)

	switch s.Type {
	case enums.SiteTypeLaravel:
		variables["APP_KEY"] = xutil.GenerateAppKey()
		variables["APP_URL"] = s.GetURL()

	case enums.SiteTypeWordpress:
		wpSaltKeys := []string{
			"AUTH_KEY", "AUTH_SALT", "LOGGED_IN_KEY", "LOGGED_IN_SALT",
			"NONCE_KEY", "NONCE_SALT", "SECURE_AUTH_KEY", "SECURE_AUTH_SALT",
		}
		for _, key := range wpSaltKeys {
			variables[key] = escapeWordpressSpecialChars(generateWordpressKey())
		}
	}

	return variables
}

// generateWordpressKey generates a random key for WordPress
func generateWordpressKey() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()-_ []{}<>~`+=,.;:/?|"
	key := make([]byte, 64)
	for i := range key {
		key[i] = charset[mrand.Intn(len(charset))]
	}
	return string(key)
}

// escapeWordpressSpecialChars escapes special characters for WordPress config
func escapeWordpressSpecialChars(s string) string {
	s = strings.ReplaceAll(s, "&", "\\&")
	s = strings.ReplaceAll(s, "!", "\\!")
	s = strings.ReplaceAll(s, "$", "\\$")
	return s
}

// EnabledFeature represents an enabled Laravel feature with metadata
type EnabledFeature struct {
	Name      string     `json:"name"`
	QueueID   *string    `json:"queue_id,omitempty"`
	CronID    *string    `json:"cron_id,omitempty"`
	EnabledAt *time.Time `json:"enabled_at,omitempty"`
}

// EnabledFeaturesSlice is a slice of EnabledFeature that handles JSON serialization
type EnabledFeaturesSlice []EnabledFeature

// Scan implements sql.Scanner for EnabledFeaturesSlice
func (e *EnabledFeaturesSlice) Scan(value interface{}) error {
	if value == nil {
		*e = nil
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("failed to scan EnabledFeaturesSlice: %T", value)
	}

	if len(bytes) == 0 || string(bytes) == "null" {
		*e = nil
		return nil
	}

	return json.Unmarshal(bytes, e)
}

// Value implements driver.Valuer for EnabledFeaturesSlice
func (e EnabledFeaturesSlice) Value() (driver.Value, error) {
	if e == nil {
		return nil, nil
	}
	return json.Marshal(e)
}

// HasEnabledFeature checks if a specific feature is enabled
func (s *Site) HasEnabledFeature(featureName string) bool {
	for _, f := range s.EnabledFeatures {
		if f.Name == featureName {
			return true
		}
	}
	return false
}

// GetEnabledFeature returns the enabled feature data if found
func (s *Site) GetEnabledFeature(featureName string) *EnabledFeature {
	for _, f := range s.EnabledFeatures {
		if f.Name == featureName {
			return &f
		}
	}
	return nil
}

// HasPendingFeature checks if a feature is pending enable/disable
func (s *Site) HasPendingFeature(featureName string) bool {
	return slices.Contains(s.PendingFeatures, featureName)
}

// AddEnabledFeature adds a feature to the enabled features list
func (s *Site) AddEnabledFeature(feature EnabledFeature) {
	// Remove if already exists
	s.RemoveEnabledFeature(feature.Name)
	s.EnabledFeatures = append(s.EnabledFeatures, feature)
}

// RemoveEnabledFeature removes a feature from the enabled features list
func (s *Site) RemoveEnabledFeature(featureName string) {
	result := make([]EnabledFeature, 0, len(s.EnabledFeatures))
	for _, f := range s.EnabledFeatures {
		if f.Name != featureName {
			result = append(result, f)
		}
	}
	s.EnabledFeatures = result
}

// AddPendingFeature adds a feature to the pending features list
func (s *Site) AddPendingFeature(featureName string) {
	if !s.HasPendingFeature(featureName) {
		s.PendingFeatures = append(s.PendingFeatures, featureName)
	}
}

// RemovePendingFeature removes a feature from the pending features list
func (s *Site) RemovePendingFeature(featureName string) {
	result := make([]string, 0, len(s.PendingFeatures))
	for _, f := range s.PendingFeatures {
		if f != featureName {
			result = append(result, f)
		}
	}
	s.PendingFeatures = result
}
