package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0004_09_30_000000_create_posts_table",
		Name:      "Create posts table",
		Timestamp: time.Date(2004, 9, 30, 0, 0, 0, 0, time.UTC),
		Up:        createPostsTableUp,
		Down:      createPostsTableDown,
	})
}

// postMigration model for migration
type postMigration struct {
	ID        uint64     `gorm:"primaryKey;autoIncrement"`
	UserID    string     `gorm:"column:user_id;type:char(26);not null;index"`
	Title     string     `gorm:"type:varchar(255);not null"`
	Slug      string     `gorm:"type:varchar(255);not null"`
	Body      string     `gorm:"type:longtext;not null"`
	Status    string     `gorm:"type:varchar(255);not null;default:draft"` // draft, published
	CreatedAt *time.Time `gorm:"type:timestamp null"`
	UpdatedAt *time.Time `gorm:"type:timestamp null"`
}

func (postMigration) TableName() string {
	return "posts"
}

// postWithUserFK defines the user foreign key
type postWithUserFK struct {
	UserID string         `gorm:"column:user_id"`
	User   *userMigration `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
}

func (postWithUserFK) TableName() string {
	return "posts"
}

func createPostsTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&postMigration{}); err != nil {
		return err
	}

	return migrator.CreateConstraint(&postWithUserFK{}, "User")
}

func createPostsTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&postMigration{})
}
