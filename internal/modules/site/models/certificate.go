package models

import (
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/site/enums"
	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// Certificate represents an SSL certificate for a site
type Certificate struct {
	ID          string                `gorm:"type:char(26);primaryKey" json:"id"`
	SiteID      string                `gorm:"column:site_id;type:char(26);not null;index" json:"site_id"`
	Type        enums.CertificateType `gorm:"type:varchar(255);not null;default:letsencrypt" json:"type"`
	Domains     *string               `gorm:"type:varchar(255)" json:"domains,omitempty"`
	CSR         *string               `gorm:"column:csr;type:longtext" json:"csr,omitempty"`
	PublicKey   *string               `gorm:"column:public_key;type:longtext" json:"public_key,omitempty"`
	PrivateKey  *string               `gorm:"column:private_key;type:longtext" json:"-"`
	Certificate *string               `gorm:"type:longtext" json:"certificate,omitempty"`
	UploadedAt  *time.Time            `gorm:"column:uploaded_at;type:timestamp null" json:"uploaded_at,omitempty"`
	IsActive    bool                  `gorm:"column:is_active;default:false" json:"is_active"`
	CreatedAt   *time.Time            `gorm:"type:timestamp null" json:"created_at,omitempty"`
	UpdatedAt   *time.Time            `gorm:"type:timestamp null" json:"updated_at,omitempty"`

	// Relations
	Site *Site `gorm:"foreignKey:SiteID;references:ID" json:"site,omitempty"`
}

func (c *Certificate) TableName() string {
	return "certificates"
}

func (c *Certificate) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = utils.NewULID()
	}

	return nil
}

// SiteDirectory returns the directory path for the certificate
func (c *Certificate) SiteDirectory(sitePath string) string {
	return fmt.Sprintf("%s/certificates/%s", sitePath, c.ID)
}

// CertificatePath returns the path to the certificate file
func (c *Certificate) CertificatePath(sitePath string) string {
	return fmt.Sprintf("%s/certificate.cert", c.SiteDirectory(sitePath))
}

// PrivateKeyPath returns the path to the private key file
func (c *Certificate) PrivateKeyPath(sitePath string) string {
	return fmt.Sprintf("%s/private.key", c.SiteDirectory(sitePath))
}

// GetDomains returns domains as a slice
func (c *Certificate) GetDomains() []string {
	if c.Domains == nil || *c.Domains == "" || *c.Domains == "null" {
		return []string{}
	}

	var domains []string
	if err := json.Unmarshal([]byte(*c.Domains), &domains); err != nil {
		return []string{}
	}

	return domains
}

// SetDomains sets domains from a slice
func (c *Certificate) SetDomains(domains []string) error {
	data, err := json.Marshal(domains)
	if err != nil {
		return err
	}

	str := string(data)
	c.Domains = &str

	return nil
}
