package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/database/models"
	"github.com/kkz6/launch-go/internal/modules/database/tasks"
	serverenums "github.com/kkz6/launch-go/internal/modules/server/enums"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/repository"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

// InstallDatabaseUserJob handles database user installation on a server.
// Similar to Laravel's Modules\Database\Jobs\InstallDatabaseUser
type InstallDatabaseUserJob struct {
	DatabaseJobBase
	jobs.InstallationTracker
	Payload InstallDatabaseUserPayload
}

// Type returns the job type identifier.
func (j *InstallDatabaseUserJob) Type() string {
	return TypeInstallDatabaseUser
}

// Handle processes the install database user job.
func (j *InstallDatabaseUserJob) Handle(ctx context.Context) error {
	j.LogInfo("Installing database user", "database_user_id", j.Payload.DatabaseUserID)

	// Fetch the database user with databases
	dbUser, err := repository.NewQuery[models.DatabaseUser](j.DB, ctx).
		WithModel("DatabaseUser").
		Preload("Databases").
		FindByID(j.Payload.DatabaseUserID).
		FirstOrFail()
	if err != nil {
		return fmt.Errorf("failed to find database user: %w", err)
	}

	// Fetch the server
	server, err := repository.Find[servermodels.Server](j.DB, ctx, dbUser.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.broadcastProgress(dbUser.ServerID, j.Payload.DatabaseUserID, "installing", fmt.Sprintf("Creating database user: %s", dbUser.Name))

	// Determine database type from installed services
	dbType := j.getDatabaseType(ctx, dbUser.ServerID)

	if dbType == "mysql" {
		// Create MySQL user
		createUserTask := tasks.MySQLCreateUser(tasks.MySQLCreateUserConfig{
			AdminUser:     "root",
			AdminPassword: server.DatabasePassword.String(),
			Username:      dbUser.Name,
			UserPassword:  j.Payload.Password,
			Hosts:         []string{"%"},
		})

		result, err := j.RunTaskOnServer(server, createUserTask).AsRoot().Dispatch(ctx)
		if err != nil {
			return fmt.Errorf("failed to create database user: %w", err)
		}

		if !result.IsSuccessful() {
			return fmt.Errorf("failed to create database user: %s", result.GetOutput())
		}

		// Grant privileges on each associated database
		for _, db := range dbUser.Databases {
			grantTask := tasks.MySQLGrantPrivileges(tasks.MySQLGrantPrivilegesConfig{
				AdminUser:     "root",
				AdminPassword: server.DatabasePassword.String(),
				Username:      dbUser.Name,
				DatabaseName:  db.Name,
				Hosts:         []string{"%"},
			})

			if err := j.runTask(ctx, server, grantTask); err != nil {
				j.LogError(err, "Failed to grant privileges", "database", db.Name)
			}
		}
	} else {
		// Create PostgreSQL user
		createUserTask := tasks.PostgreSQLCreateUser(tasks.PostgreSQLCreateUserConfig{
			Username: dbUser.Name,
			Password: j.Payload.Password,
		})

		result, err := j.RunTaskOnServer(server, createUserTask).AsRoot().Dispatch(ctx)
		if err != nil {
			return fmt.Errorf("failed to create database user: %w", err)
		}

		if !result.IsSuccessful() {
			return fmt.Errorf("failed to create database user: %s", result.GetOutput())
		}

		// Grant privileges on each associated database
		for _, db := range dbUser.Databases {
			grantTask := tasks.PostgreSQLGrantPrivileges(tasks.PostgreSQLGrantPrivilegesConfig{
				Username:     dbUser.Name,
				DatabaseName: db.Name,
			})

			if err := j.runTask(ctx, server, grantTask); err != nil {
				j.LogError(err, "Failed to grant privileges", "database", db.Name)
			}
		}
	}

	// Mark the database user as installed
	if err := j.MarkAsInstalled(j.DB, dbUser); err != nil {
		return fmt.Errorf("failed to update database user status: %w", err)
	}

	j.broadcastProgress(dbUser.ServerID, j.Payload.DatabaseUserID, "installed", fmt.Sprintf("Database user %s created successfully", dbUser.Name))

	return nil
}

// runTask is a helper to run a task and handle errors.
func (j *InstallDatabaseUserJob) runTask(ctx context.Context, server *servermodels.Server, task taskrunner.Task) error {
	result, err := j.RunTaskOnServer(server, task).AsRoot().Dispatch(ctx)
	if err != nil {
		return err
	}
	if !result.IsSuccessful() {
		j.LogInfo("Task completed with errors", "output", result.GetOutput())
	}
	return nil
}

// Failed is called when the job fails after all retries.
func (j *InstallDatabaseUserJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to install database user", "database_user_id", j.Payload.DatabaseUserID)

	// Fetch the database user to get server ID for broadcasting
	dbUser, findErr := repository.Find[models.DatabaseUser](j.DB, ctx, j.Payload.DatabaseUserID)
	if findErr != nil {
		return
	}

	// Mark installation as failed
	j.MarkInstallationFailed(j.DB, dbUser)

	j.broadcastProgress(dbUser.ServerID, j.Payload.DatabaseUserID, "failed", fmt.Sprintf("Failed to create database user: %s", dbUser.Name))
}

func (j *InstallDatabaseUserJob) broadcastProgress(serverID, userID, status, message string) {
	j.BroadcastDatabaseEvent(serverID, "database_user.progress", map[string]any{
		"server_id": serverID,
		"user_id":   userID,
		"status":    status,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// getDatabaseType determines the database type from server's installed services.
func (j *InstallDatabaseUserJob) getDatabaseType(ctx context.Context, serverID string) string {
	service, err := repository.NewQuery[servermodels.InstalledService](j.DB, ctx).
		Where("server_id = ? AND type IN ?", serverID, []string{
			string(serverenums.ServiceTypeMySql),
			string(serverenums.ServiceTypePostgreSql),
		}).
		First()

	if err != nil {
		return "mysql" // Default to MySQL
	}

	if service.Type == serverenums.ServiceTypePostgreSql {
		return "postgresql"
	}
	return "mysql"
}
