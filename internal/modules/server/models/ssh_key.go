package models

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/utils"
)

// SshKey represents an SSH public key
type SshKey struct {
	ID          string    `gorm:"primaryKey;size:26" json:"id"`
	UserID      *string   `gorm:"size:26;index" json:"user_id,omitempty"`
	TeamID      *string   `gorm:"size:26;index" json:"team_id,omitempty"`
	IsGlobal    bool      `gorm:"default:false" json:"is_global"`
	PublicKey   string    `gorm:"type:text;not null" json:"-"`
	Name        string    `gorm:"size:255;not null" json:"name"`
	Description *string   `gorm:"type:text" json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Relations
	Servers []Server `gorm:"many2many:server_ssh_keys" json:"servers,omitempty"`
}

func (k *SshKey) BeforeCreate(tx *gorm.DB) error {
	if k.ID == "" {
		k.ID = utils.NewULID()
	}

	return nil
}

func (k *SshKey) TableName() string {
	return "ssh_keys"
}

func (k *SshKey) GetFingerprint() string {
	return GenerateSSHFingerprint(k.PublicKey, FingerprintAlgorithmMD5)
}

// ServerSshKey represents the many-to-many relationship between servers and SSH keys
type ServerSshKey struct {
	ServerID  string    `gorm:"primaryKey;size:26" json:"server_id"`
	SshKeyID  string    `gorm:"primaryKey;size:26" json:"ssh_key_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *ServerSshKey) TableName() string {
	return "server_ssh_keys"
}

// FingerprintAlgorithm represents the algorithm used for SSH fingerprints
type FingerprintAlgorithm string

const (
	FingerprintAlgorithmMD5    FingerprintAlgorithm = "md5"
	FingerprintAlgorithmSHA256 FingerprintAlgorithm = "sha256"
)

// GenerateSSHFingerprint generates a fingerprint for an SSH public key
func GenerateSSHFingerprint(publicKey string, algorithm FingerprintAlgorithm) string {
	if !strings.HasPrefix(publicKey, "ssh-") {
		return ""
	}

	parts := strings.SplitN(publicKey, " ", 3)
	if len(parts) < 2 {
		return ""
	}

	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}

	switch algorithm {
	case FingerprintAlgorithmMD5:
		hash := md5.Sum(decoded)
		hexParts := make([]string, len(hash))
		for i, b := range hash {
			hexParts[i] = fmt.Sprintf("%02x", b)
		}

		return strings.Join(hexParts, ":")
	case FingerprintAlgorithmSHA256:
		return base64.StdEncoding.EncodeToString(decoded)
	}

	return ""
}
