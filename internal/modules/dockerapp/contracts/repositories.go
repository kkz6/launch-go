package contracts

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/dockerapp/models"
)

// AppRepository persists and queries the dockerapp.App aggregate.
type AppRepository interface {
	Create(ctx context.Context, app *models.App) error
	FindByID(ctx context.Context, id string) (*models.App, error)
	FindByIDWithRelations(ctx context.Context, id string) (*models.App, error)
	FindByIDAndServer(ctx context.Context, id, serverID string) (*models.App, error)
	FindByServer(ctx context.Context, serverID string) ([]models.App, error)
	FindByNameAndServer(ctx context.Context, name, serverID string) (*models.App, error)
	Update(ctx context.Context, id string, updates map[string]any) error
	Delete(ctx context.Context, id string) error
}

// EnvVarRepository persists env-var rows attached to an app.
type EnvVarRepository interface {
	Create(ctx context.Context, e *models.EnvVar) error
	FindByApp(ctx context.Context, appID string) ([]models.EnvVar, error)
	Update(ctx context.Context, id string, updates map[string]any) error
	Delete(ctx context.Context, id string) error
	DeleteByApp(ctx context.Context, appID string) error
}

// PortRepository persists port rows.
type PortRepository interface {
	Create(ctx context.Context, p *models.Port) error
	FindByApp(ctx context.Context, appID string) ([]models.Port, error)
	Update(ctx context.Context, id string, updates map[string]any) error
	Delete(ctx context.Context, id string) error
}

// VolumeRepository persists volume rows.
type VolumeRepository interface {
	Create(ctx context.Context, v *models.Volume) error
	FindByApp(ctx context.Context, appID string) ([]models.Volume, error)
	Delete(ctx context.Context, id string) error
}

// DomainRepository persists domain rows.
type DomainRepository interface {
	Create(ctx context.Context, d *models.Domain) error
	FindByApp(ctx context.Context, appID string) ([]models.Domain, error)
	FindByDomain(ctx context.Context, domain string) (*models.Domain, error)
	Update(ctx context.Context, id string, updates map[string]any) error
	Delete(ctx context.Context, id string) error
}

// RepositoryRegistry exposes the module's repositories.
type RepositoryRegistry interface {
	App() AppRepository
	EnvVar() EnvVarRepository
	Port() PortRepository
	Volume() VolumeRepository
	Domain() DomainRepository
}
