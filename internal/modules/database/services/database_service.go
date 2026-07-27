package services

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/database/dto"
	"github.com/kkz6/launch-go/internal/modules/database/jobs"
	"github.com/kkz6/launch-go/internal/modules/database/models"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
)

// CreateDatabase creates a new database on a server and returns the response DTO.
// Service signature follows the framework CreateNested convention:
// (ctx, parentID/serverID, teamID, userID, req).
func (s *Service) CreateDatabase(ctx context.Context, serverID, teamID, userID string, req *dto.CreateDatabaseRequest) (dto.DatabaseResponse, error) {
	database, err := s.buildAndDispatchDatabase(ctx, serverID, teamID, userID, req)
	if err != nil {
		return dto.DatabaseResponse{}, err
	}
	return dto.ToDatabaseResponse(database), nil
}

func (s *Service) buildAndDispatchDatabase(ctx context.Context, serverID, teamID, userID string, req *dto.CreateDatabaseRequest) (*models.Database, error) {
	exists, err := s.repos.Database().ExistsByNameAndServer(ctx, req.Name, serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing database: %w", err)
	}
	if exists {
		return nil, ErrDatabaseNameExists
	}

	if req.CreateUser {
		userExists, err := s.repos.User().ExistsByNameAndServer(ctx, req.UserName, serverID)
		if err != nil {
			return nil, fmt.Errorf("failed to check existing user: %w", err)
		}
		if userExists {
			return nil, ErrDatabaseUserNameExists
		}
	}

	database := &models.Database{Name: req.Name}
	database.ServerID = serverID
	database.TeamID = teamID

	var dbUser, existingUser *models.DatabaseUser

	err = s.WithTransaction(ctx, func(tx *gorm.DB) error {
		if err := tx.Create(database).Error; err != nil {
			return fmt.Errorf("failed to create database: %w", err)
		}

		if rootUser, rootErr := s.repos.User().FindRootUser(ctx, serverID); rootErr == nil {
			if err := tx.Create(&models.DatabaseDatabaseUser{DatabaseID: database.ID, DatabaseUserID: rootUser.ID}).Error; err != nil {
				return fmt.Errorf("failed to attach root user to database: %w", err)
			}
		}

		if !req.CreateUser && req.ExistingUserID != nil && *req.ExistingUserID != "" {
			user, err := s.repos.User().FindByIDAndServerAndTeam(ctx, *req.ExistingUserID, serverID, teamID)
			if err != nil {
				return fmt.Errorf("existing user not found: %w", err)
			}
			existingUser = user
			if err := tx.Create(&models.DatabaseDatabaseUser{DatabaseID: database.ID, DatabaseUserID: existingUser.ID}).Error; err != nil {
				return fmt.Errorf("failed to attach existing user to database: %w", err)
			}
			return nil
		}

		if req.CreateUser {
			password := &dbtype.EncryptedNullableString{}
			password.Set(req.UserPassword)

			dbUser = &models.DatabaseUser{Name: req.UserName, Password: password}
			dbUser.ServerID = serverID
			dbUser.TeamID = teamID

			if err := tx.Create(dbUser).Error; err != nil {
				return fmt.Errorf("failed to create database user: %w", err)
			}
			if err := tx.Create(&models.DatabaseDatabaseUser{DatabaseID: database.ID, DatabaseUserID: dbUser.ID}).Error; err != nil {
				return fmt.Errorf("failed to attach user to database: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	activity.RecordEvent(ctx, "created", userID, database, "Database was created")

	switch {
	case existingUser != nil:
		s.dispatchCreateDatabaseWithExistingUser(database, existingUser, userID)
	case dbUser != nil:
		s.dispatchCreateDatabaseWithNewUser(database, dbUser, req.UserPassword, userID)
	default:
		s.dispatchCreateDatabase(database, userID)
	}

	return database, nil
}

// GetDatabase retrieves a database by ID and returns the response DTO.
// Signature matches ShowNestedFunc: (ctx, id, parentID/serverID, teamID).
func (s *Service) GetDatabase(ctx context.Context, id, serverID, teamID string) (dto.DatabaseResponse, error) {
	database, err := s.repos.Database().FindByIDAndServerAndTeam(ctx, id, serverID, teamID)
	if err != nil {
		return dto.DatabaseResponse{}, err
	}
	return dto.ToDatabaseResponse(database), nil
}

// CreateDatabaseRaw is the model-returning entrypoint preserved for
// cross-module callers (site provisioning) that need the raw domain
// model. HTTP handlers should use CreateDatabase (DTO-returning).
func (s *Service) CreateDatabaseRaw(ctx context.Context, serverID, teamID string, req *dto.CreateDatabaseRequest, userID *string) (*models.Database, error) {
	uid := ""
	if userID != nil {
		uid = *userID
	}
	return s.buildAndDispatchDatabase(ctx, serverID, teamID, uid, req)
}

// GetDatabaseRaw is the model-returning fetch preserved for cross-module
// callers. HTTP handlers should use GetDatabase.
func (s *Service) GetDatabaseRaw(ctx context.Context, id, serverID, teamID string) (*models.Database, error) {
	return s.repos.Database().FindByIDAndServerAndTeam(ctx, id, serverID, teamID)
}

// GetDatabaseUserRaw is the model-returning fetch for cross-module
// callers. HTTP handlers should use GetDatabaseUser.
func (s *Service) GetDatabaseUserRaw(ctx context.Context, id, serverID string) (*models.DatabaseUser, error) {
	return s.repos.User().FindByIDAndServer(ctx, id, serverID)
}

// ListDatabases lists all databases for a server.
// Signature matches IndexNestedFunc: (ctx, parentID/serverID, teamID).
func (s *Service) ListDatabases(ctx context.Context, serverID, teamID string) ([]dto.DatabaseResponse, error) {
	databases, err := s.repos.Database().FindByServerAndTeam(ctx, serverID, teamID)
	if err != nil {
		return nil, err
	}
	resp := dto.ToDatabaseResponseList(databases)

	// Attach the backup configurations that include each database so
	// the Databases tab can render a "Run Backup" row action. Done via
	// a single join keyed by database_id to avoid the N+1 a per-row
	// repo call would produce. We deliberately query by raw table name
	// (backup_databases / backups) to keep the database module
	// independent of the backup module's Go types — the schema
	// contract is owned by migration 0048+ and is stable.
	if len(resp) == 0 || !s.HasDB() {
		return resp, nil
	}
	dbIDs := make([]string, 0, len(resp))
	for _, d := range resp {
		dbIDs = append(dbIDs, d.ID)
	}
	type backupRow struct {
		DatabaseID string `gorm:"column:database_id"`
		BackupID   string `gorm:"column:backup_id"`
		Path       string `gorm:"column:path"`
		Enabled    bool   `gorm:"column:enabled"`
	}
	var rows []backupRow
	// NOTE: the backups table has no soft-delete column (BaseModel only
	// carries id/created_at/updated_at), so we don't filter on deleted_at
	// — adding the column would be a schema change, not a query fix.
	err = s.DB().WithContext(ctx).
		Table("backup_databases AS bd").
		Select("bd.database_id, bd.backup_id, b.path, b.enabled").
		Joins("JOIN backups AS b ON b.id = bd.backup_id").
		Where("bd.database_id IN ?", dbIDs).
		Where("b.server_id = ?", serverID).
		Where("b.team_id = ?", teamID).
		Find(&rows).Error
	if err != nil {
		// Don't fail the whole list response if the join hits a transient
		// error — the row action just won't show. Surface in logs so we
		// notice; the rest of the page is unaffected.
		s.Logger.Warn().Err(err).Str("server_id", serverID).
			Msg("failed to load backup associations for databases list")
		return resp, nil
	}
	byDB := make(map[string][]dto.DatabaseBackupBrief, len(rows))
	for _, r := range rows {
		byDB[r.DatabaseID] = append(byDB[r.DatabaseID], dto.DatabaseBackupBrief{
			ID:      r.BackupID,
			Path:    r.Path,
			Enabled: r.Enabled,
		})
	}
	for i := range resp {
		if bs, ok := byDB[resp[i].ID]; ok {
			resp[i].Backups = bs
		}
	}
	return resp, nil
}

// DeleteDatabase marks a database for uninstallation and dispatches the
// uninstall job. Signature matches DeleteNestedFunc:
// (ctx, id, parentID/serverID, teamID, userID).
func (s *Service) DeleteDatabase(ctx context.Context, id, serverID, teamID, userID string) error {
	database, err := s.repos.Database().FindByIDAndServerAndTeam(ctx, id, serverID, teamID)
	if err != nil {
		return err
	}

	if database.IsUninstalling() {
		return ErrDatabaseBeingUninstalled
	}

	activity.RecordEvent(ctx, "deleted", userID, database, "Database deletion requested")

	if err := s.repos.Database().MarkAsUninstalling(ctx, database.ID); err != nil {
		return fmt.Errorf("failed to update database status: %w", err)
	}

	s.dispatchDeleteDatabase(database, userID)
	return nil
}

// SyncDatabases dispatches a job to sync the server's databases into the
// local DB. Signature matches ActionNestedFunc:
// (ctx, parentID/serverID, teamID, userID).
func (s *Service) SyncDatabases(ctx context.Context, serverID, teamID, userID string) error {
	task, err := jobs.NewSyncDatabasesTask(serverID, userIDPtr(userID))
	if err != nil {
		return err
	}
	return s.EnqueueTask(task)
}

// BroadcastDatabaseStatus broadcasts a database status update.
func (s *Service) BroadcastDatabaseStatus(serverID, databaseID, status, message string) {
	s.BroadcastToServer(serverID, "database.status", map[string]interface{}{
		"server_id":   serverID,
		"database_id": databaseID,
		"status":      status,
		"message":     message,
	})
}

// userIDPtr returns nil for empty userID, otherwise a pointer. Used at
// the boundary where downstream APIs (jobs, activity) accept *string.
func userIDPtr(userID string) *string {
	if userID == "" {
		return nil
	}
	return &userID
}

// Dispatch helpers.

func (s *Service) dispatchCreateDatabase(database *models.Database, userID string) {
	uid := userIDPtr(userID)
	s.DispatchTask("InstallDatabase", func() (*asynq.Task, error) {
		return jobs.NewInstallDatabaseTask(database.ID, uid)
	}, "database_id", database.ID)
}

func (s *Service) dispatchCreateDatabaseWithExistingUser(database *models.Database, user *models.DatabaseUser, userID string) {
	// NOTE: These two tasks have an ordering dependency (database must exist before granting
	// user permissions). The task implementations are idempotent. The asynq TaskID
	// deduplication also prevents duplicate concurrent execution.
	uid := userIDPtr(userID)
	s.DispatchTask("InstallDatabase", func() (*asynq.Task, error) {
		return jobs.NewInstallDatabaseTask(database.ID, uid)
	}, "database_id", database.ID)

	s.DispatchTask("UpdateDatabaseUser", func() (*asynq.Task, error) {
		return jobs.NewUpdateDatabaseUserTask(user.ID, nil, uid)
	}, "user_id", user.ID)
}

func (s *Service) dispatchCreateDatabaseWithNewUser(database *models.Database, user *models.DatabaseUser, password, userID string) {
	uid := userIDPtr(userID)
	s.DispatchTask("InstallDatabase", func() (*asynq.Task, error) {
		return jobs.NewInstallDatabaseTask(database.ID, uid)
	}, "database_id", database.ID)

	s.DispatchTask("InstallDatabaseUser", func() (*asynq.Task, error) {
		return jobs.NewInstallDatabaseUserTask(user.ID, password, uid)
	}, "user_id", user.ID)
}

func (s *Service) dispatchDeleteDatabase(database *models.Database, userID string) {
	uid := userIDPtr(userID)
	s.DispatchTask("UninstallDatabase", func() (*asynq.Task, error) {
		return jobs.NewUninstallDatabaseTask(database.ID, uid)
	}, "database_id", database.ID)
}
