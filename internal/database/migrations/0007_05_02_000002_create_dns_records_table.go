package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0007_05_02_000002_create_dns_records_table",
		Name:      "Create dns_records table",
		Timestamp: time.Date(2007, 5, 2, 0, 0, 2, 0, time.UTC),
		Up:        createDNSRecordsTableUp,
		Down:      createDNSRecordsTableDown,
	})
}

// dnsRecordMigration model for migration (with proxied field merged)
type dnsRecordMigration struct {
	ID         string     `gorm:"type:char(26);primaryKey"`
	DomainID   string     `gorm:"column:domain_id;type:char(26);not null;index"`
	ProviderID string     `gorm:"column:provider_id;type:varchar(255);not null"`
	Type       string     `gorm:"type:varchar(255);not null"`
	Name       string     `gorm:"type:varchar(255);not null"`
	Value      string     `gorm:"type:varchar(255);not null"`
	TTL        int        `gorm:"type:int;not null"`
	Priority   *int       `gorm:"type:int"`
	Tag        *string    `gorm:"type:varchar(255)"`
	Weight     *int       `gorm:"type:int"`
	Port       *int       `gorm:"type:int"`
	Flags      *int       `gorm:"type:int"`
	Comment    *string    `gorm:"type:varchar(255)"`
	Proxied    *bool      `gorm:"type:boolean"` // For Cloudflare
	CreatedAt  *time.Time `gorm:"type:timestamp null"`
	UpdatedAt  *time.Time `gorm:"type:timestamp null"`
}

func (dnsRecordMigration) TableName() string {
	return "dns_records"
}

// dnsRecordWithDomainFK defines the domain foreign key
type dnsRecordWithDomainFK struct {
	DomainID string           `gorm:"column:domain_id"`
	Domain   *domainMigration `gorm:"foreignKey:DomainID;references:ID;constraint:OnDelete:CASCADE"`
}

func (dnsRecordWithDomainFK) TableName() string {
	return "dns_records"
}

func createDNSRecordsTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&dnsRecordMigration{}); err != nil {
		return err
	}

	if err := migrator.CreateConstraint(&dnsRecordWithDomainFK{}, "Domain"); err != nil {
		return err
	}

	return nil
}

func createDNSRecordsTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&dnsRecordMigration{})
}
