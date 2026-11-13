package tasks

import "github.com/kkz6/launch-go/internal/pkg/taskrunner"

// MySQLCreateDatabaseConfig holds configuration for creating a MySQL database
type MySQLCreateDatabaseConfig struct {
	User         string
	Password     string
	DatabaseName string
	Charset      string
	Collation    string
}

// MySQLCreateDatabase creates a task to create a MySQL database
func MySQLCreateDatabase(config MySQLCreateDatabaseConfig) *taskrunner.BaseTask {
	if config.Charset == "" {
		config.Charset = "utf8mb4"
	}
	if config.Collation == "" {
		config.Collation = "utf8mb4_unicode_ci"
	}
	return newDatabaseTask("Create MySQL Database", "mysql/create_database.sh", config, 30)
}

// MySQLDropDatabaseConfig holds configuration for dropping a MySQL database
type MySQLDropDatabaseConfig struct {
	User         string
	Password     string
	DatabaseName string
}

// MySQLDropDatabase creates a task to drop a MySQL database
func MySQLDropDatabase(config MySQLDropDatabaseConfig) *taskrunner.BaseTask {
	return newDatabaseTask("Drop MySQL Database", "mysql/drop_database.sh", config, 30)
}

// MySQLCreateUserConfig holds configuration for creating a MySQL user
type MySQLCreateUserConfig struct {
	AdminUser     string
	AdminPassword string
	Username      string
	UserPassword  string
	Hosts         []string
}

// MySQLCreateUser creates a task to create a MySQL user
func MySQLCreateUser(config MySQLCreateUserConfig) *taskrunner.BaseTask {
	if len(config.Hosts) == 0 {
		config.Hosts = []string{"%"}
	}
	return newDatabaseTask("Create MySQL User", "mysql/create_user.sh", config, 30)
}

// MySQLDropUserConfig holds configuration for dropping a MySQL user
type MySQLDropUserConfig struct {
	AdminUser     string
	AdminPassword string
	Username      string
	Hosts         []string
}

// MySQLDropUser creates a task to drop a MySQL user
func MySQLDropUser(config MySQLDropUserConfig) *taskrunner.BaseTask {
	if len(config.Hosts) == 0 {
		config.Hosts = []string{"%"}
	}
	return newDatabaseTask("Drop MySQL User", "mysql/drop_user.sh", config, 30)
}

// MySQLGrantPrivilegesConfig holds configuration for granting MySQL privileges
type MySQLGrantPrivilegesConfig struct {
	AdminUser     string
	AdminPassword string
	Username      string
	DatabaseName  string
	Hosts         []string
}

// MySQLGrantPrivileges creates a task to grant privileges to a MySQL user
func MySQLGrantPrivileges(config MySQLGrantPrivilegesConfig) *taskrunner.BaseTask {
	if len(config.Hosts) == 0 {
		config.Hosts = []string{"%"}
	}
	return newDatabaseTask("Grant MySQL Privileges", "mysql/grant_privileges.sh", config, 30)
}

// MySQLRevokePrivilegesConfig holds configuration for revoking MySQL privileges
type MySQLRevokePrivilegesConfig struct {
	AdminUser     string
	AdminPassword string
	Username      string
	DatabaseName  string
	Hosts         []string
}

// MySQLRevokePrivileges creates a task to revoke privileges from a MySQL user
func MySQLRevokePrivileges(config MySQLRevokePrivilegesConfig) *taskrunner.BaseTask {
	if len(config.Hosts) == 0 {
		config.Hosts = []string{"%"}
	}
	return newDatabaseTask("Revoke MySQL Privileges", "mysql/revoke_privileges.sh", config, 30)
}

// MySQLUpdatePasswordConfig holds configuration for updating a MySQL user's password
type MySQLUpdatePasswordConfig struct {
	AdminUser     string
	AdminPassword string
	Username      string
	NewPassword   string
	Hosts         []string
}

// MySQLUpdatePassword creates a task to update a MySQL user's password
func MySQLUpdatePassword(config MySQLUpdatePasswordConfig) *taskrunner.BaseTask {
	if len(config.Hosts) == 0 {
		config.Hosts = []string{"%"}
	}
	return newDatabaseTask("Update MySQL Password", "mysql/update_password.sh", config, 30)
}

// MySQLGetDatabasesConfig holds configuration for getting MySQL databases
type MySQLGetDatabasesConfig struct {
	User     string
	Password string
}

// MySQLGetDatabases creates a task to list all MySQL databases
func MySQLGetDatabases(config MySQLGetDatabasesConfig) *taskrunner.BaseTask {
	return newDatabaseTask("Get MySQL Databases", "mysql/get_databases.sh", config, 15)
}

// MySQLGetUsersConfig holds configuration for getting MySQL users
type MySQLGetUsersConfig struct {
	User     string
	Password string
}

// MySQLGetUsers creates a task to list all MySQL users
func MySQLGetUsers(config MySQLGetUsersConfig) *taskrunner.BaseTask {
	return newDatabaseTask("Get MySQL Users", "mysql/get_users.sh", config, 15)
}

// MySQLGetTablesConfig holds configuration for getting MySQL tables
type MySQLGetTablesConfig struct {
	User         string
	Password     string
	DatabaseName string
}

// MySQLGetTables creates a task to list all tables in a MySQL database
func MySQLGetTables(config MySQLGetTablesConfig) *taskrunner.BaseTask {
	return newDatabaseTask("Get MySQL Tables", "mysql/get_tables.sh", config, 15)
}
