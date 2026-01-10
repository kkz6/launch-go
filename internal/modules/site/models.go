package site

import (
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

type SiteStatus string

const (
	SiteStatusPending     SiteStatus = "pending"
	SiteStatusProvisioning SiteStatus = "provisioning"
	SiteStatusActive      SiteStatus = "active"
	SiteStatusFailed      SiteStatus = "failed"
	SiteStatusDeploying   SiteStatus = "deploying"
)

type SiteType string

const (
	SiteTypeLaravel    SiteType = "laravel"
	SiteTypeWordpress  SiteType = "wordpress"
	SiteTypeStatic     SiteType = "static"
	SiteTypeGeneric    SiteType = "generic"
)

type Site struct {
	ID                  string         `gorm:"primaryKey;size:26" json:"id"`
	TeamID              string         `gorm:"size:26;not null;index" json:"team_id"`
	ServerID            string         `gorm:"size:26;not null;index" json:"server_id"`
	Name                string         `gorm:"size:255;not null" json:"name"`
	Domain              string         `gorm:"size:255;not null" json:"domain"`
	Aliases             string         `gorm:"type:json" json:"aliases,omitempty"`
	Type                SiteType       `gorm:"size:50;default:'laravel'" json:"type"`
	Status              SiteStatus     `gorm:"size:50;default:'pending'" json:"status"`
	Path                string         `gorm:"size:255" json:"path"`
	PublicPath          string         `gorm:"size:255;default:'public'" json:"public_path"`
	PHPVersion          string         `gorm:"size:10" json:"php_version"`
	RepositoryProvider  *string        `gorm:"size:50" json:"repository_provider,omitempty"`
	RepositoryURL       *string        `gorm:"size:500" json:"repository_url,omitempty"`
	RepositoryBranch    string         `gorm:"size:255;default:'main'" json:"repository_branch"`
	DeploymentScript    string         `gorm:"type:text" json:"deployment_script,omitempty"`
	EnvironmentVars     string         `gorm:"type:text" json:"-"`
	SSLEnabled          bool           `gorm:"default:false" json:"ssl_enabled"`
	SSLCertificatePath  *string        `gorm:"size:500" json:"ssl_certificate_path,omitempty"`
	SSLPrivateKeyPath   *string        `gorm:"size:500" json:"ssl_private_key_path,omitempty"`
	ZeroDowntime        bool           `gorm:"default:true" json:"zero_downtime"`
	ReleasesToKeep      int            `gorm:"default:5" json:"releases_to_keep"`
	CurrentRelease      *string        `gorm:"size:26" json:"current_release,omitempty"`
	QueueEnabled        bool           `gorm:"default:false" json:"queue_enabled"`
	SchedulerEnabled    bool           `gorm:"default:false" json:"scheduler_enabled"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	DeletedAt           gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Deployments []Deployment `gorm:"foreignKey:SiteID" json:"deployments,omitempty"`
	Releases    []Release    `gorm:"foreignKey:SiteID" json:"releases,omitempty"`
}

func (s *Site) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = utils.NewULID()
	}
	return nil
}

type DeploymentStatus string

const (
	DeploymentStatusPending   DeploymentStatus = "pending"
	DeploymentStatusRunning   DeploymentStatus = "running"
	DeploymentStatusSucceeded DeploymentStatus = "succeeded"
	DeploymentStatusFailed    DeploymentStatus = "failed"
	DeploymentStatusCancelled DeploymentStatus = "cancelled"
)

type Deployment struct {
	ID           string           `gorm:"primaryKey;size:26" json:"id"`
	SiteID       string           `gorm:"size:26;not null;index" json:"site_id"`
	ReleaseID    *string          `gorm:"size:26" json:"release_id,omitempty"`
	UserID       string           `gorm:"size:26;not null" json:"user_id"`
	Status       DeploymentStatus `gorm:"size:50;default:'pending'" json:"status"`
	CommitHash   *string          `gorm:"size:40" json:"commit_hash,omitempty"`
	CommitMessage *string         `gorm:"size:500" json:"commit_message,omitempty"`
	Branch       string           `gorm:"size:255" json:"branch"`
	Log          string           `gorm:"type:text" json:"log,omitempty"`
	StartedAt    *time.Time       `json:"started_at,omitempty"`
	FinishedAt   *time.Time       `json:"finished_at,omitempty"`
	Duration     *int             `json:"duration,omitempty"`
	IsRollback   bool             `gorm:"default:false" json:"is_rollback"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`

	// Relations
	Release *Release `gorm:"foreignKey:ReleaseID" json:"release,omitempty"`
}

func (d *Deployment) BeforeCreate(tx *gorm.DB) error {
	if d.ID == "" {
		d.ID = utils.NewULID()
	}
	return nil
}

type Release struct {
	ID         string         `gorm:"primaryKey;size:26" json:"id"`
	SiteID     string         `gorm:"size:26;not null;index" json:"site_id"`
	Path       string         `gorm:"size:500;not null" json:"path"`
	CommitHash *string        `gorm:"size:40" json:"commit_hash,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

func (r *Release) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = utils.NewULID()
	}
	return nil
}

type EnvironmentVariable struct {
	ID        string         `gorm:"primaryKey;size:26" json:"id"`
	SiteID    string         `gorm:"size:26;not null;index" json:"site_id"`
	Key       string         `gorm:"size:255;not null" json:"key"`
	Value     string         `gorm:"type:text" json:"value"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (e *EnvironmentVariable) BeforeCreate(tx *gorm.DB) error {
	if e.ID == "" {
		e.ID = utils.NewULID()
	}
	return nil
}
