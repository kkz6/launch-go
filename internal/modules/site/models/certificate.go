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
	ID          string               `gorm:"primaryKey;size:26" json:"id"`
	SiteID      string               `gorm:"size:26;not null;index" json:"site_id"`
	Type        enums.CertificateType `gorm:"size:50" json:"type"`
	Domains     string               `gorm:"type:json" json:"domains,omitempty"`
	CSR         *string              `gorm:"type:text" json:"csr,omitempty"`
	PublicKey   *string              `gorm:"type:text" json:"public_key,omitempty"`
	PrivateKey  *string              `gorm:"type:text" json:"-"`
	Certificate *string              `gorm:"type:text" json:"certificate,omitempty"`
	UploadedAt  *time.Time           `json:"uploaded_at,omitempty"`
	IsActive    bool                 `gorm:"default:false" json:"is_active"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`

	// Relations
	Site *Site `gorm:"foreignKey:SiteID" json:"site,omitempty"`
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
	if c.Domains == "" || c.Domains == "null" {
		return []string{}
	}

	var domains []string
	if err := json.Unmarshal([]byte(c.Domains), &domains); err != nil {
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

	c.Domains = string(data)

	return nil
}
