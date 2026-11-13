package tasks

import "github.com/kkz6/launch-go/internal/pkg/taskrunner"

// PostgreSQLCreateDatabaseConfig holds configuration for creating a PostgreSQL database
type PostgreSQLCreateDatabaseConfig struct {
	DatabaseName string
	Owner        string
}

// PostgreSQLCreateDatabase creates a task to create a PostgreSQL database
func PostgreSQLCreateDatabase(config PostgreSQLCreateDatabaseConfig) *taskrunner.BaseTask {
	return newDatabaseTask("Create PostgreSQL Database", "postgresql/create_database.sh", config, 30)
}

// PostgreSQLDropDatabaseConfig holds configuration for dropping a PostgreSQL database
type PostgreSQLDropDatabaseConfig struct {
	DatabaseName string
}

// PostgreSQLDropDatabase creates a task to drop a PostgreSQL database
func PostgreSQLDropDatabase(config PostgreSQLDropDatabaseConfig) *taskrunner.BaseTask {
	return newDatabaseTask("Drop PostgreSQL Database", "postgresql/drop_database.sh", config, 30)
}

// PostgreSQLCreateUserConfig holds configuration for creating a PostgreSQL user
type PostgreSQLCreateUserConfig struct {
	Username string
	Password string
}

// PostgreSQLCreateUser creates a task to create a PostgreSQL user
func PostgreSQLCreateUser(config PostgreSQLCreateUserConfig) *taskrunner.BaseTask {
	return newDatabaseTask("Create PostgreSQL User", "postgresql/create_user.sh", config, 30)
}

// PostgreSQLDropUserConfig holds configuration for dropping a PostgreSQL user
type PostgreSQLDropUserConfig struct {
	Username string
}

// PostgreSQLDropUser creates a task to drop a PostgreSQL user
func PostgreSQLDropUser(config PostgreSQLDropUserConfig) *taskrunner.BaseTask {
	return newDatabaseTask("Drop PostgreSQL User", "postgresql/drop_user.sh", config, 30)
}

// PostgreSQLGrantPrivilegesConfig holds configuration for granting PostgreSQL privileges
type PostgreSQLGrantPrivilegesConfig struct {
	Username     string
	DatabaseName string
}

// PostgreSQLGrantPrivileges creates a task to grant privileges to a PostgreSQL user
func PostgreSQLGrantPrivileges(config PostgreSQLGrantPrivilegesConfig) *taskrunner.BaseTask {
	return newDatabaseTask("Grant PostgreSQL Privileges", "postgresql/grant_privileges.sh", config, 30)
}

// PostgreSQLRevokePrivilegesConfig holds configuration for revoking PostgreSQL privileges
type PostgreSQLRevokePrivilegesConfig struct {
	Username     string
	DatabaseName string
}

// PostgreSQLRevokePrivileges creates a task to revoke privileges from a PostgreSQL user
func PostgreSQLRevokePrivileges(config PostgreSQLRevokePrivilegesConfig) *taskrunner.BaseTask {
	return newDatabaseTask("Revoke PostgreSQL Privileges", "postgresql/revoke_privileges.sh", config, 30)
}

// PostgreSQLUpdatePasswordConfig holds configuration for updating a PostgreSQL user's password
type PostgreSQLUpdatePasswordConfig struct {
	Username    string
	NewPassword string
}

// PostgreSQLUpdatePassword creates a task to update a PostgreSQL user's password
func PostgreSQLUpdatePassword(config PostgreSQLUpdatePasswordConfig) *taskrunner.BaseTask {
	return newDatabaseTask("Update PostgreSQL Password", "postgresql/update_password.sh", config, 30)
}

// PostgreSQLGetDatabases creates a task to list all PostgreSQL databases
func PostgreSQLGetDatabases() *taskrunner.BaseTask {
	return newDatabaseTask("Get PostgreSQL Databases", "postgresql/get_databases.sh", nil, 15)
}

// PostgreSQLGetUsers creates a task to list all PostgreSQL users
func PostgreSQLGetUsers() *taskrunner.BaseTask {
	return newDatabaseTask("Get PostgreSQL Users", "postgresql/get_users.sh", nil, 15)
}

// PostgreSQLGetTablesConfig holds configuration for getting PostgreSQL tables
type PostgreSQLGetTablesConfig struct {
	DatabaseName string
}

// PostgreSQLGetTables creates a task to list all tables in a PostgreSQL database
func PostgreSQLGetTables(config PostgreSQLGetTablesConfig) *taskrunner.BaseTask {
	return newDatabaseTask("Get PostgreSQL Tables", "postgresql/get_tables.sh", config, 15)
}
