package models

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/backup/enums"
)

// StorageProvider represents a configured storage destination
type StorageProvider struct {
	ID             uint                `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID         string              `gorm:"size:26;not null;index" json:"user_id"`
	TeamID         string              `gorm:"size:26;not null;index" json:"team_id"`
	Provider       enums.StorageDriver `gorm:"size:50;not null" json:"provider"`
	Label          string              `gorm:"size:255;not null" json:"label"`
	Token          *string             `gorm:"type:text" json:"-"`
	Credentials    EncryptedJSON       `gorm:"type:text" json:"-"`
	RefreshToken   *string             `gorm:"type:text" json:"-"`
	Connected      bool                `gorm:"default:false" json:"connected"`
	TokenExpiresAt *time.Time          `json:"token_expires_at,omitempty"`
	CreatedAt      time.Time           `json:"created_at"`
	UpdatedAt      time.Time           `json:"updated_at"`

	// Relations
	Backups []Backup `gorm:"foreignKey:StorageProviderID" json:"backups,omitempty"`
}

// TableName returns the table name for StorageProvider
func (StorageProvider) TableName() string {
	return "storage_providers"
}

// GetCredentials decrypts and returns credentials as a map
func (s *StorageProvider) GetCredentials() (map[string]interface{}, error) {
	return s.Credentials.Decrypt()
}

// SetCredentials encrypts and sets credentials from a map
func (s *StorageProvider) SetCredentials(creds map[string]interface{}) error {
	encrypted, err := NewEncryptedJSON(creds)
	if err != nil {
		return err
	}
	s.Credentials = encrypted

	return nil
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

// DropboxCredentials represents Dropbox-specific credentials
type DropboxCredentials struct {
	Token string `json:"token"`
}
