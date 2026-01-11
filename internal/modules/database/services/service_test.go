package services

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/database/dto"
	"github.com/kkz6/launch-go/internal/modules/database/models"
	"github.com/kkz6/launch-go/internal/modules/database/repositories"
	"github.com/kkz6/launch-go/internal/websocket"
)

// MockServerRepository implements ServerRepository for testing
type MockServerRepository struct {
	servers map[string]interface{}
}

func NewMockServerRepository() *MockServerRepository {
	return &MockServerRepository{
		servers: make(map[string]interface{}),
	}
}

func (m *MockServerRepository) AddServer(id string, server interface{}) {
	m.servers[id] = server
}

func (m *MockServerRepository) FindByID(ctx context.Context, id string) (interface{}, error) {
	if server, ok := m.servers[id]; ok {
		return server, nil
	}

	return nil, ErrServerNotFound
}

func (m *MockServerRepository) FindByIDAndTeam(ctx context.Context, id, teamID string) (interface{}, error) {
	if server, ok := m.servers[id]; ok {
		return server, nil
	}

	return nil, ErrServerNotFound
}

// MockQueueClient implements a mock queue client for testing
type MockQueueClient struct {
	EnqueuedTasks []interface{}
}

func (m *MockQueueClient) Enqueue(task interface{}) error {
	m.EnqueuedTasks = append(m.EnqueuedTasks, task)

	return nil
}

func setupTestService(t *testing.T) (*Service, *repositories.Repository, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.Database{}, &models.DatabaseUser{}, &models.DatabaseDatabaseUser{})
	require.NoError(t, err)

	repo := repositories.NewRepository(db)
	serverRepo := NewMockServerRepository()
	serverRepo.AddServer("01ARZ3NDEKTSV4RRFFQ69G5FAV", struct{}{})

	logger := zerolog.New(os.Stdout)
	ws := websocket.NewHub()

	// Service without queue client for testing
	service := NewService(repo, serverRepo, nil, ws, &logger)

	return service, repo, db
}

func TestNewService(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	repo := repositories.NewRepository(db)
	serverRepo := NewMockServerRepository()
	logger := zerolog.New(os.Stdout)
	ws := websocket.NewHub()

	service := NewService(repo, serverRepo, nil, ws, &logger)

	assert.NotNil(t, service)
}

func TestService_CreateDatabase(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("creates database successfully", func(t *testing.T) {
		req := &dto.CreateDatabaseRequest{
			Name:       "test_db",
			CreateUser: false,
		}

		database, err := service.CreateDatabase(ctx, serverID, req, nil)
		require.NoError(t, err)

		assert.NotEmpty(t, database.ID)
		assert.Equal(t, "test_db", database.Name)
		assert.Equal(t, serverID, database.ServerID)
	})

	t.Run("returns error for duplicate name", func(t *testing.T) {
		// Create first database
		err := repo.Create(ctx, &models.Database{
			ServerID: serverID,
			Name:     "duplicate_db",
		})
		require.NoError(t, err)

		req := &dto.CreateDatabaseRequest{
			Name:       "duplicate_db",
			CreateUser: false,
		}

		_, err = service.CreateDatabase(ctx, serverID, req, nil)
		assert.ErrorIs(t, err, ErrDatabaseNameExists)
	})

	t.Run("creates database with new user", func(t *testing.T) {
		req := &dto.CreateDatabaseRequest{
			Name:         "db_with_user",
			CreateUser:   true,
			UserName:     "new_user",
			UserPassword: "password123",
		}

		database, err := service.CreateDatabase(ctx, serverID, req, nil)
		require.NoError(t, err)

		assert.NotEmpty(t, database.ID)

		// Verify user was created
		user, err := repo.FindUserByNameAndServer(ctx, "new_user", serverID)
		require.NoError(t, err)
		assert.Equal(t, "new_user", user.Name)
	})

	t.Run("creates database with existing user", func(t *testing.T) {
		// Create existing user
		existingUser := &models.DatabaseUser{
			ServerID: serverID,
			Name:     "existing_user",
			Password: "password",
		}
		err := repo.CreateUser(ctx, existingUser)
		require.NoError(t, err)

		req := &dto.CreateDatabaseRequest{
			Name:           "db_with_existing_user",
			CreateUser:     false,
			ExistingUserID: &existingUser.ID,
		}

		database, err := service.CreateDatabase(ctx, serverID, req, nil)
		require.NoError(t, err)

		// Verify user is attached
		found, err := repo.FindByID(ctx, database.ID)
		require.NoError(t, err)
		assert.Len(t, found.Users, 1)
	})

	t.Run("returns error for duplicate user name", func(t *testing.T) {
		// Create existing user
		err := repo.CreateUser(ctx, &models.DatabaseUser{
			ServerID: serverID,
			Name:     "dup_user",
			Password: "password",
		})
		require.NoError(t, err)

		req := &dto.CreateDatabaseRequest{
			Name:         "db_dup_user",
			CreateUser:   true,
			UserName:     "dup_user",
			UserPassword: "password123",
		}

		_, err = service.CreateDatabase(ctx, serverID, req, nil)
		assert.ErrorIs(t, err, ErrDatabaseUserNameExists)
	})
}

func TestService_GetDatabase(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("returns database when found", func(t *testing.T) {
		created := &models.Database{
			ServerID: serverID,
			Name:     "get_test_db",
		}
		err := repo.Create(ctx, created)
		require.NoError(t, err)

		found, err := service.GetDatabase(ctx, created.ID, serverID)
		require.NoError(t, err)

		assert.Equal(t, created.ID, found.ID)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		_, err := service.GetDatabase(ctx, "non_existent", serverID)

		assert.ErrorIs(t, err, ErrDatabaseNotFound)
	})
}

func TestService_ListDatabases(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	// Create some databases
	err := repo.Create(ctx, &models.Database{ServerID: serverID, Name: "db1"})
	require.NoError(t, err)
	err = repo.Create(ctx, &models.Database{ServerID: serverID, Name: "db2"})
	require.NoError(t, err)

	databases, err := service.ListDatabases(ctx, serverID)
	require.NoError(t, err)

	assert.Len(t, databases, 2)
}

func TestService_DeleteDatabase(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("marks database as uninstalling", func(t *testing.T) {
		database := &models.Database{
			ServerID: serverID,
			Name:     "to_delete",
		}
		err := repo.Create(ctx, database)
		require.NoError(t, err)

		err = service.DeleteDatabase(ctx, database.ID, serverID, nil)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, database.ID)
		require.NoError(t, err)

		assert.NotNil(t, found.UninstallationRequestedAt)
	})

	t.Run("returns error when already uninstalling", func(t *testing.T) {
		now := time.Now()
		database := &models.Database{
			ServerID:                  serverID,
			Name:                      "already_uninstalling",
			UninstallationRequestedAt: &now,
		}
		err := repo.Create(ctx, database)
		require.NoError(t, err)

		err = service.DeleteDatabase(ctx, database.ID, serverID, nil)
		assert.ErrorIs(t, err, ErrDatabaseBeingUninstalled)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		err := service.DeleteDatabase(ctx, "non_existent", serverID, nil)
		assert.ErrorIs(t, err, ErrDatabaseNotFound)
	})
}

func TestService_CreateDatabaseUser(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("creates user successfully", func(t *testing.T) {
		req := &dto.CreateDatabaseUserRequest{
			Name:     "new_user",
			Password: "password123",
		}

		user, err := service.CreateDatabaseUser(ctx, serverID, req, nil)
		require.NoError(t, err)

		assert.NotEmpty(t, user.ID)
		assert.Equal(t, "new_user", user.Name)
	})

	t.Run("creates user with databases", func(t *testing.T) {
		// Create database first
		database := &models.Database{
			ServerID: serverID,
			Name:     "db_for_user",
		}
		err := repo.Create(ctx, database)
		require.NoError(t, err)

		req := &dto.CreateDatabaseUserRequest{
			Name:      "user_with_db",
			Password:  "password123",
			Databases: []string{database.ID},
		}

		user, err := service.CreateDatabaseUser(ctx, serverID, req, nil)
		require.NoError(t, err)

		// Verify database is attached
		found, err := repo.FindUserByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Len(t, found.Databases, 1)
	})

	t.Run("returns error for duplicate name", func(t *testing.T) {
		err := repo.CreateUser(ctx, &models.DatabaseUser{
			ServerID: serverID,
			Name:     "duplicate_user",
			Password: "password",
		})
		require.NoError(t, err)

		req := &dto.CreateDatabaseUserRequest{
			Name:     "duplicate_user",
			Password: "password123",
		}

		_, err = service.CreateDatabaseUser(ctx, serverID, req, nil)
		assert.ErrorIs(t, err, ErrDatabaseUserNameExists)
	})

	t.Run("returns error for invalid database", func(t *testing.T) {
		req := &dto.CreateDatabaseUserRequest{
			Name:      "user_invalid_db",
			Password:  "password123",
			Databases: []string{"non_existent_db"},
		}

		_, err := service.CreateDatabaseUser(ctx, serverID, req, nil)
		assert.Error(t, err)
	})
}

func TestService_GetDatabaseUser(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("returns user when found", func(t *testing.T) {
		created := &models.DatabaseUser{
			ServerID: serverID,
			Name:     "get_test_user",
			Password: "password",
		}
		err := repo.CreateUser(ctx, created)
		require.NoError(t, err)

		found, err := service.GetDatabaseUser(ctx, created.ID, serverID)
		require.NoError(t, err)

		assert.Equal(t, created.ID, found.ID)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		_, err := service.GetDatabaseUser(ctx, "non_existent", serverID)
		assert.ErrorIs(t, err, ErrDatabaseUserNotFound)
	})
}

func TestService_ListDatabaseUsers(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	// Create some users
	err := repo.CreateUser(ctx, &models.DatabaseUser{ServerID: serverID, Name: "user1", Password: "pass"})
	require.NoError(t, err)
	err = repo.CreateUser(ctx, &models.DatabaseUser{ServerID: serverID, Name: "user2", Password: "pass"})
	require.NoError(t, err)

	users, err := service.ListDatabaseUsers(ctx, serverID)
	require.NoError(t, err)

	assert.Len(t, users, 2)
}

func TestService_UpdateDatabaseUser(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("updates user successfully", func(t *testing.T) {
		user := &models.DatabaseUser{
			ServerID: serverID,
			Name:     "update_test_user",
			Password: "old_password",
		}
		err := repo.CreateUser(ctx, user)
		require.NoError(t, err)

		req := &dto.UpdateDatabaseUserRequest{
			Password: "new_password",
		}

		updated, err := service.UpdateDatabaseUser(ctx, user.ID, serverID, req, nil)
		require.NoError(t, err)

		assert.Equal(t, "new_password", updated.Password)
	})

	t.Run("updates user databases", func(t *testing.T) {
		// Create databases
		db1 := &models.Database{ServerID: serverID, Name: "upd_db1"}
		err := repo.Create(ctx, db1)
		require.NoError(t, err)

		db2 := &models.Database{ServerID: serverID, Name: "upd_db2"}
		err = repo.Create(ctx, db2)
		require.NoError(t, err)

		// Create user with db1
		user := &models.DatabaseUser{
			ServerID: serverID,
			Name:     "user_upd_dbs",
			Password: "password",
		}
		err = repo.CreateUser(ctx, user)
		require.NoError(t, err)
		err = repo.AttachUser(ctx, db1.ID, user.ID)
		require.NoError(t, err)

		// Update to db2
		req := &dto.UpdateDatabaseUserRequest{
			Password:  "password",
			Databases: []string{db2.ID},
		}

		updated, err := service.UpdateDatabaseUser(ctx, user.ID, serverID, req, nil)
		require.NoError(t, err)

		assert.Len(t, updated.Databases, 1)
		assert.Equal(t, db2.ID, updated.Databases[0].ID)
	})

	t.Run("returns error when user being uninstalled", func(t *testing.T) {
		now := time.Now()
		user := &models.DatabaseUser{
			ServerID:                  serverID,
			Name:                      "uninstalling_user",
			Password:                  "password",
			UninstallationRequestedAt: &now,
		}
		err := repo.CreateUser(ctx, user)
		require.NoError(t, err)

		req := &dto.UpdateDatabaseUserRequest{
			Password: "new_password",
		}

		_, err = service.UpdateDatabaseUser(ctx, user.ID, serverID, req, nil)
		assert.ErrorIs(t, err, ErrUserBeingUninstalled)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		req := &dto.UpdateDatabaseUserRequest{
			Password: "password",
		}

		_, err := service.UpdateDatabaseUser(ctx, "non_existent", serverID, req, nil)
		assert.ErrorIs(t, err, ErrDatabaseUserNotFound)
	})

	t.Run("returns error for invalid database", func(t *testing.T) {
		user := &models.DatabaseUser{
			ServerID: serverID,
			Name:     "user_inv_db",
			Password: "password",
		}
		err := repo.CreateUser(ctx, user)
		require.NoError(t, err)

		req := &dto.UpdateDatabaseUserRequest{
			Password:  "password",
			Databases: []string{"non_existent_db"},
		}

		_, err = service.UpdateDatabaseUser(ctx, user.ID, serverID, req, nil)
		assert.Error(t, err)
	})
}

func TestService_DeleteDatabaseUser(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("marks user as uninstalling", func(t *testing.T) {
		user := &models.DatabaseUser{
			ServerID: serverID,
			Name:     "to_delete_user",
			Password: "password",
		}
		err := repo.CreateUser(ctx, user)
		require.NoError(t, err)

		err = service.DeleteDatabaseUser(ctx, user.ID, serverID, nil)
		require.NoError(t, err)

		found, err := repo.FindUserByID(ctx, user.ID)
		require.NoError(t, err)

		assert.NotNil(t, found.UninstallationRequestedAt)
	})

	t.Run("returns error when already uninstalling", func(t *testing.T) {
		now := time.Now()
		user := &models.DatabaseUser{
			ServerID:                  serverID,
			Name:                      "already_del_user",
			Password:                  "password",
			UninstallationRequestedAt: &now,
		}
		err := repo.CreateUser(ctx, user)
		require.NoError(t, err)

		err = service.DeleteDatabaseUser(ctx, user.ID, serverID, nil)
		assert.ErrorIs(t, err, ErrUserBeingUninstalled)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		err := service.DeleteDatabaseUser(ctx, "non_existent", serverID, nil)
		assert.ErrorIs(t, err, ErrDatabaseUserNotFound)
	})
}

func TestService_AttachRootUser(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("attaches root user when exists", func(t *testing.T) {
		// Create root user
		rootUser := &models.DatabaseUser{
			ServerID: serverID,
			Name:     "root",
			Password: "rootpass",
		}
		err := repo.CreateUser(ctx, rootUser)
		require.NoError(t, err)

		// Create database
		database := &models.Database{
			ServerID: serverID,
			Name:     "db_with_root",
		}
		err = repo.Create(ctx, database)
		require.NoError(t, err)

		// Attach root user
		service.attachRootUser(ctx, serverID, database.ID)

		// Verify
		found, err := repo.FindByID(ctx, database.ID)
		require.NoError(t, err)
		assert.Len(t, found.Users, 1)
		assert.Equal(t, "root", found.Users[0].Name)
	})

	t.Run("does nothing when root user not exists", func(t *testing.T) {
		otherServerID := "01ARZ3NDEKTSV4RRFFQ69G5FAW"

		database := &models.Database{
			ServerID: otherServerID,
			Name:     "db_no_root",
		}
		err := repo.Create(ctx, database)
		require.NoError(t, err)

		// Should not panic
		service.attachRootUser(ctx, otherServerID, database.ID)

		found, err := repo.FindByID(ctx, database.ID)
		require.NoError(t, err)
		assert.Len(t, found.Users, 0)
	})
}

func TestService_BroadcastDatabaseStatus(t *testing.T) {
	service, _, _ := setupTestService(t)

	// Should not panic
	service.BroadcastDatabaseStatus("server_id", "database_id", "installed", "Test message")
}

func TestService_BroadcastDatabaseUserStatus(t *testing.T) {
	service, _, _ := setupTestService(t)

	// Should not panic
	service.BroadcastDatabaseUserStatus("server_id", "user_id", "installed", "Test message")
}

func TestService_SyncDatabases(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("syncs databases successfully", func(t *testing.T) {
		err := service.SyncDatabases(ctx, serverID, nil)
		require.NoError(t, err)
	})
}

func TestService_CreateDatabase_WithInvalidExistingUser(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	invalidUserID := "invalid_user_id"
	req := &dto.CreateDatabaseRequest{
		Name:           "db_invalid_user",
		CreateUser:     false,
		ExistingUserID: &invalidUserID,
	}

	// Should still create database even if existing user is invalid
	database, err := service.CreateDatabase(ctx, serverID, req, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, database.ID)
}

func TestService_CreateDatabase_WithValidExistingUser(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	// Create an existing user
	existingUser := &models.DatabaseUser{
		ServerID: serverID,
		Name:     "existing_user_valid",
		Password: "password",
	}
	err := repo.CreateUser(ctx, existingUser)
	require.NoError(t, err)

	req := &dto.CreateDatabaseRequest{
		Name:           "db_with_valid_existing_user",
		CreateUser:     false,
		ExistingUserID: &existingUser.ID,
	}

	database, err := service.CreateDatabase(ctx, serverID, req, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, database.ID)

	// Verify user is attached
	found, err := repo.FindByID(ctx, database.ID)
	require.NoError(t, err)
	assert.Len(t, found.Users, 1)
	assert.Equal(t, existingUser.ID, found.Users[0].ID)
}

func TestService_CreateDatabase_WithEmptyExistingUserID(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	emptyUserID := ""
	req := &dto.CreateDatabaseRequest{
		Name:           "db_empty_user_id",
		CreateUser:     false,
		ExistingUserID: &emptyUserID,
	}

	database, err := service.CreateDatabase(ctx, serverID, req, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, database.ID)
}

func TestService_UpdateDatabaseUser_EmptyDatabases(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	// Create database and user
	database := &models.Database{ServerID: serverID, Name: "db_upd_empty"}
	err := repo.Create(ctx, database)
	require.NoError(t, err)

	user := &models.DatabaseUser{
		ServerID: serverID,
		Name:     "user_upd_empty_dbs",
		Password: "password",
	}
	err = repo.CreateUser(ctx, user)
	require.NoError(t, err)

	// Attach database to user
	err = repo.AttachUser(ctx, database.ID, user.ID)
	require.NoError(t, err)

	// Update with empty databases list
	req := &dto.UpdateDatabaseUserRequest{
		Password:  "new_password",
		Databases: []string{},
	}

	updated, err := service.UpdateDatabaseUser(ctx, user.ID, serverID, req, nil)
	require.NoError(t, err)

	// Verify databases were removed
	assert.Len(t, updated.Databases, 0)
}

func TestService_DispatchFunctions_WithNilQueue(t *testing.T) {
	service, repo, _ := setupTestService(t)
	ctx := context.Background()
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	database := &models.Database{ServerID: serverID, Name: "dispatch_test_db"}
	err := repo.Create(ctx, database)
	require.NoError(t, err)

	user := &models.DatabaseUser{
		ServerID: serverID,
		Name:     "dispatch_test_user",
		Password: "password",
	}
	err = repo.CreateUser(ctx, user)
	require.NoError(t, err)

	// These should not panic with nil queue
	service.dispatchCreateDatabase(ctx, database, nil)
	service.dispatchCreateDatabaseWithExistingUser(ctx, database, user, nil)
	service.dispatchCreateDatabaseWithNewUser(ctx, database, user, "password", nil)
	service.dispatchDeleteDatabase(ctx, database, nil)
	service.dispatchCreateDatabaseUser(ctx, user, "password", nil)
	service.dispatchUpdateDatabaseUser(ctx, user, nil, nil)
	service.dispatchDeleteDatabaseUser(ctx, user, nil)
}
