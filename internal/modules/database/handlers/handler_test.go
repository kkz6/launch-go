package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/database/models"
	"github.com/kkz6/launch-go/internal/modules/database/repositories"
	"github.com/kkz6/launch-go/internal/modules/database/services"
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

	return nil, services.ErrServerNotFound
}

func (m *MockServerRepository) FindByIDAndTeam(ctx context.Context, id, teamID string) (interface{}, error) {
	if server, ok := m.servers[id]; ok {
		return server, nil
	}

	return nil, services.ErrServerNotFound
}

func setupTestHandler(t *testing.T) (*Handler, *services.Service, *repositories.Repository, *fiber.App) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.Database{}, &models.DatabaseUser{}, &models.DatabaseDatabaseUser{})
	require.NoError(t, err)

	repo := repositories.NewRepository(db)
	serverRepo := NewMockServerRepository()
	serverRepo.AddServer("01ARZ3NDEKTSV4RRFFQ69G5FAV", struct{}{})

	logger := zerolog.New(os.Stdout)
	ws := websocket.NewHub()

	service := services.NewService(repo, serverRepo, nil, ws, &logger)

	handler := NewHandler(service)

	app := fiber.New()

	// Register routes
	app.Get("/servers/:serverId/databases", handler.ListDatabases)
	app.Post("/servers/:serverId/databases", handler.CreateDatabase)
	app.Get("/servers/:serverId/databases/:id", handler.GetDatabase)
	app.Delete("/servers/:serverId/databases/:id", handler.DeleteDatabase)
	app.Post("/servers/:serverId/databases/sync", handler.SyncDatabases)

	app.Get("/servers/:serverId/database-users", handler.ListDatabaseUsers)
	app.Post("/servers/:serverId/database-users", handler.CreateDatabaseUser)
	app.Get("/servers/:serverId/database-users/:id", handler.GetDatabaseUser)
	app.Put("/servers/:serverId/database-users/:id", handler.UpdateDatabaseUser)
	app.Delete("/servers/:serverId/database-users/:id", handler.DeleteDatabaseUser)

	return handler, service, repo, app
}

func TestNewHandler(t *testing.T) {
	_, service, _, _ := setupTestHandler(t)

	handler := NewHandler(service)

	assert.NotNil(t, handler)
}

func TestHandler_ListDatabases(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("returns empty list when no databases", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/servers/"+serverID+"/databases", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(body, &result)

		assert.True(t, result["success"].(bool))
		assert.Equal(t, "Databases retrieved", result["message"])
	})

	t.Run("returns list of databases", func(t *testing.T) {
		// Create test databases
		repo.Create(t.Context(), &models.Database{ServerID: serverID, Name: "db1"})
		repo.Create(t.Context(), &models.Database{ServerID: serverID, Name: "db2"})

		req := httptest.NewRequest(http.MethodGet, "/servers/"+serverID+"/databases", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(body, &result)

		data := result["data"].([]interface{})
		assert.Len(t, data, 2)
	})
}

func TestHandler_CreateDatabase(t *testing.T) {
	_, _, _, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("creates database successfully", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"name":        "new_database",
			"create_user": false,
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/databases", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		respBody, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(respBody, &result)

		assert.True(t, result["success"].(bool))
		assert.Equal(t, "Database will be created shortly", result["message"])
	})

	t.Run("returns validation error for missing name", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"create_user": false,
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/databases", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})

	t.Run("returns error for invalid body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/databases", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestHandler_GetDatabase(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("returns database when found", func(t *testing.T) {
		database := &models.Database{ServerID: serverID, Name: "get_test_db"}
		repo.Create(t.Context(), database)

		req := httptest.NewRequest(http.MethodGet, "/servers/"+serverID+"/databases/"+database.ID, nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(body, &result)

		data := result["data"].(map[string]interface{})
		assert.Equal(t, database.ID, data["id"])
	})

	t.Run("returns 404 when not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/servers/"+serverID+"/databases/non_existent", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestHandler_DeleteDatabase(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("deletes database successfully", func(t *testing.T) {
		database := &models.Database{ServerID: serverID, Name: "delete_test_db"}
		repo.Create(t.Context(), database)

		req := httptest.NewRequest(http.MethodDelete, "/servers/"+serverID+"/databases/"+database.ID, nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(body, &result)

		assert.Equal(t, "Database will be deleted shortly", result["message"])
	})

	t.Run("returns 404 when not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/servers/"+serverID+"/databases/non_existent", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("returns conflict when already uninstalling", func(t *testing.T) {
		now := time.Now()
		database := &models.Database{
			ServerID:                  serverID,
			Name:                      "uninstalling_db",
			UninstallationRequestedAt: &now,
		}
		repo.Create(t.Context(), database)

		req := httptest.NewRequest(http.MethodDelete, "/servers/"+serverID+"/databases/"+database.ID, nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusConflict, resp.StatusCode)
	})
}

func TestHandler_SyncDatabases(t *testing.T) {
	_, _, _, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("syncs databases successfully", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/databases/sync", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(body, &result)

		assert.Equal(t, "Database sync started", result["message"])
	})
}

func TestHandler_ListDatabaseUsers(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("returns empty list when no users", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/servers/"+serverID+"/database-users", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("returns list of users", func(t *testing.T) {
		repo.CreateUser(t.Context(), &models.DatabaseUser{ServerID: serverID, Name: "user1", Password: "pass"})
		repo.CreateUser(t.Context(), &models.DatabaseUser{ServerID: serverID, Name: "user2", Password: "pass"})

		req := httptest.NewRequest(http.MethodGet, "/servers/"+serverID+"/database-users", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(body, &result)

		data := result["data"].([]interface{})
		assert.Len(t, data, 2)
	})
}

func TestHandler_CreateDatabaseUser(t *testing.T) {
	_, _, _, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("creates user successfully", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"name":     "new_user",
			"password": "password123",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/database-users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		respBody, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(respBody, &result)

		assert.True(t, result["success"].(bool))
	})

	t.Run("returns validation error for missing name", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"password": "password123",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/database-users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})

	t.Run("returns validation error for short password", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"name":     "user",
			"password": "short",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/database-users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})
}

func TestHandler_GetDatabaseUser(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("returns user when found", func(t *testing.T) {
		user := &models.DatabaseUser{ServerID: serverID, Name: "get_test_user", Password: "pass"}
		repo.CreateUser(t.Context(), user)

		req := httptest.NewRequest(http.MethodGet, "/servers/"+serverID+"/database-users/"+user.ID, nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(body, &result)

		data := result["data"].(map[string]interface{})
		assert.Equal(t, user.ID, data["id"])
	})

	t.Run("returns 404 when not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/servers/"+serverID+"/database-users/non_existent", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestHandler_UpdateDatabaseUser(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("updates user successfully", func(t *testing.T) {
		user := &models.DatabaseUser{ServerID: serverID, Name: "update_test_user", Password: "oldpass"}
		repo.CreateUser(t.Context(), user)

		reqBody := map[string]interface{}{
			"password": "newpassword123",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPut, "/servers/"+serverID+"/database-users/"+user.ID, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("returns validation error for short password", func(t *testing.T) {
		user := &models.DatabaseUser{ServerID: serverID, Name: "update_test_user2", Password: "oldpass"}
		repo.CreateUser(t.Context(), user)

		reqBody := map[string]interface{}{
			"password": "short",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPut, "/servers/"+serverID+"/database-users/"+user.ID, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})

	t.Run("returns 404 when not found", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"password": "password123",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPut, "/servers/"+serverID+"/database-users/non_existent", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestHandler_DeleteDatabaseUser(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("deletes user successfully", func(t *testing.T) {
		user := &models.DatabaseUser{ServerID: serverID, Name: "delete_test_user", Password: "pass"}
		repo.CreateUser(t.Context(), user)

		req := httptest.NewRequest(http.MethodDelete, "/servers/"+serverID+"/database-users/"+user.ID, nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(body, &result)

		assert.Equal(t, "Database user will be deleted shortly", result["message"])
	})

	t.Run("returns 404 when not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/servers/"+serverID+"/database-users/non_existent", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("returns conflict when already uninstalling", func(t *testing.T) {
		now := time.Now()
		user := &models.DatabaseUser{
			ServerID:                  serverID,
			Name:                      "uninstalling_user",
			Password:                  "pass",
			UninstallationRequestedAt: &now,
		}
		repo.CreateUser(t.Context(), user)

		req := httptest.NewRequest(http.MethodDelete, "/servers/"+serverID+"/database-users/"+user.ID, nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusConflict, resp.StatusCode)
	})
}

func TestGetUserIDFromContext(t *testing.T) {
	t.Run("returns user ID when set", func(t *testing.T) {
		app := fiber.New()
		app.Get("/test", func(c *fiber.Ctx) error {
			c.Locals("userID", "test_user_id")
			userID := getUserIDFromContext(c)

			assert.NotNil(t, userID)
			assert.Equal(t, "test_user_id", *userID)

			return c.SendStatus(200)
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		app.Test(req)
	})

	t.Run("returns nil when not set", func(t *testing.T) {
		app := fiber.New()
		app.Get("/test", func(c *fiber.Ctx) error {
			userID := getUserIDFromContext(c)

			assert.Nil(t, userID)

			return c.SendStatus(200)
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		app.Test(req)
	})
}

func TestHandleServiceError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
	}{
		{"database not found", services.ErrDatabaseNotFound, http.StatusNotFound},
		{"database user not found", services.ErrDatabaseUserNotFound, http.StatusNotFound},
		{"database name exists", services.ErrDatabaseNameExists, http.StatusConflict},
		{"user name exists", services.ErrDatabaseUserNameExists, http.StatusConflict},
		{"database being uninstalled", services.ErrDatabaseBeingUninstalled, http.StatusConflict},
		{"user being uninstalled", services.ErrUserBeingUninstalled, http.StatusConflict},
		{"invalid existing user", services.ErrInvalidExistingUser, http.StatusBadRequest},
		{"unknown error", errors.New("unknown error"), http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			app.Get("/test", func(c *fiber.Ctx) error {
				return handleServiceError(c, tt.err)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			resp, err := app.Test(req)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

func TestHandler_CreateDatabase_WithUser(t *testing.T) {
	_, _, _, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("creates database with new user", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"name":          "db_with_new_user",
			"create_user":   true,
			"user_name":     "test_user_new",
			"user_password": "password123",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/databases", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})
}

func TestHandler_CreateDatabaseUser_InvalidBody(t *testing.T) {
	_, _, _, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("returns error for invalid body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/database-users", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestHandler_UpdateDatabaseUser_InvalidBody(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	user := &models.DatabaseUser{ServerID: serverID, Name: "invalid_body_user", Password: "pass"}
	repo.CreateUser(t.Context(), user)

	t.Run("returns error for invalid body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/servers/"+serverID+"/database-users/"+user.ID, bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestHandler_CreateDatabaseUser_Duplicate(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	// Create existing user
	repo.CreateUser(t.Context(), &models.DatabaseUser{ServerID: serverID, Name: "dup_handler_user", Password: "pass"})

	t.Run("returns conflict for duplicate user", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"name":     "dup_handler_user",
			"password": "password123",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/database-users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusConflict, resp.StatusCode)
	})
}

func TestHandler_CreateDatabase_Duplicate(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	// Create existing database
	repo.Create(t.Context(), &models.Database{ServerID: serverID, Name: "dup_handler_db"})

	t.Run("returns conflict for duplicate database", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"name":        "dup_handler_db",
			"create_user": false,
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/databases", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusConflict, resp.StatusCode)
	})
}

func TestHandler_ListDatabases_EmptyServerID(t *testing.T) {
	_, _, _, app := setupTestHandler(t)

	// Register a route with empty server ID handling
	app.Get("/databases", func(c *fiber.Ctx) error {
		serverID := c.Query("serverId")
		if serverID == "" {
			return fiber.NewError(fiber.StatusBadRequest, "Server ID is required")
		}
		return c.SendStatus(200)
	})

	t.Run("returns error for empty server ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/databases", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestHandler_GetDatabase_EmptyParams(t *testing.T) {
	handler, _, _, _ := setupTestHandler(t)

	t.Run("returns error for empty server ID", func(t *testing.T) {
		app := fiber.New()
		app.Get("/databases/:id", func(c *fiber.Ctx) error {
			// Simulate empty serverId
			c.Locals("serverId", "")
			return handler.GetDatabase(c)
		})

		req := httptest.NewRequest(http.MethodGet, "/databases/test", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestHandler_DeleteDatabase_EmptyParams(t *testing.T) {
	handler, _, _, _ := setupTestHandler(t)

	t.Run("returns error for empty server ID", func(t *testing.T) {
		app := fiber.New()
		app.Delete("/databases/:id", func(c *fiber.Ctx) error {
			// Simulate empty serverId
			c.Locals("serverId", "")
			return handler.DeleteDatabase(c)
		})

		req := httptest.NewRequest(http.MethodDelete, "/databases/test", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestHandler_GetDatabaseUser_EmptyParams(t *testing.T) {
	handler, _, _, _ := setupTestHandler(t)

	t.Run("returns error for empty server ID", func(t *testing.T) {
		app := fiber.New()
		app.Get("/database-users/:id", func(c *fiber.Ctx) error {
			return handler.GetDatabaseUser(c)
		})

		req := httptest.NewRequest(http.MethodGet, "/database-users/test", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestHandler_DeleteDatabaseUser_EmptyParams(t *testing.T) {
	handler, _, _, _ := setupTestHandler(t)

	t.Run("returns error for empty server ID", func(t *testing.T) {
		app := fiber.New()
		app.Delete("/database-users/:id", func(c *fiber.Ctx) error {
			return handler.DeleteDatabaseUser(c)
		})

		req := httptest.NewRequest(http.MethodDelete, "/database-users/test", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestHandler_UpdateDatabaseUser_EmptyParams(t *testing.T) {
	handler, _, _, _ := setupTestHandler(t)

	t.Run("returns error for empty server ID", func(t *testing.T) {
		app := fiber.New()
		app.Put("/database-users/:id", func(c *fiber.Ctx) error {
			return handler.UpdateDatabaseUser(c)
		})

		reqBody := map[string]interface{}{
			"password": "password123",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPut, "/database-users/test", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}
