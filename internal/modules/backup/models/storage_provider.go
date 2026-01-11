package models

import (
	"time"

	"github.com/kkz6/launch-go/internal/modules/backup/enums"
)

// StorageProvider represents a configured storage destination
type StorageProvider struct {
	ID             uint64              `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID         string              `gorm:"column:user_id;type:char(26);not null;index" json:"user_id"`
	TeamID         *string             `gorm:"column:team_id;type:char(26);index" json:"team_id,omitempty"`
	Provider       enums.StorageDriver `gorm:"type:varchar(255);not null" json:"provider"`
	Label          *string             `gorm:"type:varchar(255)" json:"label,omitempty"`
	Token          *string             `gorm:"type:varchar(1000)" json:"-"`
	Credentials    EncryptedJSON       `gorm:"type:longtext" json:"-"`
	RefreshToken   *string             `gorm:"column:refresh_token;type:varchar(1000)" json:"-"`
	Connected      bool                `gorm:"default:true" json:"connected"`
	TokenExpiresAt *time.Time          `gorm:"column:token_expires_at;type:timestamp null" json:"token_expires_at,omitempty"`
	CreatedAt      *time.Time          `gorm:"type:timestamp null" json:"created_at,omitempty"`
	UpdatedAt      *time.Time          `gorm:"type:timestamp null" json:"updated_at,omitempty"`

	// Relations
	Backups []Backup `gorm:"foreignKey:StorageProviderID;references:ID" json:"backups,omitempty"`
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
