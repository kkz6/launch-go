package contracts

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/dockerregistry/models"
)

// CredentialRepository is the persistence contract for docker registry
// credentials.
type CredentialRepository interface {
	Create(ctx context.Context, c *models.Credential) error
	FindByID(ctx context.Context, id string) (*models.Credential, error)
	FindByIDAndTeam(ctx context.Context, id, teamID string) (*models.Credential, error)
	FindByTeam(ctx context.Context, teamID string) ([]models.Credential, error)
	FindByNameAndTeam(ctx context.Context, name, teamID string) (*models.Credential, error)
	Update(ctx context.Context, id string, updates map[string]any) error
	Delete(ctx context.Context, id string) error
}

// RepositoryRegistry exposes the module's repositories.
type RepositoryRegistry interface {
	Credential() CredentialRepository
}
