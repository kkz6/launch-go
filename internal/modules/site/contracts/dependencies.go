package contracts

import (
	"context"

	databasedto "github.com/kkz6/launch-go/internal/modules/database/dto"
	databasemodels "github.com/kkz6/launch-go/internal/modules/database/models"
	gitmodels "github.com/kkz6/launch-go/internal/modules/git/models"
	serverdto "github.com/kkz6/launch-go/internal/modules/server/dto"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
)

// ServerReader provides read-only access to server data.
// Used by the site module to read server information without depending on concrete implementations.
type ServerReader interface {
	// FindServerByID retrieves a server by ID with its services preloaded
	FindServerByID(ctx context.Context, id string) (*servermodels.Server, error)
	// FindServicesByServer retrieves all installed services for a server
	FindServicesByServer(ctx context.Context, serverID string) ([]servermodels.InstalledService, error)
}

// CronCreator provides the ability to create and query cron jobs.
// Used by the site module to create scheduler crons for Laravel/WordPress sites.
type CronCreator interface {
	// CreateCron creates a new cron job on a server
	CreateCron(ctx context.Context, serverID, teamID string, req *serverdto.CreateCronRequest) (*servermodels.Cron, error)
	// CountCronsBySite counts cron jobs associated with a site
	CountCronsBySite(ctx context.Context, siteID string) (int64, error)
}

// DatabaseManager provides database creation and retrieval operations.
// Used by the site module when creating sites with associated databases.
type DatabaseManager interface {
	// CreateDatabase creates a new database on a server
	CreateDatabase(ctx context.Context, serverID, teamID string, req *databasedto.CreateDatabaseRequest, userID *string) (*databasemodels.Database, error)
	// GetDatabase retrieves a database by ID with server and team validation
	GetDatabase(ctx context.Context, id, serverID, teamID string) (*databasemodels.Database, error)
	// GetDatabaseUser retrieves a database user by ID and server
	GetDatabaseUser(ctx context.Context, id, serverID string) (*databasemodels.DatabaseUser, error)
}

// GitReader provides read-only access to git/source control data.
// Used by the site module to read source control and repository information.
type GitReader interface {
	// FindSourceControlByID retrieves a source control by ID and team ID
	FindSourceControlByID(ctx context.Context, id, teamID string) (*gitmodels.SourceControl, error)
	// FindRepositoryByID retrieves a source control repository by ID
	FindRepositoryByID(ctx context.Context, id string) (*gitmodels.SourceControlRepository, error)
}
