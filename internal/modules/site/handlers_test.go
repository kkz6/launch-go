package site

import (
	"bytes"
	"context"
	"encoding/json"
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

	"github.com/kkz6/launch-go/internal/websocket"
)

func setupTestHandler(t *testing.T) (*Handler, *Service, *Repository, *fiber.App) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&Site{}, &Deployment{}, &Certificate{}, &Queue{}, &Command{}, &Redirect{}, &Release{})
	require.NoError(t, err)

	repo := NewRepository(db)
	logger := zerolog.New(os.Stdout)
	ws := websocket.NewHub()

	service := &Service{
		repo:   repo,
		queue:  nil,
		ws:     ws,
		logger: &logger,
	}

	handler := NewHandler(service)

	app := fiber.New()

	// Add middleware to set user context
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("userID", "01ARZ3NDEKTSV4RRFFQ69G5FAU")
		c.Locals("username", "deploy")
		return c.Next()
	})

	// Register routes
	servers := app.Group("/servers/:serverId")
	sites := servers.Group("/sites")

	sites.Get("/", handler.List)
	sites.Post("/", handler.Create)
	sites.Get("/:id", handler.Show)
	sites.Put("/:id", handler.Update)
	sites.Delete("/:id", handler.Delete)
	sites.Get("/:id/deletion-summary", handler.GetDeletionSummary)

	sites.Post("/:id/deploy", handler.Deploy)
	sites.Get("/:id/deployments", handler.ListDeployments)
	sites.Get("/:id/deployments/:deploymentId", handler.ShowDeployment)
	sites.Post("/:id/rollback/:deploymentId", handler.Rollback)
	sites.Delete("/:id/deployments/queued", handler.CancelQueuedDeployments)

	sites.Post("/:id/auto-deployment/enable", handler.EnableAutoDeployment)
	sites.Post("/:id/auto-deployment/disable", handler.DisableAutoDeployment)
	sites.Post("/:id/auto-restart-queue/enable", handler.EnableAutoRestartQueue)
	sites.Post("/:id/auto-restart-queue/disable", handler.DisableAutoRestartQueue)
	sites.Post("/:id/deploy-token/regenerate", handler.RegenerateDeployToken)
	sites.Put("/:id/deployment-settings", handler.UpdateDeploymentSettings)

	sites.Put("/:id/ssl", handler.UpdateSSL)
	sites.Get("/:id/certificates", handler.ListCertificates)

	sites.Get("/:id/queues", handler.ListQueues)
	sites.Post("/:id/queues", handler.CreateQueue)
	sites.Delete("/:id/queues/:queueId", handler.DeleteQueue)

	sites.Get("/:id/commands", handler.ListCommands)
	sites.Post("/:id/commands", handler.CreateCommand)

	sites.Get("/:id/redirects", handler.ListRedirects)
	sites.Post("/:id/redirects", handler.CreateRedirect)
	sites.Delete("/:id/redirects/:redirectId", handler.DeleteRedirect)

	return handler, service, repo, app
}

func createHandlerTestSite(t *testing.T, repo *Repository, serverID, address string) *Site {
	site := &Site{
		ServerID:               serverID,
		UserID:                 "01ARZ3NDEKTSV4RRFFQ69G5FAU",
		Address:                address,
		Type:                   SiteTypeLaravel,
		User:                   "deploy",
		Path:                   "/home/deploy/" + address,
		ZeroDowntimeDeployment: true,
	}
	err := repo.Create(context.Background(), site)
	require.NoError(t, err)

	return site
}

func TestNewHandler(t *testing.T) {
	_, service, _, _ := setupTestHandler(t)

	handler := NewHandler(service)

	assert.NotNil(t, handler)
	assert.Equal(t, service, handler.service)
}

func TestHandler_List(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("returns empty list when no sites", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/servers/"+serverID+"/sites", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(body, &result)

		assert.True(t, result["success"].(bool))
		assert.Equal(t, "Sites retrieved", result["message"])
	})

	t.Run("returns list of sites", func(t *testing.T) {
		createHandlerTestSite(t, repo, serverID, "site1.com")
		createHandlerTestSite(t, repo, serverID, "site2.com")

		req := httptest.NewRequest(http.MethodGet, "/servers/"+serverID+"/sites", nil)
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

func TestHandler_Create(t *testing.T) {
	_, _, _, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("creates site successfully", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"address":                   "newsite.com",
			"type":                      "laravel",
			"php_version":               "8.3",
			"zero_downtime_deployment":  true,
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/sites", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		respBody, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(respBody, &result)

		assert.True(t, result["success"].(bool))
		assert.Equal(t, "Site created", result["message"])

		data := result["data"].(map[string]interface{})
		assert.NotEmpty(t, data["id"])
		assert.Equal(t, "newsite.com", data["address"])
	})

	t.Run("returns validation error for missing address", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"type":        "laravel",
			"php_version": "8.3",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/sites", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})

	t.Run("returns validation error for invalid type", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"address":     "invalid.com",
			"type":        "invalid",
			"php_version": "8.3",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/sites", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})

	t.Run("returns error for invalid body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/sites", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestHandler_Show(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("returns site when found", func(t *testing.T) {
		site := createHandlerTestSite(t, repo, serverID, "show.com")

		req := httptest.NewRequest(http.MethodGet, "/servers/"+serverID+"/sites/"+site.ID, nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(body, &result)

		data := result["data"].(map[string]interface{})
		assert.Equal(t, site.ID, data["id"])
	})

	t.Run("returns 404 when not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/servers/"+serverID+"/sites/non_existent", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestHandler_Update(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("updates site successfully", func(t *testing.T) {
		site := createHandlerTestSite(t, repo, serverID, "update.com")

		reqBody := map[string]interface{}{
			"php_version": "8.4",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPut, "/servers/"+serverID+"/sites/"+site.ID, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		respBody, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(respBody, &result)

		data := result["data"].(map[string]interface{})
		assert.Equal(t, "8.4", data["php_version"])
	})

	t.Run("returns 404 when not found", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"php_version": "8.4",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPut, "/servers/"+serverID+"/sites/non_existent", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestHandler_Delete(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("deletes site successfully", func(t *testing.T) {
		site := createHandlerTestSite(t, repo, serverID, "delete.com")

		req := httptest.NewRequest(http.MethodDelete, "/servers/"+serverID+"/sites/"+site.ID, nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(body, &result)

		assert.Equal(t, "Site deletion initiated", result["message"])
	})

	t.Run("returns 404 when not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/servers/"+serverID+"/sites/non_existent", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestHandler_Deploy(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("creates deployment successfully", func(t *testing.T) {
		site := createHandlerTestSite(t, repo, serverID, "deploy.com")

		req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/sites/"+site.ID+"/deploy", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(body, &result)

		assert.Equal(t, "Deployment started", result["message"])
	})

	t.Run("returns 404 when site not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/sites/non_existent/deploy", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("returns 409 when deployment in progress", func(t *testing.T) {
		site := createHandlerTestSite(t, repo, serverID, "deploy-conflict.com")
		repo.CreateDeployment(context.Background(), &Deployment{SiteID: site.ID, Status: DeploymentStatusInstalling})

		req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/sites/"+site.ID+"/deploy", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusConflict, resp.StatusCode)
	})
}

func TestHandler_Rollback(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("creates rollback successfully", func(t *testing.T) {
		site := createHandlerTestSite(t, repo, serverID, "rollback.com")

		targetDeployment := &Deployment{SiteID: site.ID, Status: DeploymentStatusFinished}
		repo.CreateDeployment(context.Background(), targetDeployment)

		latestDeployment := &Deployment{SiteID: site.ID, Status: DeploymentStatusFinished}
		repo.CreateDeployment(context.Background(), latestDeployment)

		req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/sites/"+site.ID+"/rollback/"+targetDeployment.ID, nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("returns error for non-zero-downtime site", func(t *testing.T) {
		site := createHandlerTestSite(t, repo, serverID, "no-rollback.com")
		site.ZeroDowntimeDeployment = false
		repo.Update(context.Background(), site)

		deployment := &Deployment{SiteID: site.ID, Status: DeploymentStatusFinished}
		repo.CreateDeployment(context.Background(), deployment)

		req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/sites/"+site.ID+"/rollback/"+deployment.ID, nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestHandler_ListDeployments(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("returns list of deployments", func(t *testing.T) {
		site := createHandlerTestSite(t, repo, serverID, "deployments.com")
		repo.CreateDeployment(context.Background(), &Deployment{SiteID: site.ID, Status: DeploymentStatusFinished})
		repo.CreateDeployment(context.Background(), &Deployment{SiteID: site.ID, Status: DeploymentStatusPending})

		req := httptest.NewRequest(http.MethodGet, "/servers/"+serverID+"/sites/"+site.ID+"/deployments", nil)
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

func TestHandler_ShowDeployment(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("returns deployment when found", func(t *testing.T) {
		site := createHandlerTestSite(t, repo, serverID, "show-deploy.com")
		deployment := &Deployment{SiteID: site.ID, Status: DeploymentStatusPending}
		repo.CreateDeployment(context.Background(), deployment)

		req := httptest.NewRequest(http.MethodGet, "/servers/"+serverID+"/sites/"+site.ID+"/deployments/"+deployment.ID, nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(body, &result)

		data := result["data"].(map[string]interface{})
		assert.Equal(t, deployment.ID, data["id"])
	})

	t.Run("returns 404 when not found", func(t *testing.T) {
		site := createHandlerTestSite(t, repo, serverID, "no-deploy.com")

		req := httptest.NewRequest(http.MethodGet, "/servers/"+serverID+"/sites/"+site.ID+"/deployments/non_existent", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestHandler_CancelQueuedDeployments(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("cancels queued deployments", func(t *testing.T) {
		site := createHandlerTestSite(t, repo, serverID, "cancel.com")
		repo.CreateDeployment(context.Background(), &Deployment{SiteID: site.ID, Status: DeploymentStatusQueued})
		repo.CreateDeployment(context.Background(), &Deployment{SiteID: site.ID, Status: DeploymentStatusQueued})

		req := httptest.NewRequest(http.MethodDelete, "/servers/"+serverID+"/sites/"+site.ID+"/deployments/queued", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(body, &result)

		data := result["data"].(map[string]interface{})
		assert.Equal(t, float64(2), data["cancelled"])
	})
}

func TestHandler_EnableAutoDeployment(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("enables auto deployment", func(t *testing.T) {
		site := createHandlerTestSite(t, repo, serverID, "auto-enable.com")
		scID := "01ARZ3NDEKTSV4RRFFQ69G5FAW"
		repoID := "01ARZ3NDEKTSV4RRFFQ69G5FAX"
		site.SourceControlID = &scID
		site.SourceControlRepositoriesID = &repoID
		repo.Update(context.Background(), site)

		req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/sites/"+site.ID+"/auto-deployment/enable", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("returns error when source control not connected", func(t *testing.T) {
		site := createHandlerTestSite(t, repo, serverID, "no-sc.com")

		req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/sites/"+site.ID+"/auto-deployment/enable", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestHandler_DisableAutoDeployment(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("disables auto deployment", func(t *testing.T) {
		site := createHandlerTestSite(t, repo, serverID, "auto-disable.com")
		site.AutoDeployment = true
		repo.Update(context.Background(), site)

		req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/sites/"+site.ID+"/auto-deployment/disable", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestHandler_EnableAutoRestartQueue(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site := createHandlerTestSite(t, repo, serverID, "restart-enable.com")

	req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/sites/"+site.ID+"/auto-restart-queue/enable", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHandler_DisableAutoRestartQueue(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site := createHandlerTestSite(t, repo, serverID, "restart-disable.com")

	req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/sites/"+site.ID+"/auto-restart-queue/disable", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHandler_RegenerateDeployToken(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site := createHandlerTestSite(t, repo, serverID, "token.com")

	req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/sites/"+site.ID+"/deploy-token/regenerate", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHandler_GetDeletionSummary(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site := createHandlerTestSite(t, repo, serverID, "summary.com")
	repo.CreateQueue(context.Background(), &Queue{SiteID: site.ID, ServerID: serverID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU"})

	req := httptest.NewRequest(http.MethodGet, "/servers/"+serverID+"/sites/"+site.ID+"/deletion-summary", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	data := result["data"].(map[string]interface{})
	assert.Equal(t, float64(1), data["queues"])
}

func TestHandler_UpdateSSL(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("updates SSL setting", func(t *testing.T) {
		site := createHandlerTestSite(t, repo, serverID, "ssl.com")

		reqBody := map[string]interface{}{
			"tls_setting": "off",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPut, "/servers/"+serverID+"/sites/"+site.ID+"/ssl", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("returns validation error for invalid TLS setting", func(t *testing.T) {
		site := createHandlerTestSite(t, repo, serverID, "ssl-invalid.com")

		reqBody := map[string]interface{}{
			"tls_setting": "invalid",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPut, "/servers/"+serverID+"/sites/"+site.ID+"/ssl", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})
}

func TestHandler_ListCertificates(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site := createHandlerTestSite(t, repo, serverID, "certs.com")
	repo.CreateCertificate(context.Background(), &Certificate{SiteID: site.ID, Type: CertificateTypeAuto})

	req := httptest.NewRequest(http.MethodGet, "/servers/"+serverID+"/sites/"+site.ID+"/certificates", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	data := result["data"].([]interface{})
	assert.Len(t, data, 1)
}

func TestHandler_CreateQueue(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("creates queue successfully", func(t *testing.T) {
		site := createHandlerTestSite(t, repo, serverID, "queue.com")

		reqBody := map[string]interface{}{
			"queue_connection":        "redis",
			"queue":                   "default",
			"rest_seconds_on_empty":   5,
			"max_seconds_per_job":     60,
			"failed_job_delay_seconds": 3,
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/sites/"+site.ID+"/queues", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("returns validation error for missing fields", func(t *testing.T) {
		site := createHandlerTestSite(t, repo, serverID, "queue-invalid.com")

		reqBody := map[string]interface{}{
			"queue_connection": "redis",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/sites/"+site.ID+"/queues", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})
}

func TestHandler_ListQueues(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site := createHandlerTestSite(t, repo, serverID, "list-queues.com")
	repo.CreateQueue(context.Background(), &Queue{SiteID: site.ID, ServerID: serverID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU"})

	req := httptest.NewRequest(http.MethodGet, "/servers/"+serverID+"/sites/"+site.ID+"/queues", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	data := result["data"].([]interface{})
	assert.Len(t, data, 1)
}

func TestHandler_DeleteQueue(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("deletes queue successfully", func(t *testing.T) {
		site := createHandlerTestSite(t, repo, serverID, "delete-queue.com")
		queue := &Queue{SiteID: site.ID, ServerID: serverID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU"}
		repo.CreateQueue(context.Background(), queue)

		req := httptest.NewRequest(http.MethodDelete, "/servers/"+serverID+"/sites/"+site.ID+"/queues/"+queue.ID, nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("returns 404 when queue not found", func(t *testing.T) {
		site := createHandlerTestSite(t, repo, serverID, "no-queue.com")

		req := httptest.NewRequest(http.MethodDelete, "/servers/"+serverID+"/sites/"+site.ID+"/queues/non_existent", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestHandler_CreateCommand(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("creates command successfully", func(t *testing.T) {
		site := createHandlerTestSite(t, repo, serverID, "command.com")
		now := time.Now()
		site.InstalledAt = &now
		repo.Update(context.Background(), site)

		reqBody := map[string]interface{}{
			"command": "php artisan migrate",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/sites/"+site.ID+"/commands", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("returns error when site not installed", func(t *testing.T) {
		site := createHandlerTestSite(t, repo, serverID, "not-installed.com")

		reqBody := map[string]interface{}{
			"command": "php artisan migrate",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/sites/"+site.ID+"/commands", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestHandler_ListCommands(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site := createHandlerTestSite(t, repo, serverID, "list-cmds.com")
	repo.CreateCommand(context.Background(), &Command{SiteID: site.ID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU", Command: "test"})

	req := httptest.NewRequest(http.MethodGet, "/servers/"+serverID+"/sites/"+site.ID+"/commands", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	data := result["data"].([]interface{})
	assert.Len(t, data, 1)
}

func TestHandler_CreateRedirect(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("creates redirect successfully", func(t *testing.T) {
		site := createHandlerTestSite(t, repo, serverID, "redirect.com")

		reqBody := map[string]interface{}{
			"from": "/old",
			"to":   "/new",
			"mode": 1,
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/sites/"+site.ID+"/redirects", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("returns validation error for missing fields", func(t *testing.T) {
		site := createHandlerTestSite(t, repo, serverID, "redirect-invalid.com")

		reqBody := map[string]interface{}{
			"from": "/old",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/servers/"+serverID+"/sites/"+site.ID+"/redirects", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})
}

func TestHandler_ListRedirects(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	site := createHandlerTestSite(t, repo, serverID, "list-redirects.com")
	repo.CreateRedirect(context.Background(), &Redirect{SiteID: site.ID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU", Mode: RedirectModePermanent, From: "/a", To: "/b"})

	req := httptest.NewRequest(http.MethodGet, "/servers/"+serverID+"/sites/"+site.ID+"/redirects", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	data := result["data"].([]interface{})
	assert.Len(t, data, 1)
}

func TestHandler_DeleteRedirect(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("deletes redirect successfully", func(t *testing.T) {
		site := createHandlerTestSite(t, repo, serverID, "delete-redirect.com")
		redirect := &Redirect{SiteID: site.ID, UserID: "01ARZ3NDEKTSV4RRFFQ69G5FAU", Mode: RedirectModePermanent, From: "/a", To: "/b"}
		repo.CreateRedirect(context.Background(), redirect)

		req := httptest.NewRequest(http.MethodDelete, "/servers/"+serverID+"/sites/"+site.ID+"/redirects/"+redirect.ID, nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("returns 404 when redirect not found", func(t *testing.T) {
		site := createHandlerTestSite(t, repo, serverID, "no-redirect.com")

		req := httptest.NewRequest(http.MethodDelete, "/servers/"+serverID+"/sites/"+site.ID+"/redirects/non_existent", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestHandler_UpdateDeploymentSettings(t *testing.T) {
	_, _, repo, app := setupTestHandler(t)
	serverID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("updates deployment settings successfully", func(t *testing.T) {
		site := createHandlerTestSite(t, repo, serverID, "settings.com")

		reqBody := map[string]interface{}{
			"deployment_releases_retention": 10,
			"queue_deployments":             true,
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPut, "/servers/"+serverID+"/sites/"+site.ID+"/deployment-settings", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		respBody, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(respBody, &result)

		data := result["data"].(map[string]interface{})
		assert.Equal(t, float64(10), data["deployment_releases_retention"])
		assert.Equal(t, true, data["queue_deployments"])
	})

	t.Run("returns 404 when site not found", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"deployment_releases_retention": 10,
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPut, "/servers/"+serverID+"/sites/non_existent/deployment-settings", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}
