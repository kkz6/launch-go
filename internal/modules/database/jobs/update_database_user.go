package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/kkz6/launch-go/internal/modules/database/models"
	"github.com/kkz6/launch-go/internal/modules/database/tasks"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/pkg/repository"
)

type UpdateDatabaseUserJob struct {
	DatabaseJobBase
	Payload UpdateDatabaseUserPayload
}

func (j *UpdateDatabaseUserJob) Type() string {
	return TypeUpdateDatabaseUser
}

func (j *UpdateDatabaseUserJob) Handle(ctx context.Context) error {
	j.LogInfo("Updating database user", "database_user_id", j.Payload.DatabaseUserID)

	dbUser, err := repository.NewQuery[models.DatabaseUser](j.DB, ctx).
		WithModel("DatabaseUser").
		Preload("Databases").
		FindByID(j.Payload.DatabaseUserID).
		FirstOrFail()
	if err != nil {
		return fmt.Errorf("failed to find database user: %w", err)
	}

	server, err := repository.Find[servermodels.Server](j.DB, ctx, dbUser.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.BroadcastUserProgress(server, "database_user.progress", j.Payload.DatabaseUserID, "updating", fmt.Sprintf("Updating database user: %s", dbUser.Name))

	if j.Payload.Password != nil && *j.Payload.Password != "" {
		factory := j.GetTaskFactory(ctx, dbUser.ServerID)
		task := factory.UpdatePassword(tasks.UpdatePasswordConfig{
			Username:      dbUser.Name,
			NewPassword:   *j.Payload.Password,
			AdminUser:     "root",
			AdminPassword: server.DatabasePassword.String(),
			Hosts:         []string{"%"},
		})

		result, err := j.RunTaskOnServer(server, task).
			AsRoot().
			Dispatch(ctx)
		if err != nil {
			return fmt.Errorf("failed to update database user password: %w", err)
		}

		if !result.IsSuccessful() {
			return fmt.Errorf("failed to update database user password: %s", result.GetOutput())
		}
	}

	now := time.Now()
	if err := j.DB.WithContext(ctx).Model(dbUser).Update("updated_at", &now).Error; err != nil {
		return fmt.Errorf("failed to update database user record: %w", err)
	}

	j.BroadcastUserProgress(server, "database_user.progress", j.Payload.DatabaseUserID, "updated", fmt.Sprintf("Database user %s updated successfully", dbUser.Name))

	return nil
}

func (j *UpdateDatabaseUserJob) Failed(ctx context.Context, err error) {
	j.LogError(err, "Failed to update database user", "database_user_id", j.Payload.DatabaseUserID)

	dbUser, findErr := repository.Find[models.DatabaseUser](j.DB, ctx, j.Payload.DatabaseUserID)
	if findErr != nil {
		return
	}

	server, findErr := repository.Find[servermodels.Server](j.DB, ctx, dbUser.ServerID)
	if findErr != nil {
		return
	}

	j.BroadcastUserProgress(server, "database_user.progress", j.Payload.DatabaseUserID, "failed", fmt.Sprintf("Failed to update database user: %s", dbUser.Name))
}
