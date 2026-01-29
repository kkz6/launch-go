package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0002_06_14_000008_create_server_ssh_keys_table",
		Name:      "Create server_ssh_keys pivot table",
		Timestamp: time.Date(2002, 6, 14, 0, 0, 8, 0, time.UTC),
		Up:        createServerSSHKeysTableUp,
		Down:      createServerSSHKeysTableDown,
	})
}

// serverSSHKeyMigration model for migration (pivot table, matches Laravel schema)
type serverSSHKeyMigration struct {
	ServerID  string     `gorm:"column:server_id;type:char(26);not null;primaryKey"`
	SSHKeyID  string     `gorm:"column:ssh_key_id;type:char(26);not null;primaryKey"`
	CreatedAt *time.Time `gorm:"type:timestamp null"`
	UpdatedAt *time.Time `gorm:"type:timestamp null"`
}

func (serverSSHKeyMigration) TableName() string {
	return "server_ssh_keys"
}

// serverSSHKeyWithFK defines foreign key relationships
type serverSSHKeyWithFK struct {
	ServerID string           `gorm:"column:server_id"`
	SSHKeyID string           `gorm:"column:ssh_key_id"`
	Server   *serverMigration `gorm:"foreignKey:ServerID;references:ID;constraint:OnDelete:CASCADE"`
	SSHKey   *sshKeyMigration `gorm:"foreignKey:SSHKeyID;references:ID;constraint:OnDelete:CASCADE"`
}

func (serverSSHKeyWithFK) TableName() string {
	return "server_ssh_keys"
}

func createServerSSHKeysTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&serverSSHKeyMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&serverSSHKeyWithFK{}, "Server"); err != nil {
		return err
	}

	return migrator.CreateConstraint(&serverSSHKeyWithFK{}, "SSHKey")
}

func createServerSSHKeysTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&serverSSHKeyMigration{})
}
