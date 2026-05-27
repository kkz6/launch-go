package models

import (
	"fmt"
	"time"

	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// Certificate represents an SSL certificate for a site
type Certificate struct {
	basemodels.BaseModel
	basemodels.SiteScoped
	basemodels.TeamScoped
	Type        sitetypes.CertificateType `gorm:"type:varchar(255);not null;default:letsencrypt" json:"type"`
	Domains     dbtype.JSONStringSlice    `gorm:"type:json" json:"domains,omitempty"`
	CSR         *string                   `gorm:"column:csr;type:longtext" json:"csr,omitempty"`
	PublicKey   *string                   `gorm:"column:public_key;type:longtext" json:"public_key,omitempty"`
	PrivateKey  dbtype.EncryptedString    `gorm:"column:private_key;type:longtext" json:"-"`
	Certificate *string                   `gorm:"type:longtext" json:"certificate,omitempty"`
	UploadedAt  *time.Time                `gorm:"column:uploaded_at;type:timestamp null" json:"uploaded_at,omitempty"`
	IsActive    bool                      `gorm:"column:is_active;default:false" json:"is_active"`

	// StoredCertificateID is the FK to the team-scoped
	// stored_certificates library row this certificate was sourced
	// from. Populated when the site SSL update specifies a stored
	// cert; nil for legacy inline-paste certs and Let's Encrypt certs.
	// The DB column was added in migration 0047 with ON DELETE SET
	// NULL, so the FK self-clears on stored cert hard-delete.
	StoredCertificateID *string `gorm:"column:stored_certificate_id;type:char(26)" json:"stored_certificate_id,omitempty"`

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
