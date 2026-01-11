package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0005_06_14_000002_create_database_database_user_table",
		Name:      "Create database_database_user pivot table",
		Timestamp: time.Date(2005, 6, 14, 0, 0, 2, 0, time.UTC),
		Up:        createDatabaseDatabaseUserTableUp,
		Down:      createDatabaseDatabaseUserTableDown,
	})
}

// databaseDatabaseUserMigration model for pivot table
type databaseDatabaseUserMigration struct {
	DatabaseID     string     `gorm:"column:database_id;type:char(26);not null;primaryKey"`
	DatabaseUserID string     `gorm:"column:database_user_id;type:char(26);not null;primaryKey"`
	CreatedAt      *time.Time `gorm:"type:timestamp null"`
	UpdatedAt      *time.Time `gorm:"type:timestamp null"`
}

func (databaseDatabaseUserMigration) TableName() string {
	return "database_database_user"
}

// databaseDatabaseUserWithDatabaseFK defines the database foreign key
type databaseDatabaseUserWithDatabaseFK struct {
	DatabaseID string             `gorm:"column:database_id"`
	Database   *databaseMigration `gorm:"foreignKey:DatabaseID;references:ID;constraint:OnDelete:CASCADE"`
}

func (databaseDatabaseUserWithDatabaseFK) TableName() string {
	return "database_database_user"
}

// databaseDatabaseUserWithUserFK defines the database_user foreign key
type databaseDatabaseUserWithUserFK struct {
	DatabaseUserID string                 `gorm:"column:database_user_id"`
	DatabaseUser   *databaseUserMigration `gorm:"foreignKey:DatabaseUserID;references:ID;constraint:OnDelete:CASCADE"`
}

func (databaseDatabaseUserWithUserFK) TableName() string {
	return "database_database_user"
}

func createDatabaseDatabaseUserTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&databaseDatabaseUserMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&databaseDatabaseUserWithDatabaseFK{}, "Database"); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&databaseDatabaseUserWithUserFK{}, "DatabaseUser"); err != nil {
		return err
	}

	return nil
}

func createDatabaseDatabaseUserTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&databaseDatabaseUserMigration{})
}
