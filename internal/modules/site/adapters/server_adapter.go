package adapters

import (
	"context"

	servercontracts "github.com/kkz6/launch-go/internal/modules/server/contracts"
	serverdto "github.com/kkz6/launch-go/internal/modules/server/dto"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/contracts"
)

// ServerReaderAdapter adapts server repositories to the ServerReader interface
type ServerReaderAdapter struct {
	repos servercontracts.RepositoryRegistry
}

// NewServerReaderAdapter creates a new ServerReaderAdapter
func NewServerReaderAdapter(repos servercontracts.RepositoryRegistry) contracts.ServerReader {
	return &ServerReaderAdapter{repos: repos}
}

// FindServerByID retrieves a server by ID with its services preloaded
func (a *ServerReaderAdapter) FindServerByID(ctx context.Context, id string) (*servermodels.Server, error) {
	return a.repos.Server().FindByID(ctx, id)
}

// FindServicesByServer retrieves all installed services for a server
func (a *ServerReaderAdapter) FindServicesByServer(ctx context.Context, serverID string) ([]servermodels.InstalledService, error) {
	return a.repos.Service().FindByServer(ctx, serverID)
}

// CronCreatorAdapter adapts server service to the CronCreator interface
type CronCreatorAdapter struct {
	svc servercontracts.CronService
}

// NewCronCreatorAdapter creates a new CronCreatorAdapter
func NewCronCreatorAdapter(svc servercontracts.CronService) contracts.CronCreator {
	return &CronCreatorAdapter{svc: svc}
}

// CreateCron creates a new cron job on a server.
func (a *CronCreatorAdapter) CreateCron(ctx context.Context, serverID, teamID string, req *serverdto.CreateCronRequest) (*servermodels.Cron, error) {
	return a.svc.CreateCronRaw(ctx, serverID, teamID, req)
}

// CountCronsBySite counts cron jobs associated with a site
func (a *CronCreatorAdapter) CountCronsBySite(ctx context.Context, siteID string) (int64, error) {
	return a.svc.CountCronsBySite(ctx, siteID)
}
