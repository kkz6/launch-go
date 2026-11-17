package handlers

import (
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/modules/server/services"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/pkg/signedurl"
)

func TestProvisioningHTTPHandlers(t *testing.T) {
	const id = "01HJXVHGRGTQRX4P0G3Y8R6CK7"
	const missingID = "01HJXVHGRGTQRX4P0G3Y8R6CK8"
	const serverPath = "/servers/" + id
	const downloadPath = "/provision/" + id
	const callbackPrefix = "/webhooks/servers/"
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	require.NoError(t, err)
	conn, err := db.DB()
	require.NoError(t, err)
	conn.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	require.NoError(t, db.AutoMigrate(&models.Server{}, &models.InstalledService{}))
	server := models.Server{
		BaseModel: basemodels.BaseModel{ID: id}, TeamScoped: basemodels.TeamScoped{TeamID: "team-a"},
		Provider: types.ProviderCustom, Status: types.ServerStatusAwaitingConnection,
	}
	require.NoError(t, db.Create(&server).Error)
	repos := repositories.NewRegistry(db)
	service := services.NewService(services.ServiceDeps{Repos: repos, FrontendURL: "https://app.example.test"})
	signer := signedurl.NewSigner("provision-test")
	previous := signedurl.GetDefaultSigner()
	signedurl.SetDefaultSigner(signer)
	t.Cleanup(func() { signedurl.SetDefaultSigner(previous) })

	app := fiber.New(fiber.Config{ErrorHandler: fiberutil.NewErrorHandler()})
	app.Use("/servers", func(c *fiber.Ctx) error {
		fiberutil.SetTeamContext(c, "team-a", "editor")
		return c.Next()
	})
	h := NewHandler(service, nil, nil, nil)
	script := NewProvisionScriptHandler(repos, service)
	app.Post("/servers/:id/provision-command", h.GetProvisionCommand)
	app.Get("/servers/:id/provision-script-content", h.GetProvisionScriptContent)
	app.Get("/provision/:id", signedurl.RequireSignedURL(signer), script.GetProvisionScript)
	app.Post(callbackPrefix+":id/provision", signedurl.RequireSignedURL(signer), script.StartProvisioning)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- app.Listener(listener) }()
	t.Cleanup(func() {
		require.NoError(t, app.ShutdownWithTimeout(3*time.Second))
		require.NoError(t, <-done)
	})
	client := &http.Client{Timeout: 3 * time.Second}

	request := func(method, path string, status int) (string, http.Header) {
		t.Helper()
		req, err := http.NewRequest(method, "http://"+listener.Addr().String()+path, nil)
		require.NoError(t, err)
		response, err := client.Do(req)
		require.NoError(t, err)
		body, err := io.ReadAll(response.Body)
		require.NoError(t, response.Body.Close())
		require.NoError(t, err)
		require.Equal(t, status, response.StatusCode, "%s %s: %s", method, path, body)
		return string(body), response.Header
	}
	request("POST", "/servers/invalid/provision-command", http.StatusBadRequest)
	request("POST", "/servers/"+missingID+"/provision-command", http.StatusNotFound)
	body, header := request("POST", serverPath+"/provision-command", http.StatusOK)
	require.Contains(t, body, "start=1")
	require.Equal(t, "no-store", header.Get("Cache-Control"))
	body, _ = request("GET", serverPath+"/provision-script-content", http.StatusOK)
	require.NotContains(t, body, "--post-data")
	legacyURL := signer.PermanentSignedURL(downloadPath, nil)
	request("GET", legacyURL+"&start=1", http.StatusForbidden)
	body, header = request("GET", signer.TemporarySignedURL(downloadPath, map[string]string{"start": "1"}, time.Hour), http.StatusOK)
	require.Contains(t, body, "--post-data")
	require.Equal(t, "no-store", header.Get("Cache-Control"))

	callback := signer.TemporarySignedURL(callbackPrefix+id+"/provision", nil, time.Hour)
	request("POST", callbackPrefix+id+"/provision", http.StatusForbidden)
	request("POST", signer.TemporarySignedURL(callbackPrefix+"invalid/provision", nil, time.Hour), http.StatusBadRequest)
	body, _ = request("POST", callback, http.StatusUnprocessableEntity)
	require.Contains(t, body, "Could not start provisioning")
	require.NotContains(t, body, "IP address")
	require.NoError(t, db.Model(&server).Updates(map[string]any{"status": types.ServerStatusStarting, "connected": true}).Error)
	request("POST", callback, http.StatusNoContent)
}
