package adapters

import (
	"context"

	databasedto "github.com/kkz6/launch-go/internal/modules/database/dto"
	databasemodels "github.com/kkz6/launch-go/internal/modules/database/models"
	"github.com/kkz6/launch-go/internal/modules/site/contracts"
)

// DatabaseService defines the interface for database service operations needed by the adapter
type DatabaseService interface {
	CreateDatabase(ctx context.Context, serverID, teamID string, req *databasedto.CreateDatabaseRequest, userID *string) (*databasemodels.Database, error)
	GetDatabase(ctx context.Context, id, serverID, teamID string) (*databasemodels.Database, error)
}

// DatabaseManagerAdapter adapts database service to the DatabaseManager interface
type DatabaseManagerAdapter struct {
	svc DatabaseService
}

// NewDatabaseManagerAdapter creates a new DatabaseManagerAdapter
func NewDatabaseManagerAdapter(svc DatabaseService) contracts.DatabaseManager {
	return &DatabaseManagerAdapter{svc: svc}
}

// CreateDatabase creates a new database on a server
func (a *DatabaseManagerAdapter) CreateDatabase(ctx context.Context, serverID, teamID string, req *databasedto.CreateDatabaseRequest, userID *string) (*databasemodels.Database, error) {
	return a.svc.CreateDatabase(ctx, serverID, teamID, req, userID)
}

// GetDatabase retrieves a database by ID with server and team validation
func (a *DatabaseManagerAdapter) GetDatabase(ctx context.Context, id, serverID, teamID string) (*databasemodels.Database, error) {
	return a.svc.GetDatabase(ctx, id, serverID, teamID)
}
