package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/hibiken/asynq"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/database/dto"
	"github.com/kkz6/launch-go/internal/modules/database/jobs"
	"github.com/kkz6/launch-go/internal/modules/database/models"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
)

// CreateDatabaseUser creates a new database user under a server and
// returns the response DTO. Signature matches CreateNestedFunc.
func (s *Service) CreateDatabaseUser(ctx context.Context, serverID, teamID, userID string, req *dto.CreateDatabaseUserRequest) (dto.DatabaseUserResponse, error) {
	dbUser, err := s.buildAndDispatchDatabaseUser(ctx, serverID, teamID, userID, req)
	if err != nil {
		return dto.DatabaseUserResponse{}, err
	}
	return dto.ToDatabaseUserResponse(dbUser), nil
}

func (s *Service) buildAndDispatchDatabaseUser(ctx context.Context, serverID, teamID, userID string, req *dto.CreateDatabaseUserRequest) (*models.DatabaseUser, error) {
	exists, err := s.repos.User().ExistsByNameAndServer(ctx, req.Name, serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}
	if exists {
		return nil, ErrDatabaseUserNameExists
	}

	if len(req.Databases) > 0 {
		validDBs, err := s.repos.Database().FindByIDsAndServer(ctx, req.Databases, serverID)
		if err != nil {
			return nil, fmt.Errorf("failed to validate databases: %w", err)
		}
		if len(validDBs) != len(req.Databases) {
			return nil, errors.New("one or more databases not found on this server")
		}
	}

	password := &dbtype.EncryptedNullableString{}
	password.Set(req.Password)

	dbUser := &models.DatabaseUser{Name: req.Name, Password: password}
	dbUser.ServerID = serverID
	dbUser.TeamID = teamID

	err = s.WithTransaction(ctx, func(tx *gorm.DB) error {
		if err := tx.Create(dbUser).Error; err != nil {
			return fmt.Errorf("failed to create database user: %w", err)
		}
		for _, dbID := range req.Databases {
			if err := tx.Create(&models.DatabaseDatabaseUser{DatabaseID: dbID, DatabaseUserID: dbUser.ID}).Error; err != nil {
				return fmt.Errorf("failed to attach database %s to user: %w", dbID, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.dispatchCreateDatabaseUser(dbUser, req.Password, userID)
	return dbUser, nil
}

// GetDatabaseUser retrieves a database user by ID and returns the
// response DTO. Signature matches ShowNestedFunc; teamID is currently
// unused but threaded for the framework convention.
func (s *Service) GetDatabaseUser(ctx context.Context, id, serverID, teamID string) (dto.DatabaseUserResponse, error) {
	_ = teamID
	user, err := s.repos.User().FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return dto.DatabaseUserResponse{}, err
	}
	return dto.ToDatabaseUserResponse(user), nil
}

// ListDatabaseUsers lists all database users for a server. Signature
// matches IndexNestedFunc; teamID is currently unused.
func (s *Service) ListDatabaseUsers(ctx context.Context, serverID, teamID string) ([]dto.DatabaseUserResponse, error) {
	_ = teamID
	users, err := s.repos.User().FindByServer(ctx, serverID)
	if err != nil {
		return nil, err
	}
	return dto.ToDatabaseUserResponseList(users), nil
}

// UpdateDatabaseUser updates a database user and returns the response
// DTO. Signature matches UpdateNestedFunc.
func (s *Service) UpdateDatabaseUser(ctx context.Context, id, serverID, teamID, userID string, req *dto.UpdateDatabaseUserRequest) (dto.DatabaseUserResponse, error) {
	_ = teamID
	dbUser, err := s.applyDatabaseUserUpdate(ctx, id, serverID, userID, req)
	if err != nil {
		return dto.DatabaseUserResponse{}, err
	}
	return dto.ToDatabaseUserResponse(dbUser), nil
}

func (s *Service) applyDatabaseUserUpdate(ctx context.Context, id, serverID, userID string, req *dto.UpdateDatabaseUserRequest) (*models.DatabaseUser, error) {
	dbUser, err := s.repos.User().FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return nil, err
	}

	if dbUser.IsUninstalling() {
		return nil, ErrUserBeingUninstalled
	}

	if len(req.Databases) > 0 {
		validDBs, err := s.repos.Database().FindByIDsAndServer(ctx, req.Databases, serverID)
		if err != nil {
			return nil, fmt.Errorf("failed to validate databases: %w", err)
		}
		if len(validDBs) != len(req.Databases) {
			return nil, errors.New("one or more databases not found on this server")
		}
	}

	var passwordForJob *string
	if req.Password != "" {
		password := &dbtype.EncryptedNullableString{}
		password.Set(req.Password)
		dbUser.Password = password
		passwordForJob = &req.Password

		if err := s.repos.User().Update(ctx, dbUser); err != nil {
			return nil, fmt.Errorf("failed to update database user: %w", err)
		}
	}

	if err := s.repos.User().SyncDatabases(ctx, dbUser.ID, req.Databases); err != nil {
		return nil, fmt.Errorf("failed to sync user databases: %w", err)
	}

	s.dispatchUpdateDatabaseUser(dbUser, passwordForJob, userID)

	dbUser, err = s.repos.User().FindByID(ctx, dbUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to reload database user: %w", err)
	}
	return dbUser, nil
}

// DeleteDatabaseUser deletes a database user from a server. Signature
// matches DeleteNestedFunc.
func (s *Service) DeleteDatabaseUser(ctx context.Context, id, serverID, teamID, userID string) error {
	_ = teamID
	dbUser, err := s.repos.User().FindByIDAndServer(ctx, id, serverID)
	if err != nil {
		return err
	}

	if dbUser.IsUninstalling() {
		return ErrUserBeingUninstalled
	}

	if err := s.repos.User().MarkAsUninstalling(ctx, dbUser.ID); err != nil {
		return fmt.Errorf("failed to update user status: %w", err)
	}

	s.dispatchDeleteDatabaseUser(dbUser, userID)
	return nil
}

// BroadcastDatabaseUserStatus broadcasts a database user status update.
func (s *Service) BroadcastDatabaseUserStatus(serverID, userID, status, message string) {
	s.BroadcastToServer(serverID, "database_user.status", map[string]interface{}{
		"server_id": serverID,
		"user_id":   userID,
		"status":    status,
		"message":   message,
	})
}

func (s *Service) dispatchCreateDatabaseUser(user *models.DatabaseUser, password, userID string) {
	uid := userIDPtr(userID)
	s.DispatchTask("InstallDatabaseUser", func() (*asynq.Task, error) {
		return jobs.NewInstallDatabaseUserTask(user.ID, password, uid)
	}, "user_id", user.ID)
}

func (s *Service) dispatchUpdateDatabaseUser(user *models.DatabaseUser, password *string, userID string) {
	uid := userIDPtr(userID)
	s.DispatchTask("UpdateDatabaseUser", func() (*asynq.Task, error) {
		return jobs.NewUpdateDatabaseUserTask(user.ID, password, uid)
	}, "user_id", user.ID)
}

func (s *Service) dispatchDeleteDatabaseUser(user *models.DatabaseUser, userID string) {
	uid := userIDPtr(userID)
	s.DispatchTask("UninstallDatabaseUser", func() (*asynq.Task, error) {
		return jobs.NewUninstallDatabaseUserTask(user.ID, uid)
	}, "user_id", user.ID)
}
