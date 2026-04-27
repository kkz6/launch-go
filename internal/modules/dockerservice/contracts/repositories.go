package contracts

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/dockerservice/models"
	"github.com/kkz6/launch-go/internal/modules/dockerservice/types"
)

// DockerServiceRepository defines the persistence contract for the
// docker-services table.
type DockerServiceRepository interface {
	Create(ctx context.Context, m *models.DockerService) error
	FindByID(ctx context.Context, id string) (*models.DockerService, error)
	FindByServerAndKind(ctx context.Context, serverID string, kind types.Kind) (*models.DockerService, error)
	FindByServer(ctx context.Context, serverID string) ([]models.DockerService, error)
	Update(ctx context.Context, id string, updates map[string]any) error
	Delete(ctx context.Context, id string) error
}
