package tasks

import (
	"github.com/kkz6/launch-go/internal/modules/database/tasks/templates"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// PostgreSQLCreateDatabaseConfig holds configuration for creating a PostgreSQL database
type PostgreSQLCreateDatabaseConfig struct {
	DatabaseName string
	Owner        string
}

// PostgreSQLCreateDatabase creates a task to create a PostgreSQL database
func PostgreSQLCreateDatabase(config PostgreSQLCreateDatabaseConfig) *taskrunner.BaseTask {
	script := templates.MustRender("postgresql/create_database.sh", config)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Create PostgreSQL Database"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// PostgreSQLDropDatabaseConfig holds configuration for dropping a PostgreSQL database
type PostgreSQLDropDatabaseConfig struct {
	DatabaseName string
}

// PostgreSQLDropDatabase creates a task to drop a PostgreSQL database
func PostgreSQLDropDatabase(config PostgreSQLDropDatabaseConfig) *taskrunner.BaseTask {
	script := templates.MustRender("postgresql/drop_database.sh", config)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Drop PostgreSQL Database"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// PostgreSQLCreateUserConfig holds configuration for creating a PostgreSQL user
type PostgreSQLCreateUserConfig struct {
	Username string
	Password string
}

// PostgreSQLCreateUser creates a task to create a PostgreSQL user
func PostgreSQLCreateUser(config PostgreSQLCreateUserConfig) *taskrunner.BaseTask {
	script := templates.MustRender("postgresql/create_user.sh", config)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Create PostgreSQL User"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// PostgreSQLDropUserConfig holds configuration for dropping a PostgreSQL user
type PostgreSQLDropUserConfig struct {
	Username string
}

// PostgreSQLDropUser creates a task to drop a PostgreSQL user
func PostgreSQLDropUser(config PostgreSQLDropUserConfig) *taskrunner.BaseTask {
	script := templates.MustRender("postgresql/drop_user.sh", config)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Drop PostgreSQL User"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// PostgreSQLGrantPrivilegesConfig holds configuration for granting PostgreSQL privileges
type PostgreSQLGrantPrivilegesConfig struct {
	Username     string
	DatabaseName string
}

// PostgreSQLGrantPrivileges creates a task to grant privileges to a PostgreSQL user
func PostgreSQLGrantPrivileges(config PostgreSQLGrantPrivilegesConfig) *taskrunner.BaseTask {
	script := templates.MustRender("postgresql/grant_privileges.sh", config)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Grant PostgreSQL Privileges"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// PostgreSQLRevokePrivilegesConfig holds configuration for revoking PostgreSQL privileges
type PostgreSQLRevokePrivilegesConfig struct {
	Username     string
	DatabaseName string
}

// PostgreSQLRevokePrivileges creates a task to revoke privileges from a PostgreSQL user
func PostgreSQLRevokePrivileges(config PostgreSQLRevokePrivilegesConfig) *taskrunner.BaseTask {
	script := templates.MustRender("postgresql/revoke_privileges.sh", config)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Revoke PostgreSQL Privileges"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// PostgreSQLUpdatePasswordConfig holds configuration for updating a PostgreSQL user's password
type PostgreSQLUpdatePasswordConfig struct {
	Username    string
	NewPassword string
}

// PostgreSQLUpdatePassword creates a task to update a PostgreSQL user's password
func PostgreSQLUpdatePassword(config PostgreSQLUpdatePasswordConfig) *taskrunner.BaseTask {
	script := templates.MustRender("postgresql/update_password.sh", config)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Update PostgreSQL Password"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(30),
	)
}

// PostgreSQLGetDatabases creates a task to list all PostgreSQL databases
func PostgreSQLGetDatabases() *taskrunner.BaseTask {
	script := templates.MustRender("postgresql/get_databases.sh", nil)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Get PostgreSQL Databases"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(15),
	)
}

// PostgreSQLGetUsers creates a task to list all PostgreSQL users
func PostgreSQLGetUsers() *taskrunner.BaseTask {
	script := templates.MustRender("postgresql/get_users.sh", nil)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Get PostgreSQL Users"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(15),
	)
}

// PostgreSQLGetTablesConfig holds configuration for getting PostgreSQL tables
type PostgreSQLGetTablesConfig struct {
	DatabaseName string
}

// PostgreSQLGetTables creates a task to list all tables in a PostgreSQL database
func PostgreSQLGetTables(config PostgreSQLGetTablesConfig) *taskrunner.BaseTask {
	script := templates.MustRender("postgresql/get_tables.sh", config)
	return taskrunner.NewBaseTask(
		taskrunner.WithName("Get PostgreSQL Tables"),
		taskrunner.WithScript(script),
		taskrunner.WithTimeoutSeconds(15),
	)
}
