package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestHandler(t *testing.T) (*fiber.App, *Handler, *Repository, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(AllModels()...)
	require.NoError(t, err)

	repo := NewRepository(db)
	logger := zerolog.Nop()
	service := NewService(repo, nil, nil, nil, &logger)
	handler := NewHandler(service)

	app := fiber.New()

	// Add middleware to set locals
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("teamID", "01ARZ3NDEKTSV4RRFFQ69G5FAV")
		c.Locals("userID", "01ARZ3NDEKTSV4RRFFQ69G5FAU")
		return c.Next()
	})

	return app, handler, repo, db
}

func createTestServerForHandler(t *testing.T, repo *Repository, teamID, name string) *Server {
	server := &Server{
		TeamID:          teamID,
		UserID:          "01ARZ3NDEKTSV4RRFFQ69G5FAU",
		Name:            name,
		Provider:        ProviderCustom, // Use custom provider to avoid queue operations
		Type:            ServerTypePhp,
		OperatingSystem: OSUbuntu24,
		Status:          ServerStatusRunning,
	}
	err := repo.CreateServer(t.Context(), server)
	require.NoError(t, err)
	return server
}

func performRequest(app *fiber.App, method, path string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")

	// Use a longer timeout for operations that may generate SSH keys
	return app.Test(req, -1)
}

func parseResponse(t *testing.T, resp *http.Response) map[string]interface{} {
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	defer resp.Body.Close()

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)

	return result
}

func TestHandler_List(t *testing.T) {
	app, handler, repo, _ := setupTestHandler(t)
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	app.Get("/servers", handler.List)

	// Create some test servers
	createTestServerForHandler(t, repo, teamID, "server1")
	createTestServerForHandler(t, repo, teamID, "server2")

	resp, err := performRequest(app, "GET", "/servers", nil)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := parseResponse(t, resp)
	data := result["data"].(map[string]interface{})
	servers := data["servers"].([]interface{})
	assert.Len(t, servers, 2)
}

func TestHandler_ListPaginated(t *testing.T) {
	app, handler, repo, _ := setupTestHandler(t)
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	app.Get("/servers", handler.List)

	// Create 5 test servers
	for i := 0; i < 5; i++ {
		createTestServerForHandler(t, repo, teamID, "server-"+string(rune('0'+i)))
	}

	resp, err := performRequest(app, "GET", "/servers?page=1&per_page=2", nil)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := parseResponse(t, resp)
	data := result["data"].(map[string]interface{})
	servers := data["servers"].([]interface{})
	assert.Len(t, servers, 2)
	assert.Equal(t, float64(5), data["total"])
}

func TestHandler_Create(t *testing.T) {
	app, handler, _, _ := setupTestHandler(t)

	app.Post("/servers", handler.Create)

	t.Run("creates server successfully", func(t *testing.T) {
		body := CreateServerRequest{
			Name:            "new-server",
			Provider:        "custom_server",
			Type:            "php",
			OperatingSystem: "ubuntu_24",
			IPAddress:       "192.168.1.1",
			PrivateKey:      "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----",
		}

		resp, err := performRequest(app, "POST", "/servers", body)
		require.NoError(t, err)

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		result := parseResponse(t, resp)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, "new-server", data["name"])
	})

	t.Run("returns error for invalid body", func(t *testing.T) {
		resp, err := performRequest(app, "POST", "/servers", "invalid json")
		require.NoError(t, err)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestHandler_Show(t *testing.T) {
	app, handler, repo, _ := setupTestHandler(t)
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	app.Get("/servers/:id", handler.Show)

	t.Run("shows existing server", func(t *testing.T) {
		server := createTestServerForHandler(t, repo, teamID, "test-server")

		resp, err := performRequest(app, "GET", "/servers/"+server.ID, nil)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		result := parseResponse(t, resp)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, server.ID, data["id"])
	})

	t.Run("returns 404 for non-existent server", func(t *testing.T) {
		resp, err := performRequest(app, "GET", "/servers/non-existent", nil)
		require.NoError(t, err)

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestHandler_Update(t *testing.T) {
	app, handler, repo, _ := setupTestHandler(t)
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	app.Put("/servers/:id", handler.Update)

	t.Run("updates server successfully", func(t *testing.T) {
		server := createTestServerForHandler(t, repo, teamID, "original-name")

		body := UpdateServerRequest{
			Name: strPtr("updated-name"),
		}

		resp, err := performRequest(app, "PUT", "/servers/"+server.ID, body)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		result := parseResponse(t, resp)
		data := result["data"].(map[string]interface{})
		assert.Equal(t, "updated-name", data["name"])
	})

	t.Run("returns 404 for non-existent server", func(t *testing.T) {
		body := UpdateServerRequest{
			Name: strPtr("new-name"),
		}

		resp, err := performRequest(app, "PUT", "/servers/non-existent", body)
		require.NoError(t, err)

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestHandler_Delete(t *testing.T) {
	app, handler, repo, _ := setupTestHandler(t)
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	app.Delete("/servers/:id", handler.Delete)

	t.Run("deletes server successfully", func(t *testing.T) {
		server := createTestServerForHandler(t, repo, teamID, "to-delete")

		resp, err := performRequest(app, "DELETE", "/servers/"+server.ID, nil)
		require.NoError(t, err)

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})

	t.Run("returns 404 for non-existent server", func(t *testing.T) {
		resp, err := performRequest(app, "DELETE", "/servers/non-existent", nil)
		require.NoError(t, err)

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestHandler_Archive(t *testing.T) {
	app, handler, repo, _ := setupTestHandler(t)
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	app.Post("/servers/:id/archive", handler.Archive)

	server := createTestServerForHandler(t, repo, teamID, "to-archive")

	resp, err := performRequest(app, "POST", "/servers/"+server.ID+"/archive", nil)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHandler_Unarchive(t *testing.T) {
	app, handler, repo, _ := setupTestHandler(t)
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	app.Post("/servers/:id/archive", handler.Archive)
	app.Post("/servers/:id/unarchive", handler.Unarchive)

	server := createTestServerForHandler(t, repo, teamID, "to-unarchive")

	// Archive first
	performRequest(app, "POST", "/servers/"+server.ID+"/archive", nil)

	// Then unarchive
	resp, err := performRequest(app, "POST", "/servers/"+server.ID+"/unarchive", nil)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHandler_ListServices(t *testing.T) {
	app, handler, repo, _ := setupTestHandler(t)
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	app.Get("/servers/:id/services", handler.ListServices)

	server := createTestServerForHandler(t, repo, teamID, "test-server")

	// Create services
	php := SoftwarePhp84
	svc := &InstalledService{ServerID: server.ID, Type: ServiceTypePhp, Name: "PHP 8.4", Software: &php}
	repo.CreateService(t.Context(), svc)

	resp, err := performRequest(app, "GET", "/servers/"+server.ID+"/services", nil)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := parseResponse(t, resp)
	data := result["data"].([]interface{})
	assert.Len(t, data, 1)
}

func TestHandler_ListFirewallRules(t *testing.T) {
	app, handler, repo, _ := setupTestHandler(t)
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	app.Get("/servers/:id/firewall-rules", handler.ListFirewallRules)

	server := createTestServerForHandler(t, repo, teamID, "test-server")

	// Create firewall rules
	port := "22"
	rule := &FirewallRule{ServerID: server.ID, Name: "SSH", Port: &port, Action: RuleActionAllow}
	repo.CreateFirewallRule(t.Context(), rule)

	resp, err := performRequest(app, "GET", "/servers/"+server.ID+"/firewall-rules", nil)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := parseResponse(t, resp)
	data := result["data"].([]interface{})
	assert.Len(t, data, 1)
}

func TestHandler_ListCrons(t *testing.T) {
	app, handler, repo, _ := setupTestHandler(t)
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	app.Get("/servers/:id/crons", handler.ListCrons)

	server := createTestServerForHandler(t, repo, teamID, "test-server")

	// Create cron
	cron := &Cron{ServerID: server.ID, User: "root", Expression: "*/5 * * * *", Command: "cmd1"}
	repo.CreateCron(t.Context(), cron)

	resp, err := performRequest(app, "GET", "/servers/"+server.ID+"/crons", nil)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := parseResponse(t, resp)
	data := result["data"].([]interface{})
	assert.Len(t, data, 1)
}

func TestHandler_ListDaemons(t *testing.T) {
	app, handler, repo, _ := setupTestHandler(t)
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	app.Get("/servers/:id/daemons", handler.ListDaemons)

	server := createTestServerForHandler(t, repo, teamID, "test-server")

	// Create daemon
	dir := "/tmp"
	daemon := &Daemon{ServerID: server.ID, Command: "cmd", User: "root", Directory: &dir}
	repo.CreateDaemon(t.Context(), daemon)

	resp, err := performRequest(app, "GET", "/servers/"+server.ID+"/daemons", nil)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := parseResponse(t, resp)
	data := result["data"].([]interface{})
	assert.Len(t, data, 1)
}

func TestHandler_ListSshKeys(t *testing.T) {
	app, handler, repo, _ := setupTestHandler(t)
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	app.Get("/ssh-keys", handler.ListSshKeys)

	// Create SSH keys
	key := &SshKey{TeamID: &teamID, UserID: &teamID, Name: "Key 1", PublicKey: "ssh-rsa AAA1"}
	repo.CreateSshKey(t.Context(), key)

	resp, err := performRequest(app, "GET", "/ssh-keys", nil)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := parseResponse(t, resp)
	data := result["data"].([]interface{})
	assert.Len(t, data, 1)
}

func TestHandler_CreateSshKey(t *testing.T) {
	app, handler, _, _ := setupTestHandler(t)

	app.Post("/ssh-keys", handler.CreateSshKey)

	body := CreateSshKeyRequest{
		Name:      "My SSH Key",
		PublicKey: "ssh-rsa AAAAB3... user@example.com",
	}

	resp, err := performRequest(app, "POST", "/ssh-keys", body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	result := parseResponse(t, resp)
	data := result["data"].(map[string]interface{})
	assert.Equal(t, "My SSH Key", data["name"])
}

func TestHandler_DeleteSshKey(t *testing.T) {
	app, handler, repo, _ := setupTestHandler(t)
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	app.Delete("/ssh-keys/:sshKeyId", handler.DeleteSshKey)

	key := &SshKey{TeamID: &teamID, UserID: &teamID, Name: "To Delete", PublicKey: "ssh-rsa AAA"}
	repo.CreateSshKey(t.Context(), key)

	resp, err := performRequest(app, "DELETE", "/ssh-keys/"+key.ID, nil)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestHandler_ListDatabases(t *testing.T) {
	app, handler, repo, _ := setupTestHandler(t)
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	app.Get("/servers/:id/databases", handler.ListDatabases)

	server := createTestServerForHandler(t, repo, teamID, "test-server")

	resp, err := performRequest(app, "GET", "/servers/"+server.ID+"/databases", nil)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHandler_CreateDatabase(t *testing.T) {
	app, handler, repo, _ := setupTestHandler(t)
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	app.Post("/servers/:id/databases", handler.CreateDatabase)

	server := createTestServerForHandler(t, repo, teamID, "test-server")

	body := CreateDatabaseRequest{
		Name: "my_database",
	}

	resp, err := performRequest(app, "POST", "/servers/"+server.ID+"/databases", body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	result := parseResponse(t, resp)
	data := result["data"].(map[string]interface{})
	assert.Equal(t, "my_database", data["name"])
}

func TestHandler_ListTasks(t *testing.T) {
	app, handler, repo, _ := setupTestHandler(t)
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	app.Get("/servers/:id/tasks", handler.ListTasks)

	server := createTestServerForHandler(t, repo, teamID, "test-server")

	// Create tasks
	taskName := "provision"
	taskScript := "cmd"
	task := &Task{ServerID: server.ID, Type: "provision", Name: &taskName, Script: &taskScript}
	repo.CreateTask(t.Context(), task)

	resp, err := performRequest(app, "GET", "/servers/"+server.ID+"/tasks", nil)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := parseResponse(t, resp)
	data := result["data"].([]interface{})
	assert.Len(t, data, 1)
}

func TestHandler_GetLatestTask(t *testing.T) {
	app, handler, repo, _ := setupTestHandler(t)
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	app.Get("/servers/:id/tasks/latest", handler.GetLatestTask)

	server := createTestServerForHandler(t, repo, teamID, "test-server")

	taskName := "provision"
	taskScript := "cmd"
	task := &Task{ServerID: server.ID, Type: "provision", Name: &taskName, Script: &taskScript}
	repo.CreateTask(t.Context(), task)

	resp, err := performRequest(app, "GET", "/servers/"+server.ID+"/tasks/latest", nil)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHandler_GetMetrics(t *testing.T) {
	app, handler, repo, _ := setupTestHandler(t)
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	app.Get("/servers/:id/metrics", handler.GetMetrics)

	server := createTestServerForHandler(t, repo, teamID, "test-server")

	// Create metrics
	metric := &Metric{ServerID: server.ID, CPUUsage: 25.5}
	repo.CreateMetric(t.Context(), metric)

	resp, err := performRequest(app, "GET", "/servers/"+server.ID+"/metrics", nil)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := parseResponse(t, resp)
	data := result["data"].([]interface{})
	assert.Len(t, data, 1)
}

func TestHandler_GetLatestMetric(t *testing.T) {
	app, handler, repo, _ := setupTestHandler(t)
	teamID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	app.Get("/servers/:id/metrics/latest", handler.GetLatestMetric)

	server := createTestServerForHandler(t, repo, teamID, "test-server")

	metric := &Metric{ServerID: server.ID, CPUUsage: 25.5}
	repo.CreateMetric(t.Context(), metric)

	resp, err := performRequest(app, "GET", "/servers/"+server.ID+"/metrics/latest", nil)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
