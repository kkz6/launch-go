package tasks_test

import (
	"testing"

	"github.com/kkz6/launch-go/internal/modules/database/tasks"
	"github.com/kkz6/launch-go/tests/testutil"
)

func TestPostgreSQLCreateDatabase(t *testing.T) {
	config := tasks.PostgreSQLCreateDatabaseConfig{
		DatabaseName: "myapp_production",
		Owner:        "appuser",
	}

	task := tasks.PostgreSQLCreateDatabase(config)

	testutil.AssertTask(t, task).
		HasName("Create PostgreSQL Database").
		ScriptContains("CREATE DATABASE").
		ScriptContains(config.DatabaseName).
		ScriptContains(config.Owner).
		ScriptMatches("tasks/postgresql_create_database")
}

func TestPostgreSQLDropDatabase(t *testing.T) {
	config := tasks.PostgreSQLDropDatabaseConfig{
		DatabaseName: "old_database",
	}

	task := tasks.PostgreSQLDropDatabase(config)

	testutil.AssertTask(t, task).
		HasName("Drop PostgreSQL Database").
		ScriptContains("DROP DATABASE").
		ScriptContains(config.DatabaseName).
		ScriptMatches("tasks/postgresql_drop_database")
}

func TestPostgreSQLCreateUser(t *testing.T) {
	config := tasks.PostgreSQLCreateUserConfig{
		Username: "appuser",
		Password: "apppassword",
	}

	task := tasks.PostgreSQLCreateUser(config)

	testutil.AssertTask(t, task).
		HasName("Create PostgreSQL User").
		ScriptContains("CREATE USER").
		ScriptContains(config.Username).
		ScriptMatches("tasks/postgresql_create_user")
}

func TestPostgreSQLDropUser(t *testing.T) {
	config := tasks.PostgreSQLDropUserConfig{
		Username: "olduser",
	}

	task := tasks.PostgreSQLDropUser(config)

	testutil.AssertTask(t, task).
		HasName("Drop PostgreSQL User").
		ScriptContains("DROP USER").
		ScriptContains(config.Username).
		ScriptMatches("tasks/postgresql_drop_user")
}

func TestPostgreSQLGrantPrivileges(t *testing.T) {
	config := tasks.PostgreSQLGrantPrivilegesConfig{
		Username:     "appuser",
		DatabaseName: "myapp",
	}

	task := tasks.PostgreSQLGrantPrivileges(config)

	testutil.AssertTask(t, task).
		HasName("Grant PostgreSQL Privileges").
		ScriptContains("GRANT").
		ScriptContains(config.DatabaseName).
		ScriptContains(config.Username).
		ScriptMatches("tasks/postgresql_grant_privileges")
}

func TestPostgreSQLRevokePrivileges(t *testing.T) {
	config := tasks.PostgreSQLRevokePrivilegesConfig{
		Username:     "appuser",
		DatabaseName: "myapp",
	}

	task := tasks.PostgreSQLRevokePrivileges(config)

	testutil.AssertTask(t, task).
		HasName("Revoke PostgreSQL Privileges").
		ScriptContains("REVOKE").
		ScriptContains(config.DatabaseName).
		ScriptContains(config.Username).
		ScriptMatches("tasks/postgresql_revoke_privileges")
}

func TestPostgreSQLUpdatePassword(t *testing.T) {
	config := tasks.PostgreSQLUpdatePasswordConfig{
		Username:    "appuser",
		NewPassword: "newpassword123",
	}

	task := tasks.PostgreSQLUpdatePassword(config)

	testutil.AssertTask(t, task).
		HasName("Update PostgreSQL Password").
		ScriptContains("ALTER USER").
		ScriptContains(config.Username).
		ScriptMatches("tasks/postgresql_update_password")
}

func TestPostgreSQLGetDatabases(t *testing.T) {
	task := tasks.PostgreSQLGetDatabases()

	testutil.AssertTask(t, task).
		HasName("Get PostgreSQL Databases").
		ScriptContains("psql").
		ScriptMatches("tasks/postgresql_get_databases")
}

func TestPostgreSQLGetUsers(t *testing.T) {
	task := tasks.PostgreSQLGetUsers()

	testutil.AssertTask(t, task).
		HasName("Get PostgreSQL Users").
		ScriptContains("psql").
		ScriptMatches("tasks/postgresql_get_users")
}

func TestPostgreSQLGetTables(t *testing.T) {
	config := tasks.PostgreSQLGetTablesConfig{
		DatabaseName: "myapp",
	}

	task := tasks.PostgreSQLGetTables(config)

	testutil.AssertTask(t, task).
		HasName("Get PostgreSQL Tables").
		ScriptContains(config.DatabaseName).
		ScriptMatches("tasks/postgresql_get_tables")
}
