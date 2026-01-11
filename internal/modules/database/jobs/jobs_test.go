package jobs

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/websocket"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	return db
}

func setupTestRegistry(t *testing.T) *Registry {
	db := setupTestDB(t)
	logger := zerolog.New(os.Stdout)
	ws := websocket.NewHub()

	return NewRegistry(db, ws, &logger)
}

func TestNewRegistry(t *testing.T) {
	registry := setupTestRegistry(t)

	assert.NotNil(t, registry)
	assert.NotNil(t, registry.InstallDatabase)
	assert.NotNil(t, registry.UninstallDatabase)
	assert.NotNil(t, registry.InstallDatabaseUser)
	assert.NotNil(t, registry.UpdateDatabaseUser)
	assert.NotNil(t, registry.UninstallDatabaseUser)
	assert.NotNil(t, registry.SyncDatabases)
}

func TestNewInstallDatabaseTask(t *testing.T) {
	userID := "test_user_id"

	t.Run("creates task with user ID", func(t *testing.T) {
		task, err := NewInstallDatabaseTask("db_id", &userID)
		require.NoError(t, err)

		assert.Equal(t, TypeInstallDatabase, task.Type())

		var payload InstallDatabasePayload
		err = json.Unmarshal(task.Payload(), &payload)
		require.NoError(t, err)

		assert.Equal(t, "db_id", payload.DatabaseID)
		assert.Equal(t, &userID, payload.UserID)
	})

	t.Run("creates task without user ID", func(t *testing.T) {
		task, err := NewInstallDatabaseTask("db_id", nil)
		require.NoError(t, err)

		var payload InstallDatabasePayload
		err = json.Unmarshal(task.Payload(), &payload)
		require.NoError(t, err)

		assert.Equal(t, "db_id", payload.DatabaseID)
		assert.Nil(t, payload.UserID)
	})
}

func TestNewUninstallDatabaseTask(t *testing.T) {
	userID := "test_user_id"

	task, err := NewUninstallDatabaseTask("db_id", &userID)
	require.NoError(t, err)

	assert.Equal(t, TypeUninstallDatabase, task.Type())

	var payload UninstallDatabasePayload
	err = json.Unmarshal(task.Payload(), &payload)
	require.NoError(t, err)

	assert.Equal(t, "db_id", payload.DatabaseID)
	assert.Equal(t, &userID, payload.UserID)
}

func TestNewInstallDatabaseUserTask(t *testing.T) {
	callerID := "caller_id"

	task, err := NewInstallDatabaseUserTask("user_id", "password123", &callerID)
	require.NoError(t, err)

	assert.Equal(t, TypeInstallDatabaseUser, task.Type())

	var payload InstallDatabaseUserPayload
	err = json.Unmarshal(task.Payload(), &payload)
	require.NoError(t, err)

	assert.Equal(t, "user_id", payload.DatabaseUserID)
	assert.Equal(t, "password123", payload.Password)
	assert.Equal(t, &callerID, payload.CallerID)
}

func TestNewUpdateDatabaseUserTask(t *testing.T) {
	callerID := "caller_id"
	password := "new_password"

	t.Run("creates task with password", func(t *testing.T) {
		task, err := NewUpdateDatabaseUserTask("user_id", &password, &callerID)
		require.NoError(t, err)

		assert.Equal(t, TypeUpdateDatabaseUser, task.Type())

		var payload UpdateDatabaseUserPayload
		err = json.Unmarshal(task.Payload(), &payload)
		require.NoError(t, err)

		assert.Equal(t, "user_id", payload.DatabaseUserID)
		assert.Equal(t, &password, payload.Password)
		assert.Equal(t, &callerID, payload.CallerID)
	})

	t.Run("creates task without password", func(t *testing.T) {
		task, err := NewUpdateDatabaseUserTask("user_id", nil, &callerID)
		require.NoError(t, err)

		var payload UpdateDatabaseUserPayload
		err = json.Unmarshal(task.Payload(), &payload)
		require.NoError(t, err)

		assert.Nil(t, payload.Password)
	})
}

func TestNewUninstallDatabaseUserTask(t *testing.T) {
	callerID := "caller_id"

	task, err := NewUninstallDatabaseUserTask("user_id", &callerID)
	require.NoError(t, err)

	assert.Equal(t, TypeUninstallDatabaseUser, task.Type())

	var payload UninstallDatabaseUserPayload
	err = json.Unmarshal(task.Payload(), &payload)
	require.NoError(t, err)

	assert.Equal(t, "user_id", payload.DatabaseUserID)
	assert.Equal(t, &callerID, payload.CallerID)
}

func TestNewSyncDatabasesTask(t *testing.T) {
	userID := "test_user_id"

	task, err := NewSyncDatabasesTask("server_id", &userID)
	require.NoError(t, err)

	assert.Equal(t, TypeSyncDatabases, task.Type())

	var payload SyncDatabasesPayload
	err = json.Unmarshal(task.Payload(), &payload)
	require.NoError(t, err)

	assert.Equal(t, "server_id", payload.ServerID)
	assert.Equal(t, &userID, payload.UserID)
}

func TestInstallDatabaseJob_Handle(t *testing.T) {
	registry := setupTestRegistry(t)
	ctx := context.Background()

	t.Run("returns error for invalid payload", func(t *testing.T) {
		task := asynq.NewTask(TypeInstallDatabase, []byte("invalid"))

		err := registry.InstallDatabase.Handle(ctx, task)
		assert.Error(t, err)
	})
}

func TestUninstallDatabaseJob_Handle(t *testing.T) {
	registry := setupTestRegistry(t)
	ctx := context.Background()

	t.Run("returns error for invalid payload", func(t *testing.T) {
		task := asynq.NewTask(TypeUninstallDatabase, []byte("invalid"))

		err := registry.UninstallDatabase.Handle(ctx, task)
		assert.Error(t, err)
	})
}

func TestInstallDatabaseUserJob_Handle(t *testing.T) {
	registry := setupTestRegistry(t)
	ctx := context.Background()

	t.Run("returns error for invalid payload", func(t *testing.T) {
		task := asynq.NewTask(TypeInstallDatabaseUser, []byte("invalid"))

		err := registry.InstallDatabaseUser.Handle(ctx, task)
		assert.Error(t, err)
	})
}

func TestUpdateDatabaseUserJob_Handle(t *testing.T) {
	registry := setupTestRegistry(t)
	ctx := context.Background()

	t.Run("returns error for invalid payload", func(t *testing.T) {
		task := asynq.NewTask(TypeUpdateDatabaseUser, []byte("invalid"))

		err := registry.UpdateDatabaseUser.Handle(ctx, task)
		assert.Error(t, err)
	})
}

func TestUninstallDatabaseUserJob_Handle(t *testing.T) {
	registry := setupTestRegistry(t)
	ctx := context.Background()

	t.Run("returns error for invalid payload", func(t *testing.T) {
		task := asynq.NewTask(TypeUninstallDatabaseUser, []byte("invalid"))

		err := registry.UninstallDatabaseUser.Handle(ctx, task)
		assert.Error(t, err)
	})
}

func TestSyncDatabasesJob_Handle(t *testing.T) {
	registry := setupTestRegistry(t)
	ctx := context.Background()

	t.Run("returns error for invalid payload", func(t *testing.T) {
		task := asynq.NewTask(TypeSyncDatabases, []byte("invalid"))

		err := registry.SyncDatabases.Handle(ctx, task)
		assert.Error(t, err)
	})
}

func TestGetDatabaseScriptTemplate(t *testing.T) {
	t.Run("returns mysql script", func(t *testing.T) {
		script := GetDatabaseScriptTemplate("mysql")

		assert.Contains(t, script, "mysql")
		assert.Contains(t, script, "CREATE DATABASE")
	})

	t.Run("returns postgres script", func(t *testing.T) {
		script := GetDatabaseScriptTemplate("postgres")

		assert.Contains(t, script, "psql")
		assert.Contains(t, script, "CREATE DATABASE")
	})

	t.Run("returns empty for unknown type", func(t *testing.T) {
		script := GetDatabaseScriptTemplate("unknown")

		assert.Empty(t, script)
	})
}

func TestGetDropDatabaseScriptTemplate(t *testing.T) {
	t.Run("returns mysql script", func(t *testing.T) {
		script := GetDropDatabaseScriptTemplate("mysql")

		assert.Contains(t, script, "mysql")
		assert.Contains(t, script, "DROP DATABASE")
	})

	t.Run("returns postgres script", func(t *testing.T) {
		script := GetDropDatabaseScriptTemplate("postgres")

		assert.Contains(t, script, "psql")
		assert.Contains(t, script, "DROP DATABASE")
	})

	t.Run("returns empty for unknown type", func(t *testing.T) {
		script := GetDropDatabaseScriptTemplate("unknown")

		assert.Empty(t, script)
	})
}

func TestGetCreateUserScriptTemplate(t *testing.T) {
	t.Run("returns mysql script", func(t *testing.T) {
		script := GetCreateUserScriptTemplate("mysql")

		assert.Contains(t, script, "mysql")
		assert.Contains(t, script, "CREATE USER")
		assert.Contains(t, script, "GRANT")
	})

	t.Run("returns postgres script", func(t *testing.T) {
		script := GetCreateUserScriptTemplate("postgres")

		assert.Contains(t, script, "psql")
		assert.Contains(t, script, "CREATE USER")
		assert.Contains(t, script, "GRANT")
	})

	t.Run("returns empty for unknown type", func(t *testing.T) {
		script := GetCreateUserScriptTemplate("unknown")

		assert.Empty(t, script)
	})
}

func TestGetDropUserScriptTemplate(t *testing.T) {
	t.Run("returns mysql script", func(t *testing.T) {
		script := GetDropUserScriptTemplate("mysql")

		assert.Contains(t, script, "mysql")
		assert.Contains(t, script, "DROP USER")
	})

	t.Run("returns postgres script", func(t *testing.T) {
		script := GetDropUserScriptTemplate("postgres")

		assert.Contains(t, script, "psql")
		assert.Contains(t, script, "DROP USER")
	})

	t.Run("returns empty for unknown type", func(t *testing.T) {
		script := GetDropUserScriptTemplate("unknown")

		assert.Empty(t, script)
	})
}

func TestTaskTypes(t *testing.T) {
	assert.Equal(t, "database:install", TypeInstallDatabase)
	assert.Equal(t, "database:uninstall", TypeUninstallDatabase)
	assert.Equal(t, "database:user:install", TypeInstallDatabaseUser)
	assert.Equal(t, "database:user:update", TypeUpdateDatabaseUser)
	assert.Equal(t, "database:user:uninstall", TypeUninstallDatabaseUser)
	assert.Equal(t, "database:sync", TypeSyncDatabases)
}

func TestAllTaskTypes(t *testing.T) {
	taskTypes := AllTaskTypes()

	assert.Len(t, taskTypes, 6)
	assert.Contains(t, taskTypes, TypeInstallDatabase)
	assert.Contains(t, taskTypes, TypeUninstallDatabase)
	assert.Contains(t, taskTypes, TypeInstallDatabaseUser)
	assert.Contains(t, taskTypes, TypeUpdateDatabaseUser)
	assert.Contains(t, taskTypes, TypeUninstallDatabaseUser)
	assert.Contains(t, taskTypes, TypeSyncDatabases)
}

func TestRegistry_RegisterHandlers(t *testing.T) {
	registry := setupTestRegistry(t)
	mux := asynq.NewServeMux()

	registry.RegisterHandlers(mux)

	// Verify all handlers are registered by checking mux has handlers
	// The mux doesn't expose a way to check registered handlers,
	// but we can verify it doesn't panic
	assert.NotNil(t, mux)
}
