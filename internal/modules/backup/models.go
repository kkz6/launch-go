package backup

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// Backup represents a backup configuration for a server
type Backup struct {
	ID                        string          `gorm:"primaryKey;size:26" json:"id"`
	ServerID                  string          `gorm:"size:26;not null;index" json:"server_id"`
	UserID                    string          `gorm:"size:26;not null;index" json:"user_id"`
	StorageProviderID         string          `gorm:"size:26;not null;index" json:"storage_provider_id"`
	CronExpression            string          `gorm:"size:100;not null" json:"cron_expression"`
	IncludeFiles              JSON            `gorm:"type:json" json:"include_files"`
	ExcludeFiles              JSON            `gorm:"type:json" json:"exclude_files"`
	Retention                 int             `gorm:"default:10" json:"retention"`
	NotificationOnFailure     bool            `gorm:"default:false" json:"notification_on_failure"`
	NotificationOnSuccess     bool            `gorm:"default:false" json:"notification_on_success"`
	Enabled                   bool            `gorm:"default:true" json:"enabled"`
	Path                      string          `gorm:"size:500" json:"path"`
	DispatchToken             string          `gorm:"size:64;not null" json:"dispatch_token"`
	InstalledAt               *time.Time      `json:"installed_at,omitempty"`
	InstallationFailedAt      *time.Time      `json:"installation_failed_at,omitempty"`
	UninstallationRequestedAt *time.Time      `json:"uninstallation_requested_at,omitempty"`
	UninstallationFailedAt    *time.Time      `json:"uninstallation_failed_at,omitempty"`
	CreatedAt                 time.Time       `json:"created_at"`
	UpdatedAt                 time.Time       `json:"updated_at"`
	DeletedAt                 gorm.DeletedAt  `gorm:"index" json:"-"`

	// Relations
	Jobs            []BackupJob      `gorm:"foreignKey:BackupID" json:"jobs,omitempty"`
	StorageProvider *StorageProvider `gorm:"foreignKey:StorageProviderID" json:"storage_provider,omitempty"`
	Databases       []BackupDatabase `gorm:"foreignKey:BackupID" json:"databases,omitempty"`
}

// BeforeCreate hook generates ULID and dispatch token
func (b *Backup) BeforeCreate(tx *gorm.DB) error {
	if b.ID == "" {
		b.ID = utils.NewULID()
	}
	if b.DispatchToken == "" {
		token := make([]byte, 32)
		if _, err := rand.Read(token); err != nil {
			return err
		}
		b.DispatchToken = base64.URLEncoding.EncodeToString(token)
	}
	if b.Retention == 0 {
		b.Retention = 10
	}
	return nil
}

// TableName returns the table name for Backup
func (Backup) TableName() string {
	return "backups"
}

// GetSizeInMB calculates total backup size in megabytes from all jobs
func (b *Backup) GetSizeInMB() int64 {
	var totalSize int64
	for _, job := range b.Jobs {
		totalSize += job.Size
	}
	return totalSize / 1024 / 1024
}

// IsInstalled returns true if the backup is installed
func (b *Backup) IsInstalled() bool {
	return b.InstalledAt != nil && b.InstallationFailedAt == nil
}

// IsPendingInstallation returns true if installation is pending
func (b *Backup) IsPendingInstallation() bool {
	return b.InstalledAt == nil && b.InstallationFailedAt == nil
}

// IsInstallationFailed returns true if installation failed
func (b *Backup) IsInstallationFailed() bool {
	return b.InstallationFailedAt != nil
}

// BackupJob represents an individual backup execution
type BackupJob struct {
	ID                string          `gorm:"primaryKey;size:26" json:"id"`
	BackupID          string          `gorm:"size:26;not null;index" json:"backup_id"`
	StorageProviderID string          `gorm:"size:26;not null;index" json:"storage_provider_id"`
	Status            BackupJobStatus `gorm:"size:20;not null;default:'pending'" json:"status"`
	Size              int64           `gorm:"default:0" json:"size"`
	Error             *string         `gorm:"type:text" json:"error,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`

	// Relations
	Backup          *Backup          `gorm:"foreignKey:BackupID" json:"backup,omitempty"`
	StorageProvider *StorageProvider `gorm:"foreignKey:StorageProviderID" json:"storage_provider,omitempty"`
}

// BeforeCreate hook generates ULID
func (j *BackupJob) BeforeCreate(tx *gorm.DB) error {
	if j.ID == "" {
		j.ID = utils.NewULID()
	}
	if j.Status == "" {
		j.Status = BackupJobStatusPending
	}
	return nil
}

// TableName returns the table name for BackupJob
func (BackupJob) TableName() string {
	return "backup_jobs"
}

// GetSizeInMB returns the backup size in megabytes
func (j *BackupJob) GetSizeInMB() int64 {
	return j.Size / 1024 / 1024
}

// IsFinished returns true if the backup job has finished successfully
func (j *BackupJob) IsFinished() bool {
	return j.Status == BackupJobStatusFinished
}

// IsFailed returns true if the backup job has failed
func (j *BackupJob) IsFailed() bool {
	return j.Status == BackupJobStatusFailed
}

// IsRunning returns true if the backup job is currently running
func (j *BackupJob) IsRunning() bool {
	return j.Status == BackupJobStatusRunning
}

// IsPending returns true if the backup job is pending
func (j *BackupJob) IsPending() bool {
	return j.Status == BackupJobStatusPending
}

// StorageProvider represents a configured storage destination
type StorageProvider struct {
	ID            uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID        string         `gorm:"size:26;not null;index" json:"user_id"`
	TeamID        string         `gorm:"size:26;not null;index" json:"team_id"`
	Provider      StorageDriver  `gorm:"size:50;not null" json:"provider"`
	Label         string         `gorm:"size:255;not null" json:"label"`
	Token         *string        `gorm:"type:text" json:"-"`
	Credentials   EncryptedJSON  `gorm:"type:text" json:"-"`
	RefreshToken  *string        `gorm:"type:text" json:"-"`
	Connected     bool           `gorm:"default:false" json:"connected"`
	TokenExpiresAt *time.Time    `json:"token_expires_at,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`

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

// BackupDatabase represents the many-to-many relationship between backups and databases
type BackupDatabase struct {
	ID         string    `gorm:"primaryKey;size:26" json:"id"`
	BackupID   string    `gorm:"size:26;not null;index" json:"backup_id"`
	DatabaseID string    `gorm:"size:26;not null;index" json:"database_id"`
	CreatedAt  time.Time `json:"created_at"`
}

// BeforeCreate hook generates ULID
func (bd *BackupDatabase) BeforeCreate(tx *gorm.DB) error {
	if bd.ID == "" {
		bd.ID = utils.NewULID()
	}
	return nil
}

// TableName returns the table name for BackupDatabase
func (BackupDatabase) TableName() string {
	return "backup_databases"
}

// JSON type for storing JSON arrays in the database
type JSON []byte

// Value implements driver.Valuer interface
func (j JSON) Value() (interface{}, error) {
	if len(j) == 0 {
		return "[]", nil
	}
	return string(j), nil
}

// Scan implements sql.Scanner interface
func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = []byte("[]")
		return nil
	}
	switch v := value.(type) {
	case []byte:
		*j = v
	case string:
		*j = []byte(v)
	}
	return nil
}

// MarshalJSON implements json.Marshaler
func (j JSON) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("[]"), nil
	}
	return j, nil
}

// UnmarshalJSON implements json.Unmarshaler
func (j *JSON) UnmarshalJSON(data []byte) error {
	*j = data
	return nil
}

// ToStringSlice converts JSON to a string slice
func (j JSON) ToStringSlice() ([]string, error) {
	if len(j) == 0 {
		return []string{}, nil
	}
	var result []string
	if err := json.Unmarshal(j, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// FromStringSlice creates JSON from a string slice
func FromStringSlice(s []string) (JSON, error) {
	if s == nil {
		s = []string{}
	}
	data, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return JSON(data), nil
}

// EncryptedJSON type for storing encrypted JSON in the database
// In production, this would use actual encryption with a key from config
type EncryptedJSON []byte

// NewEncryptedJSON creates a new EncryptedJSON from a map
func NewEncryptedJSON(data map[string]interface{}) (EncryptedJSON, error) {
	// In production, this would encrypt the data
	// For now, we just JSON encode it
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return EncryptedJSON(jsonData), nil
}

// Decrypt returns the decrypted credentials as a map
func (e EncryptedJSON) Decrypt() (map[string]interface{}, error) {
	if len(e) == 0 {
		return make(map[string]interface{}), nil
	}
	// In production, this would decrypt the data
	// For now, we just JSON decode it
	var result map[string]interface{}
	if err := json.Unmarshal(e, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Value implements driver.Valuer interface
func (e EncryptedJSON) Value() (interface{}, error) {
	if len(e) == 0 {
		return "", nil
	}
	return string(e), nil
}

// Scan implements sql.Scanner interface
func (e *EncryptedJSON) Scan(value interface{}) error {
	if value == nil {
		*e = []byte{}
		return nil
	}
	switch v := value.(type) {
	case []byte:
		*e = v
	case string:
		*e = []byte(v)
	}
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
