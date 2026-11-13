package docker

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/database/serializers"
	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/modules/docker/policies"
	"github.com/kkz6/launch-go/internal/pkg/access"
	"github.com/kkz6/launch-go/internal/pkg/app"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
)

const routeServerID = "01HJXVHGRGTQRX4P0G3Y8R6CK7"
const routeProjectPath = "/api/servers/" + routeServerID + "/docker/projects/project-a"

func setupDockerRoutes(t *testing.T) (*fiber.App, *gorm.DB) {
	t.Helper()
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(
		&models.Project{}, &models.Application{}, &models.Compose{}, &models.ProjectEnvVar{},
		&models.RegistryCredential{}, &models.ApplicationEnvVar{}, &models.ApplicationBuildSecret{},
		&models.ComposeBuildSecret{}, &models.ApplicationDomain{}, &models.ApplicationVolume{}, &models.ApplicationSchedule{},
	))
	require.NoError(t, db.Exec("CREATE TABLE servers (id TEXT PRIMARY KEY, team_id TEXT, status TEXT)").Error)
	require.NoError(t, db.Exec("INSERT INTO servers VALUES (?, ?, ?)", routeServerID, "team-a", "running").Error)
	for _, id := range []string{"project-a", "project-b"} {
		project := models.Project{Name: id}
		project.ID, project.TeamID, project.ServerID = id, "team-a", routeServerID
		require.NoError(t, db.Create(&project).Error)
	}
	application := models.Application{Name: "app", ProjectID: "project-a", SourceType: "image"}
	application.ID, application.TeamID, application.ServerID = "app-a", "team-a", routeServerID
	require.NoError(t, db.Create(&application).Error)
	compose := models.Compose{Name: "stack", ProjectID: "project-a", ComposeSourceType: "raw"}
	compose.ID, compose.TeamID, compose.ServerID = "compose-a", "team-a", routeServerID
	require.NoError(t, db.Create(&compose).Error)

	gate := access.New()
	policies.Register(gate)
	middleware.InitGate(gate)
	middleware.InitServerProvisionedMiddleware(db)
	t.Cleanup(func() {
		middleware.InitGate(nil)
		middleware.InitServerProvisionedMiddleware(nil)
	})
	auth := func(c *fiber.Ctx) error {
		if c.Get("Authorization") != "test-session" {
			return fiberutil.Unauthorized("Authentication required")
		}
		c.Locals(fiberutil.KeyUserID, "user-a")
		fiberutil.SetTeamContext(c, c.Get("X-Team-ID"), c.Get("X-Test-Role"))
		return c.Next()
	}
	log := zerolog.Nop()
	module := NewModule(app.NewBuilder(app.Deps{DB: db, Logger: &log}))
	router := fiber.New(fiber.Config{ErrorHandler: fiberutil.NewErrorHandler()})
	module.RegisterRoutes(router.Group("/api"), auth)
	return router, db
}

func dockerRequest(t *testing.T, app *fiber.App, method, path, body, role string) (int, map[string]any) {
	t.Helper()
	var req fasthttp.Request
	req.Header.SetMethod(method)
	req.SetRequestURI(path)
	req.SetBodyString(body)
	req.Header.SetContentType("application/json")
	req.Header.Set("X-Team-ID", "team-a")
	req.Header.Set("X-Test-Role", role)
	if role != "anonymous" {
		req.Header.Set("Authorization", "test-session")
	}
	// Init supplies a live request context without starting a server. ServeConn
	// alone leaves this fasthttp version's server context canceled.
	var ctx fasthttp.RequestCtx
	ctx.Init(&req, nil, nil)
	app.Handler()(&ctx)
	resp, err := http.ReadResponse(bufio.NewReader(strings.NewReader(ctx.Response.String())), nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	status := resp.StatusCode
	var data map[string]any
	if status == fiber.StatusNoContent {
		require.Empty(t, raw)
	} else {
		require.NoError(t, json.Unmarshal(raw, &data), string(raw))
	}
	return status, data
}

func TestDockerProjectEnvAndRegistryCredentialRoutes(t *testing.T) {
	for _, tc := range []struct {
		name, path, create, update, createdMessage, updatedMessage, listedMessage string
	}{
		{"project env", routeProjectPath + "/env-vars", `{"key":"APP_MODE","value":"secret","is_secret":true}`, `{"value":"updated"}`, "Project env var added", "Project env var updated", "Project env vars retrieved"},
		{"registry credential", "/api/registry-credentials", `{"name":"registry","username":"operator","password":"secret"}`, `{"name":"updated"}`, "Registry credential created", "Registry credential updated", "Registry credentials retrieved"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, db := setupDockerRoutes(t)
			for _, invalid := range []struct {
				role, body string
				status     int
			}{
				{"anonymous", tc.create, fiber.StatusUnauthorized},
				{"member", tc.create, fiber.StatusForbidden},
				{"owner", "{", fiber.StatusBadRequest},
				{"owner", "{}", fiber.StatusUnprocessableEntity},
			} {
				status, _ := dockerRequest(t, app, "POST", tc.path, invalid.body, invalid.role)
				require.Equal(t, invalid.status, status)
			}
			status, body := dockerRequest(t, app, "POST", tc.path, tc.create, "owner")
			require.Equal(t, fiber.StatusCreated, status)
			require.Equal(t, tc.createdMessage, body["message"])
			require.Equal(t, true, body["success"])
			data := body["data"].(map[string]any)
			id := data["id"].(string)
			require.NotEmpty(t, id)
			if tc.name == "project env" {
				require.Equal(t, "project-a", data["project_id"])
				require.Equal(t, "********", data["value"])
				wrongParent := strings.Replace(tc.path, "project-a", "project-b", 1) + "/" + id
				for _, method := range []string{"PATCH", "DELETE"} {
					status, _ := dockerRequest(t, app, method, wrongParent, tc.update, "owner")
					require.Equal(t, fiber.StatusNotFound, status)
				}
			} else {
				require.Equal(t, "team-a", data["team_id"])
				require.Equal(t, "user-a", data["user_id"])
				require.Equal(t, "operator", data["username"])
				require.Equal(t, true, data["has_password"])
				require.NotContains(t, data, "password")
			}
			for _, method := range []string{"PATCH", "DELETE"} {
				status, _ := dockerRequest(t, app, method, tc.path+"/"+id, tc.update, "member")
				require.Equal(t, fiber.StatusForbidden, status)
			}
			status, body = dockerRequest(t, app, "PATCH", tc.path+"/"+id, tc.update, "owner")
			require.Equal(t, fiber.StatusOK, status)
			require.Equal(t, tc.updatedMessage, body["message"])
			require.Equal(t, id, body["data"].(map[string]any)["id"])
			status, body = dockerRequest(t, app, "GET", tc.path, "", "member")
			require.Equal(t, fiber.StatusOK, status)
			require.Equal(t, tc.listedMessage, body["message"])
			require.Len(t, body["data"], 1)
			if tc.name == "project env" {
				var env models.ProjectEnvVar
				require.NoError(t, db.First(&env, "id = ?", id).Error)
				require.Equal(t, "updated", env.Value.String())
			} else {
				require.Equal(t, "updated", body["data"].([]any)[0].(map[string]any)["name"])
			}
			status, _ = dockerRequest(t, app, "DELETE", tc.path+"/"+id, "", "owner")
			require.Equal(t, fiber.StatusNoContent, status)
			status, body = dockerRequest(t, app, "GET", tc.path, "", "member")
			require.Equal(t, fiber.StatusOK, status)
			require.Empty(t, body["data"])
		})
	}
}

func TestDockerChildListRoutesPreserveScopeAndMessages(t *testing.T) {
	app, db := setupDockerRoutes(t)
	for _, tc := range []struct{ suffix, message string }{
		{"applications/app-a/env-vars", "Env vars retrieved"},
		{"applications/app-a/build-secrets", "Build secrets retrieved"},
		{"applications/app-a/schedules", "Schedules retrieved"},
		{"applications/app-a/domains", "Domains retrieved"},
		{"applications/app-a/volumes", "Volumes retrieved"},
		{"composes/compose-a/build-secrets", "Build secrets retrieved"},
		{"composes/compose-a/domains", "Domains retrieved"},
		{"composes/compose-a/volumes", "Volumes retrieved"},
	} {
		t.Run(tc.suffix, func(t *testing.T) {
			path := routeProjectPath + "/" + tc.suffix
			status, body := dockerRequest(t, app, "GET", path, "", "member")
			require.Equal(t, fiber.StatusOK, status)
			require.Equal(t, tc.message, body["message"])
			require.Equal(t, true, body["success"])
			require.Empty(t, body["data"])
			status, _ = dockerRequest(t, app, "GET", strings.Replace(path, "project-a", "project-b", 1), "", "member")
			require.Equal(t, fiber.StatusNotFound, status)
			status, _ = dockerRequest(t, app, "GET", path, "", "anonymous")
			require.Equal(t, fiber.StatusUnauthorized, status)
		})
	}
	require.NoError(t, db.Exec("UPDATE servers SET status = 'pending'").Error)
	status, _ := dockerRequest(t, app, "GET", routeProjectPath+"/env-vars", "", "owner")
	require.Equal(t, fiber.StatusForbidden, status)
	status, _ = dockerRequest(t, app, "GET", "/api/registry-credentials", "", "owner")
	require.Equal(t, fiber.StatusOK, status, "registry credentials must not require a provisioned server")
}
