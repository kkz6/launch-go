package jobs

import (
	"fmt"

	"github.com/kkz6/launch-go/internal/modules/server/enums"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks/mysql"
	"github.com/kkz6/launch-go/internal/modules/server/tasks/postgresql"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// DatabaseTaskFactory creates database-specific tasks based on service type
type DatabaseTaskFactory struct {
	serviceType enums.ServiceType
	server      *models.Server
}

// NewDatabaseTaskFactory creates a new factory for the given service type
func NewDatabaseTaskFactory(serviceType enums.ServiceType, server *models.Server) *DatabaseTaskFactory {
	return &DatabaseTaskFactory{
		serviceType: serviceType,
		server:      server,
	}
}

// CreateDatabase returns a task to create a database
func (f *DatabaseTaskFactory) CreateDatabase(name string) (taskrunner.Task, error) {
	switch f.serviceType {
	case enums.ServiceTypeMySql:
		return mysql.NewCreateDatabase(f.server, name, "utf8mb4", "utf8mb4_unicode_ci").
			OnServer(f.server), nil
	case enums.ServiceTypePostgreSql:
		return postgresql.NewCreateDatabase(f.server, name).
			OnServer(f.server), nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", f.serviceType)
	}
}

// CreateUser returns a task to create a database user
func (f *DatabaseTaskFactory) CreateUser(username, password string) (taskrunner.Task, error) {
	switch f.serviceType {
	case enums.ServiceTypeMySql:
		return mysql.NewCreateUser(f.server, username, password).
			OnServer(f.server), nil
	case enums.ServiceTypePostgreSql:
		return postgresql.NewCreateUser(f.server, username, password).
			OnServer(f.server), nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", f.serviceType)
	}
}

// DropDatabase returns a task to drop a database
func (f *DatabaseTaskFactory) DropDatabase(name string) (taskrunner.Task, error) {
	switch f.serviceType {
	case enums.ServiceTypeMySql:
		return mysql.NewDropDatabase(f.server, name).
			OnServer(f.server), nil
	case enums.ServiceTypePostgreSql:
		return postgresql.NewDropDatabase(f.server, name).
			OnServer(f.server), nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", f.serviceType)
	}
}

// DropUser returns a task to drop a database user
func (f *DatabaseTaskFactory) DropUser(username string) (taskrunner.Task, error) {
	switch f.serviceType {
	case enums.ServiceTypeMySql:
		return mysql.NewDropUser(f.server, username).
			OnServer(f.server), nil
	case enums.ServiceTypePostgreSql:
		return postgresql.NewDropUser(f.server, username).
			OnServer(f.server), nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", f.serviceType)
	}
}

// GrantAllPrivileges returns a task to grant all privileges on a database to a user
func (f *DatabaseTaskFactory) GrantAllPrivileges(database, username string) (taskrunner.Task, error) {
	switch f.serviceType {
	case enums.ServiceTypeMySql:
		return mysql.NewGrantAllPrivileges(f.server, database, username).
			OnServer(f.server), nil
	case enums.ServiceTypePostgreSql:
		return postgresql.NewGrantAllPrivileges(f.server, database, username).
			OnServer(f.server), nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", f.serviceType)
	}
}
