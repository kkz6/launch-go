package models

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"strings"

	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// SshKey represents an SSH public key
type SshKey struct {
	basemodels.BaseModel
	UserID      string  `gorm:"column:user_id;type:char(26);not null;index" json:"user_id"`
	TeamID      string  `gorm:"column:team_id;type:char(26);not null;index" json:"team_id"`
	IsGlobal    bool    `gorm:"column:is_global;type:tinyint(1);not null;default:0" json:"is_global"`
	Description *string `gorm:"type:varchar(255)" json:"description,omitempty"`
	PublicKey   string  `gorm:"type:longtext;not null" json:"-"`
	Name        string  `gorm:"type:varchar(255);not null" json:"name"`
	Fingerprint *string `gorm:"type:varchar(255)" json:"fingerprint,omitempty"`

	// Relations
	Servers []Server `gorm:"many2many:server_ssh_keys" json:"servers,omitempty"`
}

func (k *SshKey) TableName() string {
	return "ssh_keys"
}

func (k *SshKey) GetFingerprint() string {
	if k.Fingerprint != nil && *k.Fingerprint != "" {
		return *k.Fingerprint
	}

	return GenerateSSHFingerprint(k.PublicKey, FingerprintAlgorithmMD5)
}

// ServerSshKey represents the many-to-many relationship between servers and SSH keys
type ServerSshKey struct {
	basemodels.BaseModel
	ServerID string `gorm:"column:server_id;type:char(26);primaryKey" json:"server_id"`
	SshKeyID string `gorm:"column:ssh_key_id;type:char(26);primaryKey" json:"ssh_key_id"`
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
