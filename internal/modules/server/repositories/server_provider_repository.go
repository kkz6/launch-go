package repositories

import (
	"context"

	"gorm.io/gorm"

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
			return nil, ErrServerProviderNotFound
		}
		return nil, err
	}
	return provider, nil
}
