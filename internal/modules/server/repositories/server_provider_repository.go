package repositories

import (
	"context"

	"gorm.io/gorm"

	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

// ServerProviderRepository handles server provider database operations
type ServerProviderRepository struct {
	repository.Base[models.ServerProvider]
}

// NewServerProviderRepository creates a new ServerProviderRepository instance
func NewServerProviderRepository(db *gorm.DB) *ServerProviderRepository {
	return &ServerProviderRepository{
		Base: repository.NewBase[models.ServerProvider](db),
	}
}

// FindByID finds a server provider by ID
func (r *ServerProviderRepository) FindByID(ctx context.Context, id string) (*models.ServerProvider, error) {
	provider, err := r.Base.FindByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, fiberutil.NotFound()
		}
		return nil, err
	}
	return provider, nil
}

// FindByUserID finds all server providers for a user
func (r *ServerProviderRepository) FindByUserID(ctx context.Context, userID string) ([]models.ServerProvider, error) {
	return repository.FindAll[models.ServerProvider](ctx, r.DB,
		repository.WithUserID(userID),
		repository.OrderByCreatedDesc(),
	)
}
