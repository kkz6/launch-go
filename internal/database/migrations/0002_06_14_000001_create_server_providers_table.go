package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0002_06_14_000001_create_server_providers_table",
		Name:      "Create server_providers table",
		Timestamp: time.Date(2002, 6, 14, 0, 0, 1, 0, time.UTC),
		Up:        createServerProvidersTableUp,
		Down:      createServerProvidersTableDown,
	})
}

// serverProviderMigration model for migration (matches Laravel schema)
type serverProviderMigration struct {
	ID          string     `gorm:"type:char(26);primaryKey"`
	UserID      string     `gorm:"column:user_id;type:char(26);not null;index"`
	TeamID      *string    `gorm:"column:team_id;type:char(26);index"`
	Profile     *string    `gorm:"type:varchar(255)"`
	Provider    string     `gorm:"type:varchar(255);not null"`
	Credentials string     `gorm:"type:longtext;not null"`
	Connected   bool       `gorm:"type:tinyint(1);not null;default:1"`
	CreatedAt   *time.Time `gorm:"type:timestamp null"`
	UpdatedAt   *time.Time `gorm:"type:timestamp null"`
}

func (serverProviderMigration) TableName() string {
	return "server_providers"
}

// serverProviderWithFK defines foreign key relationships
type serverProviderWithFK struct {
	UserID string         `gorm:"column:user_id"`
	TeamID *string        `gorm:"column:team_id"`
	User   *userMigration `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
	Team   *teamMigration `gorm:"foreignKey:TeamID;references:ID;constraint:OnDelete:CASCADE"`
}

func (serverProviderWithFK) TableName() string {
	return "server_providers"
}

func createServerProvidersTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&serverProviderMigration{}); err != nil {
		return err
	}

	// Add foreign key constraint for user_id -> users.id
	if err := migrator.CreateConstraint(&serverProviderWithFK{}, "User"); err != nil {
		return err
	}

	// Add foreign key constraint for team_id -> teams.id
	return migrator.CreateConstraint(&serverProviderWithFK{}, "Team")
}

func createServerProvidersTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&serverProviderMigration{})
}
