package models

import (
	"github.com/kkz6/launch-go/internal/modules/dockerregistry/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

// Credential is a docker registry credential stored at the team level.
// At deploy time the application module looks up the credential by ID and
// runs `docker login` before pulling the image.
type Credential struct {
	basemodels.BaseModel
	basemodels.TeamScoped

	// Name is a user-facing label, e.g. "Acme GHCR".
	Name string `gorm:"type:varchar(255);not null" json:"name"`

	// Type narrows what URL/auth shape we expect.
	Type types.Type `gorm:"type:varchar(32);not null;index" json:"type"`

	// URL is the registry endpoint. We pre-fill from Type.DefaultURL() for
	// known kinds; for generic registries the caller must supply it.
	URL string `gorm:"type:varchar(255);not null" json:"url"`

	// Username is the account on the registry. Encrypted at rest.
	Username dbtype.EncryptedString `gorm:"type:longtext;not null" json:"-"`

	// Password is the auth secret (PAT for ghcr, password for docker hub,
	// etc.). Encrypted at rest. Never returned in API responses.
	Password dbtype.EncryptedString `gorm:"type:longtext;not null" json:"-"`
}

// TableName overrides the default GORM table name.
func (Credential) TableName() string {
	return "docker_registry_credentials"
}
