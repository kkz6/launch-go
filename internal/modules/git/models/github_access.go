package models

import (
	"time"

	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	base "github.com/kkz6/launch-go/internal/pkg/models"
)

type GitHubAuthorization struct {
	base.BaseModel
	Host             string `gorm:"not null;uniqueIndex:github_authorization_identity"`
	AppID            string `gorm:"not null;uniqueIndex:github_authorization_identity"`
	UserID           string `gorm:"type:char(26);not null;uniqueIndex:github_authorization_identity"`
	GitHubUserID     string `gorm:"not null;uniqueIndex:github_authorization_identity"`
	Login            string
	AccessToken      dbtype.EncryptedString `gorm:"type:text" json:"-"`
	RefreshToken     dbtype.EncryptedString `gorm:"type:text" json:"-"`
	ExpiresAt        time.Time
	RefreshExpiresAt time.Time
	Generation       uint64 `gorm:"not null;default:1"`
	RevokedAt        *time.Time
}

type GitHubConnectionAttempt struct {
	base.BaseModel
	UserID               string                 `gorm:"type:char(26);not null;index"`
	TeamID               string                 `gorm:"type:char(26);not null;index"`
	SessionHash          string                 `gorm:"not null" json:"-"`
	StateHash            string                 `gorm:"not null;uniqueIndex" json:"-"`
	InstallStateHash     *string                `gorm:"uniqueIndex" json:"-"`
	PKCEVerifier         dbtype.EncryptedString `gorm:"type:text" json:"-"`
	MembershipHash       string                 `json:"-"`
	SourceControlID      *string                `gorm:"type:char(26)"`
	AuthorizationID      *string                `gorm:"type:char(26)"`
	ConnectionGeneration uint64
	SelectionVersion     uint64
	Status               string `gorm:"not null"`
	ExpiresAt            time.Time
	ConsumedAt           *time.Time
}

type GitHubInstallation struct {
	base.BaseModel
	Host           string `gorm:"not null;uniqueIndex:github_installation_identity"`
	AppID          string `gorm:"not null;uniqueIndex:github_installation_identity"`
	InstallationID string `gorm:"not null;uniqueIndex:github_installation_identity"`
	AccountID      string
	AccountLogin   string
	AccountType    string
	Permissions    string `gorm:"type:text"`
	Status         string `gorm:"not null"`
	Generation     uint64 `gorm:"not null;default:1"`
	// RepositoryRevision fences selection responses across upstream repository
	// changes without invalidating unrelated existing repository capabilities.
	RepositoryRevision uint64 `gorm:"not null;default:1"`
	LastSyncedAt       *time.Time
}

type GitHubInstallationRepository struct {
	base.BaseModel
	InstallationID string `gorm:"type:char(26);not null;uniqueIndex:github_installation_repository"`
	RepositoryID   string `gorm:"not null;uniqueIndex:github_installation_repository"`
	FullName       string
	DefaultBranch  string
	Private        bool
	Available      bool
}

// GitHubCredentialLease records issuance before the provider call. The encrypted
// token is retained only to revoke it on disconnect or failed issuance fencing.
type GitHubCredentialLease struct {
	CleanupManifestID *string `gorm:"type:char(26);index"`
	base.BaseModel
	TeamID                  string `gorm:"type:char(26);not null;index"`
	SourceControlID         string `gorm:"type:char(26);not null;index"`
	ConnectionGeneration    uint64
	RepositoryID            uint64
	GrantVersion            uint64
	AuthorizationID         string `gorm:"type:char(26)"`
	AuthorizationGeneration uint64
	InstallationID          string
	Token                   dbtype.EncryptedString `gorm:"type:text" json:"-"`
	ExpiresAt               time.Time
	Status                  string `gorm:"not null;index"`
}

// GitHubCleanupManifest records resources before provisioning. Activation grants
// only a short window to remove these exact resources after local revocation.
type GitHubCleanupManifest struct {
	base.BaseModel
	SourceControlID         string `gorm:"type:char(26);not null;index"`
	TeamID                  string `gorm:"type:char(26);not null"`
	RepositoryID            string `gorm:"not null;index"`
	GitHubRepositoryID      string
	RepositoryFullName      string
	InstallationID          string
	AuthorizationID         string
	AuthorizationGeneration uint64
	InstallationGeneration  uint64
	ConnectionGeneration    uint64
	GrantVersion            uint64
	Resources               string `gorm:"type:text;not null"`
	Status                  string `gorm:"not null;index"`
	ExpiresAt               *time.Time
}
