package contracts

import (
	"context"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
)

// ServerReader exposes the read-side of the server module.
type ServerReader interface {
	FindByIDAndTeam(ctx context.Context, id, teamID string, preloads ...string) (*servermodels.Server, error)
}

// RegistryCredentialReader returns a docker-registry credential by ID.
// Implemented by the dockerregistry service.
type RegistryCredentialReader interface {
	GetForApp(ctx context.Context, id, teamID string) (*RegistryCredential, error)
}

// RegistryCredential is the cross-module projection used at deploy time.
// Keeping it free of the dockerregistry module avoids an import cycle
// when that module wants to call into us.
type RegistryCredential struct {
	ID       string
	Name     string
	URL      string
	Username string
	Password string
}
