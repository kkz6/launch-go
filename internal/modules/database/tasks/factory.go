package tasks

import (
	"fmt"

	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// DatabaseTaskType represents the type of database task operation
type DatabaseTaskType string

// Task operation constants (database-agnostic)
const (
	TaskCreateDatabase   DatabaseTaskType = "create_database"
	TaskDropDatabase     DatabaseTaskType = "drop_database"
	TaskCreateUser       DatabaseTaskType = "create_user"
	TaskDropUser         DatabaseTaskType = "drop_user"
	TaskGrantPrivileges  DatabaseTaskType = "grant_privileges"
	TaskRevokePrivileges DatabaseTaskType = "revoke_privileges"
	TaskUpdatePassword   DatabaseTaskType = "update_password"
	TaskGetDatabases     DatabaseTaskType = "get_databases"
	TaskGetUsers         DatabaseTaskType = "get_users"
	TaskGetTables        DatabaseTaskType = "get_tables"
)

// Task type string constants for task identification (format: "database:{db_type}_{operation}")
const (
	// MySQL task types
	MySQLCreateDatabaseTaskType   = "database:mysql_create_database"
	MySQLDropDatabaseTaskType     = "database:mysql_drop_database"
	MySQLCreateUserTaskType       = "database:mysql_create_user"
	MySQLDropUserTaskType         = "database:mysql_drop_user"
	MySQLGrantPrivilegesTaskType  = "database:mysql_grant_privileges"
	MySQLRevokePrivilegesTaskType = "database:mysql_revoke_privileges"
	MySQLUpdatePasswordTaskType   = "database:mysql_update_password"
	MySQLGetDatabasesTaskType     = "database:mysql_get_databases"
	MySQLGetUsersTaskType         = "database:mysql_get_users"
	MySQLGetTablesTaskType        = "database:mysql_get_tables"

	// PostgreSQL task types
	PostgreSQLCreateDatabaseTaskType   = "database:postgresql_create_database"
	PostgreSQLDropDatabaseTaskType     = "database:postgresql_drop_database"
	PostgreSQLCreateUserTaskType       = "database:postgresql_create_user"
	PostgreSQLDropUserTaskType         = "database:postgresql_drop_user"
	PostgreSQLGrantPrivilegesTaskType  = "database:postgresql_grant_privileges"
	PostgreSQLRevokePrivilegesTaskType = "database:postgresql_revoke_privileges"
	PostgreSQLUpdatePasswordTaskType   = "database:postgresql_update_password"
	PostgreSQLGetDatabasesTaskType     = "database:postgresql_get_databases"
	PostgreSQLGetUsersTaskType         = "database:postgresql_get_users"
	PostgreSQLGetTablesTaskType        = "database:postgresql_get_tables"
)

type CreateDatabaseConfig struct {
	DatabaseName  string
	Owner         string
	AdminUser     string
	AdminPassword string
	Charset       string
	Collation     string
}

type DropDatabaseConfig struct {
	DatabaseName  string
	AdminUser     string
	AdminPassword string
}

type CreateUserConfig struct {
	Username      string
	Password      string
	AdminUser     string
	AdminPassword string
	Hosts         []string
}

type DropUserConfig struct {
	Username      string
	AdminUser     string
	AdminPassword string
	Hosts         []string
}

type GrantPrivilegesConfig struct {
	Username      string
	DatabaseName  string
	AdminUser     string
	AdminPassword string
	Hosts         []string
}

type RevokePrivilegesConfig struct {
	Username      string
	DatabaseName  string
	AdminUser     string
	AdminPassword string
	Hosts         []string
}

type UpdatePasswordConfig struct {
	Username      string
	NewPassword   string
	AdminUser     string
	AdminPassword string
	Hosts         []string
}

type GetDatabasesConfig struct {
	AdminUser     string
	AdminPassword string
}

type GetUsersConfig struct {
	AdminUser     string
	AdminPassword string
}

type GetTablesConfig struct {
	DatabaseName  string
	AdminUser     string
	AdminPassword string
}

type Factory struct {
	dbType servertypes.ServiceType
}

func NewFactory(dbType servertypes.ServiceType) *Factory {
	return &Factory{dbType: dbType}
}

func (f *Factory) IsMySQL() bool {
	return f.dbType == servertypes.ServiceTypeMySQL
}

func (f *Factory) IsPostgreSQL() bool {
	return f.dbType == servertypes.ServiceTypePostgreSQL
}

// TaskType returns the full task type string for a given operation based on the database type
func (f *Factory) TaskType(taskType DatabaseTaskType) string {
	prefix := "database:"
	if f.IsMySQL() {
		return prefix + "mysql_" + string(taskType)
	}
	return prefix + "postgresql_" + string(taskType)
}

func (f *Factory) CreateDatabase(config CreateDatabaseConfig) taskrunner.Task {
	if f.IsMySQL() {
		return MySQLCreateDatabase(MySQLCreateDatabaseConfig{
			User:         config.AdminUser,
			Password:     config.AdminPassword,
			DatabaseName: config.DatabaseName,
			Charset:      config.Charset,
			Collation:    config.Collation,
		})
	}
	return PostgreSQLCreateDatabase(PostgreSQLCreateDatabaseConfig{
		DatabaseName: config.DatabaseName,
		Owner:        config.Owner,
	})
}

func (f *Factory) DropDatabase(config DropDatabaseConfig) taskrunner.Task {
	if f.IsMySQL() {
		return MySQLDropDatabase(MySQLDropDatabaseConfig{
			User:         config.AdminUser,
			Password:     config.AdminPassword,
			DatabaseName: config.DatabaseName,
		})
	}
	return PostgreSQLDropDatabase(PostgreSQLDropDatabaseConfig{
		DatabaseName: config.DatabaseName,
	})
}

func (f *Factory) CreateUser(config CreateUserConfig) taskrunner.Task {
	if f.IsMySQL() {
		return MySQLCreateUser(MySQLCreateUserConfig{
			AdminUser:     config.AdminUser,
			AdminPassword: config.AdminPassword,
			Username:      config.Username,
			UserPassword:  config.Password,
			Hosts:         config.Hosts,
		})
	}
	return PostgreSQLCreateUser(PostgreSQLCreateUserConfig{
		Username: config.Username,
		Password: config.Password,
	})
}

func (f *Factory) DropUser(config DropUserConfig) taskrunner.Task {
	if f.IsMySQL() {
		return MySQLDropUser(MySQLDropUserConfig{
			AdminUser:     config.AdminUser,
			AdminPassword: config.AdminPassword,
			Username:      config.Username,
			Hosts:         config.Hosts,
		})
	}
	return PostgreSQLDropUser(PostgreSQLDropUserConfig{
		Username: config.Username,
	})
}

func (f *Factory) GrantPrivileges(config GrantPrivilegesConfig) taskrunner.Task {
	if f.IsMySQL() {
		return MySQLGrantPrivileges(MySQLGrantPrivilegesConfig{
			AdminUser:     config.AdminUser,
			AdminPassword: config.AdminPassword,
			Username:      config.Username,
			DatabaseName:  config.DatabaseName,
			Hosts:         config.Hosts,
		})
	}
	return PostgreSQLGrantPrivileges(PostgreSQLGrantPrivilegesConfig{
		Username:     config.Username,
		DatabaseName: config.DatabaseName,
	})
}

func (f *Factory) RevokePrivileges(config RevokePrivilegesConfig) taskrunner.Task {
	if f.IsMySQL() {
		return MySQLRevokePrivileges(MySQLRevokePrivilegesConfig{
			AdminUser:     config.AdminUser,
			AdminPassword: config.AdminPassword,
			Username:      config.Username,
			DatabaseName:  config.DatabaseName,
			Hosts:         config.Hosts,
		})
	}
	return PostgreSQLRevokePrivileges(PostgreSQLRevokePrivilegesConfig{
		Username:     config.Username,
		DatabaseName: config.DatabaseName,
	})
}

func (f *Factory) UpdatePassword(config UpdatePasswordConfig) taskrunner.Task {
	if f.IsMySQL() {
		return MySQLUpdatePassword(MySQLUpdatePasswordConfig{
			AdminUser:     config.AdminUser,
			AdminPassword: config.AdminPassword,
			Username:      config.Username,
			NewPassword:   config.NewPassword,
			Hosts:         config.Hosts,
		})
	}
	return PostgreSQLUpdatePassword(PostgreSQLUpdatePasswordConfig{
		Username:    config.Username,
		NewPassword: config.NewPassword,
	})
}

func (f *Factory) GetDatabases(config GetDatabasesConfig) taskrunner.Task {
	if f.IsMySQL() {
		return MySQLGetDatabases(MySQLGetDatabasesConfig{
			User:     config.AdminUser,
			Password: config.AdminPassword,
		})
	}
	return PostgreSQLGetDatabases()
}

func (f *Factory) GetUsers(config GetUsersConfig) taskrunner.Task {
	if f.IsMySQL() {
		return MySQLGetUsers(MySQLGetUsersConfig{
			User:     config.AdminUser,
			Password: config.AdminPassword,
		})
	}
	return PostgreSQLGetUsers()
}

func (f *Factory) GetTables(config GetTablesConfig) taskrunner.Task {
	if f.IsMySQL() {
		return MySQLGetTables(MySQLGetTablesConfig{
			User:         config.AdminUser,
			Password:     config.AdminPassword,
			DatabaseName: config.DatabaseName,
		})
	}
	return PostgreSQLGetTables(PostgreSQLGetTablesConfig{
		DatabaseName: config.DatabaseName,
	})
}

func NewFactoryFromString(dbType string) (*Factory, error) {
	switch dbType {
	case "mysql":
		return NewFactory(servertypes.ServiceTypeMySQL), nil
	case "postgresql":
		return NewFactory(servertypes.ServiceTypePostgreSQL), nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
}
