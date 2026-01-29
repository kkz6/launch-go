package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0003_06_14_000001_create_deployments_table",
		Name:      "Create deployments table",
		Timestamp: time.Date(2003, 6, 14, 0, 0, 1, 0, time.UTC),
		Up:        createDeploymentsTableUp,
		Down:      createDeploymentsTableDown,
	})
}

// deploymentMigration model for migration
type deploymentMigration struct {
	ID             string     `gorm:"type:char(26);primaryKey"`
	SiteID         string     `gorm:"column:site_id;type:char(26);not null;index"`
	UserID         *string    `gorm:"column:user_id;type:char(26);index"`
	TaskID         *string    `gorm:"column:task_id;type:char(26);index"`
	Status         string     `gorm:"type:varchar(255);not null"`
	GitHash        *string    `gorm:"column:git_hash;type:varchar(255)"`
	CommitData     *string    `gorm:"column:commit_data;type:json"`
	VCSData        *string    `gorm:"column:vcs_data;type:json"`
	UserNotifiedAt *time.Time `gorm:"column:user_notified_at;type:timestamp null"`
	CreatedAt      *time.Time `gorm:"type:timestamp null"`
	UpdatedAt      *time.Time `gorm:"type:timestamp null"`
}

func (deploymentMigration) TableName() string {
	return "deployments"
}

// deploymentWithSiteFK defines the site foreign key
type deploymentWithSiteFK struct {
	SiteID string         `gorm:"column:site_id"`
	Site   *siteMigration `gorm:"foreignKey:SiteID;references:ID;constraint:OnDelete:CASCADE"`
}

func (deploymentWithSiteFK) TableName() string {
	return "deployments"
}

// deploymentWithUserFK defines the user foreign key (nullable, null on delete)
type deploymentWithUserFK struct {
	UserID *string        `gorm:"column:user_id"`
	User   *userMigration `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:SET NULL"`
}

func (deploymentWithUserFK) TableName() string {
	return "deployments"
}

// deploymentWithTaskFK defines the task foreign key (nullable, null on delete)
type deploymentWithTaskFK struct {
	TaskID *string        `gorm:"column:task_id"`
	Task   *taskMigration `gorm:"foreignKey:TaskID;references:ID;constraint:OnDelete:SET NULL"`
}

func (deploymentWithTaskFK) TableName() string {
	return "deployments"
}

func createDeploymentsTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&deploymentMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&deploymentWithSiteFK{}, "Site"); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&deploymentWithUserFK{}, "User"); err != nil {
		return err
	}

	return migrator.CreateConstraint(&deploymentWithTaskFK{}, "Task")
}

func createDeploymentsTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&deploymentMigration{})
}
