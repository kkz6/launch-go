package models

import (
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/site/enums"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// Certificate represents an SSL certificate for a site
type Certificate struct {
	basemodels.BaseModel
	basemodels.SiteScopedModel
	basemodels.TeamScopedModel
	Type        enums.CertificateType      `gorm:"type:varchar(255);not null;default:letsencrypt" json:"type"`
	Domains     basemodels.JSONStringSlice `gorm:"type:json" json:"domains,omitempty"`
	CSR         *string                    `gorm:"column:csr;type:longtext" json:"csr,omitempty"`
	PublicKey   *string                    `gorm:"column:public_key;type:longtext" json:"public_key,omitempty"`
	PrivateKey  basemodels.EncryptedString `gorm:"column:private_key;type:longtext" json:"-"`
	Certificate *string                    `gorm:"type:longtext" json:"certificate,omitempty"`
	UploadedAt  *time.Time                 `gorm:"column:uploaded_at;type:timestamp null" json:"uploaded_at,omitempty"`
	IsActive    bool                       `gorm:"column:is_active;default:false" json:"is_active"`

	// Relations
	Site *Site `gorm:"foreignKey:SiteID;references:ID" json:"site,omitempty"`
}

func (Certificate) TableName() string {
	return "certificates"
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

// GetDomains returns the domains for the certificate
func (c *Certificate) GetDomains() []string {
	if c.Domains == nil {
		return []string{}
	}
	return c.Domains
}

// SetDomains sets the domains for the certificate
func (c *Certificate) SetDomains(domains []string) error {
	c.Domains = domains
	return nil
}
