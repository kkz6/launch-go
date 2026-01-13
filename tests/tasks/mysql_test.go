package tasks_test

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/database/tasks"
	"github.com/kkz6/launch-go/tests/testutil"
)

func TestMySQLCreateDatabase(t *testing.T) {
	config := tasks.MySQLCreateDatabaseConfig{
		User:         "root",
		Password:     "rootpassword",
		DatabaseName: "myapp_production",
		Charset:      "utf8mb4",
		Collation:    "utf8mb4_unicode_ci",
	}

	task := tasks.MySQLCreateDatabase(config)

	testutil.AssertTask(t, task).
		HasName("Create MySQL Database").
		ScriptContains("mysql").
		ScriptContains(config.DatabaseName).
		ScriptContains(config.Charset).
		ScriptContains(config.Collation).
		ScriptMatches("tasks/mysql_create_database")
}

func TestMySQLCreateDatabase_Defaults(t *testing.T) {
	config := tasks.MySQLCreateDatabaseConfig{
		User:         "root",
		Password:     "rootpassword",
		DatabaseName: "testdb",
	}

	task := tasks.MySQLCreateDatabase(config)

	testutil.AssertTask(t, task).
		HasName("Create MySQL Database").
		ScriptContains("utf8mb4").
		ScriptContains("utf8mb4_unicode_ci")
}

func TestMySQLDropDatabase(t *testing.T) {
	config := tasks.MySQLDropDatabaseConfig{
		User:         "root",
		Password:     "rootpassword",
		DatabaseName: "old_database",
	}

	task := tasks.MySQLDropDatabase(config)

	testutil.AssertTask(t, task).
		HasName("Drop MySQL Database").
		ScriptContains("DROP DATABASE").
		ScriptContains(config.DatabaseName).
		ScriptMatches("tasks/mysql_drop_database")
}

func TestMySQLCreateUser(t *testing.T) {
	config := tasks.MySQLCreateUserConfig{
		AdminUser:     "root",
		AdminPassword: "rootpassword",
		Username:      "appuser",
		UserPassword:  "apppassword",
		Hosts:         []string{"%", "localhost"},
	}

	task := tasks.MySQLCreateUser(config)

	testutil.AssertTask(t, task).
		HasName("Create MySQL User").
		ScriptContains("CREATE USER").
		ScriptContains(config.Username).
		ScriptMatches("tasks/mysql_create_user")
}

func TestMySQLCreateUser_DefaultHost(t *testing.T) {
	config := tasks.MySQLCreateUserConfig{
		AdminUser:     "root",
		AdminPassword: "rootpassword",
		Username:      "appuser",
		UserPassword:  "apppassword",
	}

	task := tasks.MySQLCreateUser(config)

	testutil.AssertTask(t, task).
		HasName("Create MySQL User").
		ScriptContains("'%'") // Default host
}

func TestMySQLDropUser(t *testing.T) {
	config := tasks.MySQLDropUserConfig{
		AdminUser:     "root",
		AdminPassword: "rootpassword",
		Username:      "olduser",
		Hosts:         []string{"%"},
	}

	task := tasks.MySQLDropUser(config)

	testutil.AssertTask(t, task).
		HasName("Drop MySQL User").
		ScriptContains("DROP USER").
		ScriptContains(config.Username).
		ScriptMatches("tasks/mysql_drop_user")
}

func TestMySQLGrantPrivileges(t *testing.T) {
	config := tasks.MySQLGrantPrivilegesConfig{
		AdminUser:     "root",
		AdminPassword: "rootpassword",
		Username:      "appuser",
		DatabaseName:  "myapp",
		Hosts:         []string{"%"},
	}

	task := tasks.MySQLGrantPrivileges(config)

	testutil.AssertTask(t, task).
		HasName("Grant MySQL Privileges").
		ScriptContains("GRANT").
		ScriptContains(config.DatabaseName).
		ScriptContains(config.Username).
		ScriptMatches("tasks/mysql_grant_privileges")
}

func TestMySQLRevokePrivileges(t *testing.T) {
	config := tasks.MySQLRevokePrivilegesConfig{
		AdminUser:     "root",
		AdminPassword: "rootpassword",
		Username:      "appuser",
		DatabaseName:  "myapp",
		Hosts:         []string{"%"},
	}

	task := tasks.MySQLRevokePrivileges(config)

	testutil.AssertTask(t, task).
		HasName("Revoke MySQL Privileges").
		ScriptContains("REVOKE").
		ScriptContains(config.DatabaseName).
		ScriptContains(config.Username).
		ScriptMatches("tasks/mysql_revoke_privileges")
}

func TestMySQLUpdatePassword(t *testing.T) {
	config := tasks.MySQLUpdatePasswordConfig{
		AdminUser:     "root",
		AdminPassword: "rootpassword",
		Username:      "appuser",
		NewPassword:   "newpassword123",
		Hosts:         []string{"%"},
	}

	task := tasks.MySQLUpdatePassword(config)

	testutil.AssertTask(t, task).
		HasName("Update MySQL Password").
		ScriptContains("ALTER USER").
		ScriptContains(config.Username).
		ScriptMatches("tasks/mysql_update_password")
}

func TestMySQLGetDatabases(t *testing.T) {
	config := tasks.MySQLGetDatabasesConfig{
		User:     "root",
		Password: "rootpassword",
	}

	task := tasks.MySQLGetDatabases(config)

	testutil.AssertTask(t, task).
		HasName("Get MySQL Databases").
		ScriptContains("SHOW DATABASES").
		ScriptMatches("tasks/mysql_get_databases")
}

func TestMySQLGetUsers(t *testing.T) {
	config := tasks.MySQLGetUsersConfig{
		User:     "root",
		Password: "rootpassword",
	}

	task := tasks.MySQLGetUsers(config)

	testutil.AssertTask(t, task).
		HasName("Get MySQL Users").
		ScriptContains("mysql.user").
		ScriptMatches("tasks/mysql_get_users")
}

func TestMySQLGetTables(t *testing.T) {
	config := tasks.MySQLGetTablesConfig{
		User:         "root",
		Password:     "rootpassword",
		DatabaseName: "myapp",
	}

	task := tasks.MySQLGetTables(config)

	testutil.AssertTask(t, task).
		HasName("Get MySQL Tables").
		ScriptContains(config.DatabaseName).
		ScriptMatches("tasks/mysql_get_tables")
}
