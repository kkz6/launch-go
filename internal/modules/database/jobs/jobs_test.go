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

func setupTestHandler(t *testing.T) *Handler {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	logger := zerolog.New(os.Stdout)
	ws := websocket.NewHub()

	return NewHandler(db, ws, &logger)
}

func TestNewHandler(t *testing.T) {
	handler := setupTestHandler(t)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.db)
	assert.NotNil(t, handler.ws)
	assert.NotNil(t, handler.logger)
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

	assert.Equal(t, "user_id", payload.UserID)
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

		assert.Equal(t, "user_id", payload.UserID)
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

	assert.Equal(t, "user_id", payload.UserID)
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

func TestHandler_HandleInstallDatabase(t *testing.T) {
	handler := setupTestHandler(t)
	ctx := context.Background()

	t.Run("handles install database task", func(t *testing.T) {
		task, err := NewInstallDatabaseTask("db_id", nil)
		require.NoError(t, err)

		err = handler.HandleInstallDatabase(ctx, task)
		require.NoError(t, err)
	})

	t.Run("returns error for invalid payload", func(t *testing.T) {
		task := asynq.NewTask(TypeInstallDatabase, []byte("invalid"))

		err := handler.HandleInstallDatabase(ctx, task)
		assert.Error(t, err)
	})
}

func TestHandler_HandleUninstallDatabase(t *testing.T) {
	handler := setupTestHandler(t)
	ctx := context.Background()

	t.Run("handles uninstall database task", func(t *testing.T) {
		task, err := NewUninstallDatabaseTask("db_id", nil)
		require.NoError(t, err)

		err = handler.HandleUninstallDatabase(ctx, task)
		require.NoError(t, err)
	})

	t.Run("returns error for invalid payload", func(t *testing.T) {
		task := asynq.NewTask(TypeUninstallDatabase, []byte("invalid"))

		err := handler.HandleUninstallDatabase(ctx, task)
		assert.Error(t, err)
	})
}

func TestHandler_HandleInstallDatabaseUser(t *testing.T) {
	handler := setupTestHandler(t)
	ctx := context.Background()

	t.Run("handles install database user task", func(t *testing.T) {
		task, err := NewInstallDatabaseUserTask("user_id", "password", nil)
		require.NoError(t, err)

		err = handler.HandleInstallDatabaseUser(ctx, task)
		require.NoError(t, err)
	})

	t.Run("returns error for invalid payload", func(t *testing.T) {
		task := asynq.NewTask(TypeInstallDatabaseUser, []byte("invalid"))

		err := handler.HandleInstallDatabaseUser(ctx, task)
		assert.Error(t, err)
	})
}

func TestHandler_HandleUpdateDatabaseUser(t *testing.T) {
	handler := setupTestHandler(t)
	ctx := context.Background()

	t.Run("handles update database user task", func(t *testing.T) {
		password := "new_password"
		task, err := NewUpdateDatabaseUserTask("user_id", &password, nil)
		require.NoError(t, err)

		err = handler.HandleUpdateDatabaseUser(ctx, task)
		require.NoError(t, err)
	})

	t.Run("returns error for invalid payload", func(t *testing.T) {
		task := asynq.NewTask(TypeUpdateDatabaseUser, []byte("invalid"))

		err := handler.HandleUpdateDatabaseUser(ctx, task)
		assert.Error(t, err)
	})
}

func TestHandler_HandleUninstallDatabaseUser(t *testing.T) {
	handler := setupTestHandler(t)
	ctx := context.Background()

	t.Run("handles uninstall database user task", func(t *testing.T) {
		task, err := NewUninstallDatabaseUserTask("user_id", nil)
		require.NoError(t, err)

		err = handler.HandleUninstallDatabaseUser(ctx, task)
		require.NoError(t, err)
	})

	t.Run("returns error for invalid payload", func(t *testing.T) {
		task := asynq.NewTask(TypeUninstallDatabaseUser, []byte("invalid"))

		err := handler.HandleUninstallDatabaseUser(ctx, task)
		assert.Error(t, err)
	})
}

func TestHandler_HandleSyncDatabases(t *testing.T) {
	handler := setupTestHandler(t)
	ctx := context.Background()

	t.Run("handles sync databases task", func(t *testing.T) {
		task, err := NewSyncDatabasesTask("server_id", nil)
		require.NoError(t, err)

		err = handler.HandleSyncDatabases(ctx, task)
		require.NoError(t, err)
	})

	t.Run("returns error for invalid payload", func(t *testing.T) {
		task := asynq.NewTask(TypeSyncDatabases, []byte("invalid"))

		err := handler.HandleSyncDatabases(ctx, task)
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
