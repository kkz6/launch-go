package database

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestRepository(t *testing.T) (*Repository, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&Database{}, &DatabaseUser{}, &DatabaseDatabaseUser{})
	require.NoError(t, err)

	return NewRepository(db), db
}

func createTestDatabase(t *testing.T, repo *Repository, serverID, name string) *Database {
	database := &Database{
		ServerID: serverID,
		Name:     name,
	}
	err := repo.Create(context.Background(), database)
	require.NoError(t, err)

	return database
}

func createTestUser(t *testing.T, repo *Repository, serverID, name string) *DatabaseUser {
	user := &DatabaseUser{
		ServerID: serverID,
		Name:     name,
		Password: "testpassword",
	}
	err := repo.CreateUser(context.Background(), user)
	require.NoError(t, err)

	return user
}

func TestNewRepository(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	repo := NewRepository(db)

	assert.NotNil(t, repo)
	assert.Equal(t, db, repo.db)
}

func TestRepository_Create(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	database := &Database{
		ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		Name:     "test_db",
	}

	err := repo.Create(ctx, database)
	require.NoError(t, err)

	assert.NotEmpty(t, database.ID)
	assert.NotZero(t, database.CreatedAt)
	assert.NotZero(t, database.UpdatedAt)
}

func TestRepository_FindByID(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	t.Run("finds existing database", func(t *testing.T) {
		created := createTestDatabase(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "test_db")

		found, err := repo.FindByID(ctx, created.ID)
		require.NoError(t, err)

		assert.Equal(t, created.ID, found.ID)
		assert.Equal(t, created.Name, found.Name)
	})

	t.Run("returns error for non-existent database", func(t *testing.T) {
		_, err := repo.FindByID(ctx, "non_existent_id")

		assert.ErrorIs(t, err, ErrDatabaseNotFound)
	})
}

func TestRepository_FindByIDAndServer(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	t.Run("finds database by ID and server", func(t *testing.T) {
		serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
		created := createTestDatabase(t, repo, serverID, "test_db")

		found, err := repo.FindByIDAndServer(ctx, created.ID, serverID)
		require.NoError(t, err)

		assert.Equal(t, created.ID, found.ID)
	})

	t.Run("returns error for wrong server", func(t *testing.T) {
		created := createTestDatabase(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "test_db_2")

		_, err := repo.FindByIDAndServer(ctx, created.ID, "different_server")

		assert.ErrorIs(t, err, ErrDatabaseNotFound)
	})
}

func TestRepository_FindByServer(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	createTestDatabase(t, repo, serverID, "db1")
	createTestDatabase(t, repo, serverID, "db2")
	createTestDatabase(t, repo, "other_server", "db3")

	databases, err := repo.FindByServer(ctx, serverID)
	require.NoError(t, err)

	assert.Len(t, databases, 2)
}

func TestRepository_FindByNameAndServer(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	t.Run("finds database by name and server", func(t *testing.T) {
		serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
		created := createTestDatabase(t, repo, serverID, "unique_name")

		found, err := repo.FindByNameAndServer(ctx, "unique_name", serverID)
		require.NoError(t, err)

		assert.Equal(t, created.ID, found.ID)
	})

	t.Run("returns error for non-existent name", func(t *testing.T) {
		_, err := repo.FindByNameAndServer(ctx, "non_existent", "server_id")

		assert.ErrorIs(t, err, ErrDatabaseNotFound)
	})
}

func TestRepository_FindByUser(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	db1 := createTestDatabase(t, repo, serverID, "db1")
	db2 := createTestDatabase(t, repo, serverID, "db2")
	createTestDatabase(t, repo, serverID, "db3") // Not attached

	user := createTestUser(t, repo, serverID, "test_user")

	// Attach user to db1 and db2
	err := repo.AttachUser(ctx, db1.ID, user.ID)
	require.NoError(t, err)
	err = repo.AttachUser(ctx, db2.ID, user.ID)
	require.NoError(t, err)

	databases, err := repo.FindByUser(ctx, user.ID)
	require.NoError(t, err)

	assert.Len(t, databases, 2)
}

func TestRepository_Update(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	database := createTestDatabase(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "original_name")

	now := time.Now()
	database.InstalledAt = &now

	err := repo.Update(ctx, database)
	require.NoError(t, err)

	found, err := repo.FindByID(ctx, database.ID)
	require.NoError(t, err)

	assert.NotNil(t, found.InstalledAt)
}

func TestRepository_UpdateFields(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	database := createTestDatabase(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "test_db")

	now := time.Now()
	err := repo.UpdateFields(ctx, database.ID, map[string]interface{}{
		"installed_at": now,
	})
	require.NoError(t, err)

	found, err := repo.FindByID(ctx, database.ID)
	require.NoError(t, err)

	assert.NotNil(t, found.InstalledAt)
}

func TestRepository_Delete(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	database := createTestDatabase(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "to_delete")

	err := repo.Delete(ctx, database.ID)
	require.NoError(t, err)

	_, err = repo.FindByID(ctx, database.ID)
	assert.ErrorIs(t, err, ErrDatabaseNotFound)
}

func TestRepository_ExistsByNameAndServer(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	createTestDatabase(t, repo, serverID, "existing_db")

	t.Run("returns true for existing database", func(t *testing.T) {
		exists, err := repo.ExistsByNameAndServer(ctx, "existing_db", serverID)
		require.NoError(t, err)

		assert.True(t, exists)
	})

	t.Run("returns false for non-existent database", func(t *testing.T) {
		exists, err := repo.ExistsByNameAndServer(ctx, "non_existent", serverID)
		require.NoError(t, err)

		assert.False(t, exists)
	})

	t.Run("returns false for different server", func(t *testing.T) {
		exists, err := repo.ExistsByNameAndServer(ctx, "existing_db", "different_server")
		require.NoError(t, err)

		assert.False(t, exists)
	})
}

func TestRepository_AttachUser(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	database := createTestDatabase(t, repo, serverID, "test_db")
	user := createTestUser(t, repo, serverID, "test_user")

	err := repo.AttachUser(ctx, database.ID, user.ID)
	require.NoError(t, err)

	// Verify attachment
	found, err := repo.FindByID(ctx, database.ID)
	require.NoError(t, err)

	assert.Len(t, found.Users, 1)
	assert.Equal(t, user.ID, found.Users[0].ID)
}

func TestRepository_DetachUser(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	database := createTestDatabase(t, repo, serverID, "test_db")
	user := createTestUser(t, repo, serverID, "test_user")

	// Attach first
	err := repo.AttachUser(ctx, database.ID, user.ID)
	require.NoError(t, err)

	// Then detach
	err = repo.DetachUser(ctx, database.ID, user.ID)
	require.NoError(t, err)

	// Verify detachment
	found, err := repo.FindByID(ctx, database.ID)
	require.NoError(t, err)

	assert.Len(t, found.Users, 0)
}

func TestRepository_DetachAllUsers(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	database := createTestDatabase(t, repo, serverID, "test_db")
	user1 := createTestUser(t, repo, serverID, "user1")
	user2 := createTestUser(t, repo, serverID, "user2")

	// Attach users
	err := repo.AttachUser(ctx, database.ID, user1.ID)
	require.NoError(t, err)
	err = repo.AttachUser(ctx, database.ID, user2.ID)
	require.NoError(t, err)

	// Detach all
	err = repo.DetachAllUsers(ctx, database.ID)
	require.NoError(t, err)

	// Verify
	found, err := repo.FindByID(ctx, database.ID)
	require.NoError(t, err)

	assert.Len(t, found.Users, 0)
}

// Database User repository tests

func TestRepository_CreateUser(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := &DatabaseUser{
		ServerID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		Name:     "test_user",
		Password: "secret",
	}

	err := repo.CreateUser(ctx, user)
	require.NoError(t, err)

	assert.NotEmpty(t, user.ID)
	assert.Equal(t, "localhost", user.Host)
}

func TestRepository_FindUserByID(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	t.Run("finds existing user", func(t *testing.T) {
		created := createTestUser(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "test_user")

		found, err := repo.FindUserByID(ctx, created.ID)
		require.NoError(t, err)

		assert.Equal(t, created.ID, found.ID)
		assert.Equal(t, created.Name, found.Name)
	})

	t.Run("returns error for non-existent user", func(t *testing.T) {
		_, err := repo.FindUserByID(ctx, "non_existent_id")

		assert.ErrorIs(t, err, ErrDatabaseUserNotFound)
	})
}

func TestRepository_FindUserByIDAndServer(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	t.Run("finds user by ID and server", func(t *testing.T) {
		serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
		created := createTestUser(t, repo, serverID, "test_user")

		found, err := repo.FindUserByIDAndServer(ctx, created.ID, serverID)
		require.NoError(t, err)

		assert.Equal(t, created.ID, found.ID)
	})

	t.Run("returns error for wrong server", func(t *testing.T) {
		created := createTestUser(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "test_user_2")

		_, err := repo.FindUserByIDAndServer(ctx, created.ID, "different_server")

		assert.ErrorIs(t, err, ErrDatabaseUserNotFound)
	})
}

func TestRepository_FindUsersByServer(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	createTestUser(t, repo, serverID, "user1")
	createTestUser(t, repo, serverID, "user2")
	createTestUser(t, repo, "other_server", "user3")

	users, err := repo.FindUsersByServer(ctx, serverID)
	require.NoError(t, err)

	assert.Len(t, users, 2)
}

func TestRepository_FindUserByNameAndServer(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	t.Run("finds user by name and server", func(t *testing.T) {
		serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
		created := createTestUser(t, repo, serverID, "unique_user")

		found, err := repo.FindUserByNameAndServer(ctx, "unique_user", serverID)
		require.NoError(t, err)

		assert.Equal(t, created.ID, found.ID)
	})

	t.Run("returns error for non-existent name", func(t *testing.T) {
		_, err := repo.FindUserByNameAndServer(ctx, "non_existent", "server_id")

		assert.ErrorIs(t, err, ErrDatabaseUserNotFound)
	})
}

func TestRepository_FindUsersByDatabase(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	database := createTestDatabase(t, repo, serverID, "test_db")
	user1 := createTestUser(t, repo, serverID, "user1")
	user2 := createTestUser(t, repo, serverID, "user2")
	createTestUser(t, repo, serverID, "user3") // Not attached

	// Attach users to database
	err := repo.AttachUser(ctx, database.ID, user1.ID)
	require.NoError(t, err)
	err = repo.AttachUser(ctx, database.ID, user2.ID)
	require.NoError(t, err)

	users, err := repo.FindUsersByDatabase(ctx, database.ID)
	require.NoError(t, err)

	assert.Len(t, users, 2)
}

func TestRepository_UpdateUser(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "test_user")

	user.Password = "new_password"

	err := repo.UpdateUser(ctx, user)
	require.NoError(t, err)

	found, err := repo.FindUserByID(ctx, user.ID)
	require.NoError(t, err)

	assert.Equal(t, "new_password", found.Password)
}

func TestRepository_UpdateUserFields(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "test_user")

	now := time.Now()
	err := repo.UpdateUserFields(ctx, user.ID, map[string]interface{}{
		"installed_at": now,
	})
	require.NoError(t, err)

	found, err := repo.FindUserByID(ctx, user.ID)
	require.NoError(t, err)

	assert.NotNil(t, found.InstalledAt)
}

func TestRepository_DeleteUser(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	user := createTestUser(t, repo, "01ARZ3NDEKTSV4RRFFQ69G5FAV", "to_delete")

	err := repo.DeleteUser(ctx, user.ID)
	require.NoError(t, err)

	_, err = repo.FindUserByID(ctx, user.ID)
	assert.ErrorIs(t, err, ErrDatabaseUserNotFound)
}

func TestRepository_UserExistsByNameAndServer(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	createTestUser(t, repo, serverID, "existing_user")

	t.Run("returns true for existing user", func(t *testing.T) {
		exists, err := repo.UserExistsByNameAndServer(ctx, "existing_user", serverID)
		require.NoError(t, err)

		assert.True(t, exists)
	})

	t.Run("returns false for non-existent user", func(t *testing.T) {
		exists, err := repo.UserExistsByNameAndServer(ctx, "non_existent", serverID)
		require.NoError(t, err)

		assert.False(t, exists)
	})

	t.Run("returns false for different server", func(t *testing.T) {
		exists, err := repo.UserExistsByNameAndServer(ctx, "existing_user", "different_server")
		require.NoError(t, err)

		assert.False(t, exists)
	})
}

func TestRepository_SyncUserDatabases(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	db1 := createTestDatabase(t, repo, serverID, "db1")
	db2 := createTestDatabase(t, repo, serverID, "db2")
	db3 := createTestDatabase(t, repo, serverID, "db3")
	user := createTestUser(t, repo, serverID, "test_user")

	// Initially attach db1 and db2
	err := repo.AttachUser(ctx, db1.ID, user.ID)
	require.NoError(t, err)
	err = repo.AttachUser(ctx, db2.ID, user.ID)
	require.NoError(t, err)

	// Sync to db2 and db3 (remove db1, add db3)
	err = repo.SyncUserDatabases(ctx, user.ID, []string{db2.ID, db3.ID})
	require.NoError(t, err)

	// Verify
	found, err := repo.FindUserByID(ctx, user.ID)
	require.NoError(t, err)

	assert.Len(t, found.Databases, 2)

	databaseIDs := make([]string, len(found.Databases))
	for i, db := range found.Databases {
		databaseIDs[i] = db.ID
	}

	assert.Contains(t, databaseIDs, db2.ID)
	assert.Contains(t, databaseIDs, db3.ID)
	assert.NotContains(t, databaseIDs, db1.ID)
}

func TestRepository_FindRootUser(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("finds root user when exists", func(t *testing.T) {
		rootUser := &DatabaseUser{
			ServerID: serverID,
			Name:     "root",
			Password: "rootpassword",
		}
		err := repo.CreateUser(ctx, rootUser)
		require.NoError(t, err)

		found, err := repo.FindRootUser(ctx, serverID)
		require.NoError(t, err)

		assert.Equal(t, "root", found.Name)
	})

	t.Run("returns error when root user not found", func(t *testing.T) {
		_, err := repo.FindRootUser(ctx, "server_without_root")

		assert.ErrorIs(t, err, ErrDatabaseUserNotFound)
	})
}

func TestRepository_SyncUserDatabases_EmptyList(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	db1 := createTestDatabase(t, repo, serverID, "sync_db1")
	user := createTestUser(t, repo, serverID, "sync_user_empty")

	// Attach db1
	err := repo.AttachUser(ctx, db1.ID, user.ID)
	require.NoError(t, err)

	// Sync to empty list (remove all)
	err = repo.SyncUserDatabases(ctx, user.ID, []string{})
	require.NoError(t, err)

	// Verify
	found, err := repo.FindUserByID(ctx, user.ID)
	require.NoError(t, err)

	assert.Len(t, found.Databases, 0)
}

func TestRepository_FindByUser_EmptyResult(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	user := createTestUser(t, repo, serverID, "user_no_dbs")

	databases, err := repo.FindByUser(ctx, user.ID)
	require.NoError(t, err)

	assert.Len(t, databases, 0)
}

func TestRepository_FindUsersByDatabase_EmptyResult(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	database := createTestDatabase(t, repo, serverID, "db_no_users")

	users, err := repo.FindUsersByDatabase(ctx, database.ID)
	require.NoError(t, err)

	assert.Len(t, users, 0)
}

func TestRepository_FindByServer_EmptyResult(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	databases, err := repo.FindByServer(ctx, "empty_server")
	require.NoError(t, err)

	assert.Len(t, databases, 0)
}

func TestRepository_FindUsersByServer_EmptyResult(t *testing.T) {
	repo, _ := setupTestRepository(t)
	ctx := context.Background()

	users, err := repo.FindUsersByServer(ctx, "empty_server")
	require.NoError(t, err)

	assert.Len(t, users, 0)
}
