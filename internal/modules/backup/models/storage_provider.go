package models

import (
	"net/url"
	"strings"
	"time"

	backuptypes "github.com/kkz6/launch-go/internal/modules/backup/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
)

// StorageProvider represents a configured storage destination
// Note: Uses auto-increment ID
type StorageProvider struct {
	ID             uint64                    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID         string                    `gorm:"column:user_id;type:char(26);not null;index" json:"user_id"`
	TeamID         string                    `gorm:"column:team_id;type:char(26);not null;index" json:"team_id"`
	Provider       backuptypes.StorageDriver `gorm:"type:varchar(255);not null" json:"provider"`
	Label          *string                   `gorm:"type:varchar(255)" json:"label,omitempty"`
	Token          *string                   `gorm:"type:varchar(1000)" json:"-"`
	Credentials    dbtype.EncryptedJSONMap   `gorm:"type:longtext" json:"-"`
	RefreshToken   *string                   `gorm:"column:refresh_token;type:varchar(1000)" json:"-"`
	Connected      bool                      `gorm:"default:true" json:"connected"`
	TokenExpiresAt *time.Time                `gorm:"column:token_expires_at;type:timestamp null" json:"token_expires_at,omitempty"`
	CreatedAt      *time.Time                `gorm:"type:timestamp null" json:"created_at,omitempty"`
	UpdatedAt      *time.Time                `gorm:"type:timestamp null" json:"updated_at,omitempty"`

	// Relations
	Backups []Backup `gorm:"foreignKey:StorageProviderID;references:ID" json:"backups,omitempty"`
}

// TableName returns the table name for StorageProvider
func (StorageProvider) TableName() string {
	return "storage_providers"
}

// GetCredentials returns credentials as a map
func (s *StorageProvider) GetCredentials() map[string]any {
	if s.Credentials == nil {
		return make(map[string]any)
	}
	return s.Credentials
}

// SetCredentials sets credentials from a map
func (s *StorageProvider) SetCredentials(creds map[string]any) {
	s.Credentials = creds
}

// S3Credentials represents S3-specific credentials
type S3Credentials struct {
	Endpoint       string `json:"endpoint,omitempty"`
	Key            string `json:"key"`
	Secret         string `json:"secret"`
	Region         string `json:"region"`
	Bucket         string `json:"bucket"`
	Path           string `json:"path,omitempty"`
	ForcePathStyle bool   `json:"force_path_style,omitempty"`
}

// ApplyLegacyDefaults enables path-style access for legacy non-AWS providers.
func (c *S3Credentials) ApplyLegacyDefaults(rawCredentials map[string]any) {
	if c == nil || strings.TrimSpace(c.Endpoint) == "" {
		return
	}
	if _, configured := rawCredentials["force_path_style"]; configured {
		return
	}
	if !isAWSEndpoint(c.Endpoint) {
		c.ForcePathStyle = true
	}
}

func isAWSEndpoint(endpoint string) bool {
	normalized := strings.TrimSpace(endpoint)
	if !strings.Contains(normalized, "://") {
		normalized = "https://" + normalized
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		return false
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	return host == "amazonaws.com" ||
		strings.HasSuffix(host, ".amazonaws.com") ||
		host == "amazonaws.com.cn" ||
		strings.HasSuffix(host, ".amazonaws.com.cn")
}

// DropboxCredentials represents Dropbox-specific credentials
type DropboxCredentials struct {
	Token string `json:"token"`
}
