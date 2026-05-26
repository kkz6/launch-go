package migrations

import (
	"time"

	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		ID:        "0003_06_14_000002_create_certificates_table",
		Name:      "Create certificates table",
		Timestamp: time.Date(2003, 6, 14, 0, 0, 2, 0, time.UTC),
		Up:        createCertificatesTableUp,
		Down:      createCertificatesTableDown,
	})
}

// certificateMigration model for migration
type certificateMigration struct {
	ID          string     `gorm:"type:char(26);primaryKey"`
	SiteID      string     `gorm:"column:site_id;type:char(26);not null;index"`
	Type        string     `gorm:"type:varchar(255);not null;default:letsencrypt"`
	Domains     *string    `gorm:"type:varchar(255)"`
	CSR         *string    `gorm:"column:csr;type:text"`
	PublicKey   *string    `gorm:"column:public_key;type:text"`
	PrivateKey  *string    `gorm:"column:private_key;type:text"`
	Certificate *string    `gorm:"type:text"`
	UploadedAt  *time.Time `gorm:"column:uploaded_at;type:timestamp null"`
	IsActive    bool       `gorm:"column:is_active;default:false"`
	CreatedAt   *time.Time `gorm:"type:timestamp null"`
	UpdatedAt   *time.Time `gorm:"type:timestamp null"`
}

func (certificateMigration) TableName() string {
	return "certificates"
}

// certificateWithSiteFK defines the site foreign key
type certificateWithSiteFK struct {
	SiteID string         `gorm:"column:site_id"`
	Site   *siteMigration `gorm:"foreignKey:SiteID;references:ID;constraint:OnDelete:CASCADE"`
}

func (certificateWithSiteFK) TableName() string {
	return "certificates"
}

func createCertificatesTableUp(db *gorm.DB) error {
	migrator := db.Migrator()

	if err := migrator.CreateTable(&certificateMigration{}); err != nil {
		return err
	}

	return migrator.CreateConstraint(&certificateWithSiteFK{}, "Site")
}

func createCertificatesTableDown(db *gorm.DB) error {
	return db.Migrator().DropTable(&certificateMigration{})
}
