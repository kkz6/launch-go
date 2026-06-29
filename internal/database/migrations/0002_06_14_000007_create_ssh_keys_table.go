package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0002_06_14_000007_create_ssh_keys_table",
		Name:      "Create ssh_keys table",
		Timestamp: time.Date(2002, 6, 14, 0, 0, 7, 0, time.UTC),
		Up:        createSSHKeysTableUp,
	})
}

// sshKeyMigration model for migration (matches Laravel schema)
type sshKeyMigration struct {
	ID          string     `gorm:"type:char(26);primaryKey"`
	UserID      string     `gorm:"column:user_id;type:char(26);not null;index"`
	TeamID      string     `gorm:"column:team_id;type:char(26);not null;index"`
	IsGlobal    bool       `gorm:"type:boolean;not null;default:0"`
	Description *string    `gorm:"type:varchar(255)"`
	PublicKey   string     `gorm:"type:text;not null"`
	Name        string     `gorm:"type:varchar(255);not null"`
	Fingerprint *string    `gorm:"type:varchar(255)"`
	CreatedAt   *time.Time `gorm:"type:timestamp null"`
	UpdatedAt   *time.Time `gorm:"type:timestamp null"`
}

func (sshKeyMigration) TableName() string {
	return "ssh_keys"
}

// sshKeyWithFK defines foreign key relationships
type sshKeyWithFK struct {
	UserID string         `gorm:"column:user_id"`
	TeamID string         `gorm:"column:team_id"`
	User   *userMigration `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
	Team   *teamMigration `gorm:"foreignKey:TeamID;references:ID;constraint:OnDelete:CASCADE"`
}

func (sshKeyWithFK) TableName() string {
	return "ssh_keys"
}

func createSSHKeysTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&sshKeyMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&sshKeyWithFK{}, "User"); err != nil {
		return err
	}

	return migrator.CreateConstraint(&sshKeyWithFK{}, "Team")
}
