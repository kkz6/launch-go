package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0003_06_14_000003_create_redirects_table",
		Name:      "Create redirects table",
		Timestamp: time.Date(2003, 6, 14, 0, 0, 3, 0, time.UTC),
		Up:        createRedirectsTableUp,
		Down:      createRedirectsTableDown,
	})
}

// redirectMigration model for migration
type redirectMigration struct {
	ID        string     `gorm:"type:char(26);primaryKey"`
	SiteID    string     `gorm:"column:site_id;type:char(26);not null;index"`
	UserID    string     `gorm:"column:user_id;type:char(26);not null;index"`
	Mode      int        `gorm:"type:int;not null"`
	From      string     `gorm:"type:text;not null"`
	To        string     `gorm:"type:text;not null"`
	Status    string     `gorm:"type:varchar(255);not null;default:creating"`
	CreatedAt *time.Time `gorm:"type:timestamp null"`
	UpdatedAt *time.Time `gorm:"type:timestamp null"`
}

func (redirectMigration) TableName() string {
	return "redirects"
}

// redirectWithSiteFK defines the site foreign key
type redirectWithSiteFK struct {
	SiteID string         `gorm:"column:site_id"`
	Site   *siteMigration `gorm:"foreignKey:SiteID;references:ID;constraint:OnDelete:CASCADE"`
}

func (redirectWithSiteFK) TableName() string {
	return "redirects"
}

// redirectWithUserFK defines the user foreign key
type redirectWithUserFK struct {
	UserID string         `gorm:"column:user_id"`
	User   *userMigration `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
}

func (redirectWithUserFK) TableName() string {
	return "redirects"
}

func createRedirectsTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&redirectMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&redirectWithSiteFK{}, "Site"); err != nil {
		return err
	}

	return migrator.CreateConstraint(&redirectWithUserFK{}, "User")
}

func createRedirectsTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&redirectMigration{})
}
