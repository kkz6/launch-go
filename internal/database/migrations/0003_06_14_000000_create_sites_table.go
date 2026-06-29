package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0003_06_14_000000_create_sites_table",
		Name:      "Create sites table",
		Timestamp: time.Date(2003, 6, 14, 0, 0, 0, 0, time.UTC),
		Up:        createSitesTableUp,
	})
}

// siteMigration model for migration (matches Laravel schema with all fields merged)
type siteMigration struct {
	ID                           string     `gorm:"type:char(26);primaryKey"`
	ServerID                     string     `gorm:"column:server_id;type:char(26);not null;index"`
	UserID                       string     `gorm:"column:user_id;type:char(26);not null;index"`
	SourceControlID              *string    `gorm:"column:source_control_id;type:char(26);index"`
	Address                      string     `gorm:"type:varchar(255);not null"`
	Type                         string     `gorm:"type:varchar(255);not null;index"`
	TypeData                     *string    `gorm:"column:type_data;type:jsonb"`
	VCSData                      *string    `gorm:"column:vcs_data;type:jsonb"`
	Aliases                      *string    `gorm:"type:jsonb"`
	TLSSetting                   string     `gorm:"column:tls_setting;type:varchar(255);not null;index"`
	ZeroDowntimeDeployment       bool       `gorm:"column:zero_downtime_deployment;default:true"`
	DeploymentReleasesRetention  int        `gorm:"column:deployment_releases_retention;default:10"`
	AutoDeployment               bool       `gorm:"column:auto_deployment;default:false"`
	QueueDeployments             bool       `gorm:"column:queue_deployments;default:false"`
	AutoRestartQueue             bool       `gorm:"column:auto_restart_queue;default:false"`
	Features                     *string    `gorm:"type:jsonb"`
	SourceControlRepositoriesID  *uint64    `gorm:"column:source_control_repositories_id;index"`
	RepositoryBranch             *string    `gorm:"column:repository_branch;type:varchar(255)"`
	DeployToken                  *string    `gorm:"column:deploy_token;type:varchar(32)"`
	DeployNotificationEmail      *string    `gorm:"column:deploy_notification_email;type:varchar(255)"`
	DeployKeyPublic              *string    `gorm:"column:deploy_key_public;type:text"`
	DeployKeyPrivate             *string    `gorm:"column:deploy_key_private;type:text"`
	User                         string     `gorm:"type:varchar(255);not null"`
	Path                         string     `gorm:"type:varchar(255);not null"`
	WebFolder                    string     `gorm:"column:web_folder;type:varchar(255);not null"`
	PHPVersion                   *string    `gorm:"column:php_version;type:varchar(255);index"`
	PendingTLSUpdateSince        *time.Time `gorm:"column:pending_tls_update_since;type:timestamp null"`
	PendingCaddyfileUpdateSince  *time.Time `gorm:"column:pending_caddyfile_update_since;type:timestamp null"`
	SharedDirectories            string     `gorm:"column:shared_directories;type:jsonb;not null"`
	WriteableDirectories         string     `gorm:"column:writeable_directories;type:jsonb;not null"`
	SharedFiles                  string     `gorm:"column:shared_files;type:jsonb;not null"`
	Port                         *int       `gorm:"type:int"`
	Progress                     *int       `gorm:"default:0"`
	HookBeforeUpdatingRepository *string    `gorm:"column:hook_before_updating_repository;type:text"`
	HookAfterUpdatingRepository  *string    `gorm:"column:hook_after_updating_repository;type:text"`
	HookBeforeMakingCurrent      *string    `gorm:"column:hook_before_making_current;type:text"`
	HookAfterMakingCurrent       *string    `gorm:"column:hook_after_making_current;type:text"`
	InstalledAt                  *time.Time `gorm:"column:installed_at;type:timestamp null"`
	InstallationFailedAt         *time.Time `gorm:"column:installation_failed_at;type:timestamp null"`
	UninstallationRequestedAt    *time.Time `gorm:"column:uninstallation_requested_at;type:timestamp null"`
	UninstallationFailedAt       *time.Time `gorm:"column:uninstallation_failed_at;type:timestamp null"`
	CreatedAt                    *time.Time `gorm:"type:timestamp null"`
	UpdatedAt                    *time.Time `gorm:"type:timestamp null"`
}

func (siteMigration) TableName() string {
	return "sites"
}

// siteWithServerFK defines the server foreign key
type siteWithServerFK struct {
	ServerID string           `gorm:"column:server_id"`
	Server   *serverMigration `gorm:"foreignKey:ServerID;references:ID;constraint:OnDelete:CASCADE"`
}

func (siteWithServerFK) TableName() string {
	return "sites"
}

// siteWithUserFK defines the user foreign key
type siteWithUserFK struct {
	UserID string         `gorm:"column:user_id"`
	User   *userMigration `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
}

func (siteWithUserFK) TableName() string {
	return "sites"
}

// siteWithSourceControlFK defines the source control foreign key
type siteWithSourceControlFK struct {
	SourceControlID *string                 `gorm:"column:source_control_id"`
	SourceControl   *sourceControlMigration `gorm:"foreignKey:SourceControlID;references:ID;constraint:OnDelete:SET NULL"`
}

func (siteWithSourceControlFK) TableName() string {
	return "sites"
}

// siteWithRepoFK defines the source control repositories foreign key
type siteWithRepoFK struct {
	SourceControlRepositoriesID *uint64                           `gorm:"column:source_control_repositories_id"`
	SourceControlRepositories   *sourceControlRepositoryMigration `gorm:"foreignKey:SourceControlRepositoriesID;references:ID;constraint:OnDelete:SET NULL"`
}

func (siteWithRepoFK) TableName() string {
	return "sites"
}

// cronWithSiteFK defines the site foreign key for crons table (deferred from crons migration)
type cronWithSiteFK struct {
	SiteID *string        `gorm:"column:site_id"`
	Site   *siteMigration `gorm:"foreignKey:SiteID;references:ID;constraint:OnDelete:CASCADE"`
}

func (cronWithSiteFK) TableName() string {
	return "crons"
}

func createSitesTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&siteMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&siteWithServerFK{}, "Server"); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&siteWithUserFK{}, "User"); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&siteWithSourceControlFK{}, "SourceControl"); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&siteWithRepoFK{}, "SourceControlRepositories"); err != nil {
		return err
	}

	// Add FK constraint for crons.site_id -> sites.id (deferred from crons migration)
	return migrator.CreateConstraint(&cronWithSiteFK{}, "Site")
}
