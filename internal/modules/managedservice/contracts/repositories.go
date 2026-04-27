package contracts

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/managedservice/models"
	"github.com/kkz6/launch-go/internal/modules/managedservice/types"
)

// ManagedServiceRepository defines the persistence contract for the
// managed-services table.
type ManagedServiceRepository interface {
	Create(ctx context.Context, m *models.ManagedService) error
	FindByID(ctx context.Context, id string) (*models.ManagedService, error)
	FindByServerAndKind(ctx context.Context, serverID string, kind types.Kind) (*models.ManagedService, error)
	FindByServer(ctx context.Context, serverID string) ([]models.ManagedService, error)
	Update(ctx context.Context, id string, updates map[string]any) error
	Delete(ctx context.Context, id string) error
}
