package server

import (
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
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/policies"
	"github.com/kkz6/launch-go/internal/modules/server/services"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/access"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	pkgservice "github.com/kkz6/launch-go/internal/pkg/service"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

func TestSystemUpdatesRoutesEnforceAuthenticationRoleScopeAndProvisioning(t *testing.T) {
	const serverID = "01HJXVHGRGTQRX4P0G3Y8R6CK7"
	const path = "/servers/" + serverID + "/system-updates"
	const body = `{"enabled":true,"schedule":"04:15","notifications":true}`
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true, Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	conn, err := db.DB()
	require.NoError(t, err)
	conn.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	require.NoError(t, db.AutoMigrate(&models.Server{}, &models.InstalledService{}, &models.SystemUpdateMonitor{}, &models.ServerRestart{}))
	ip := "192.0.2.6"
	server := models.Server{
		BaseModel: basemodels.BaseModel{ID: serverID}, TeamScoped: basemodels.TeamScoped{TeamID: "team-a"},
		Status: types.ServerStatusRunning, PublicIPv4: &ip, PrivateKey: dbtype.EncryptedString("fixture-key"),
	}
	require.NoError(t, db.Create(&server).Error)
	gate := access.New()
	policies.Register(gate)
	middleware.InitGate(gate)
	middleware.InitServerProvisionedMiddleware(db)
	t.Cleanup(func() {
		middleware.InitGate(nil)
		middleware.InitServerProvisionedMiddleware(nil)
	})
	log := zerolog.Nop()
	dispatcher := taskrunner.NewFakeDispatcher()
	dispatcher.DefaultResult.Output = `{"supported":true,"enabled":true,"schedule":"04:15","notifications":true,"timer_active":true}`
	module := NewModule(app.NewBuilder(app.Deps{DB: db, Logger: &log}))
	module.service = services.NewService(services.ServiceDeps{
		Dependencies: pkgservice.Dependencies{DB: db, Logger: &log}, Repos: module.repos,
		TaskRunnerDeps: &tasks.TaskRunnerDeps{Dispatcher: dispatcher},
	})
	router := fiber.New(fiber.Config{ErrorHandler: fiberutil.NewErrorHandler()})
	module.RegisterRoutes(router, func(c *fiber.Ctx) error {
		if c.Get("Authorization") != "test-session" {
			return fiberutil.Unauthorized()
		}
		c.Locals(fiberutil.KeyUserID, "user-a")
		fiberutil.SetTeamContext(c, c.Get("X-Team-ID"), c.Get("X-Test-Role"))
		return c.Next()
	}, nil)

	for _, tc := range []struct {
		name, method, role, team, status, body string
		want                                   int
		remote                                 bool
		suffix                                 string
	}{
		{"anonymous read", "GET", "anonymous", "team-a", "running", "", fiber.StatusUnauthorized, false, ""},
		{"anonymous write", "PUT", "anonymous", "team-a", "running", body, fiber.StatusUnauthorized, false, ""},
		{"member can read", "GET", "member", "team-a", "running", "", fiber.StatusOK, true, ""},
		{"member cannot save", "PUT", "member", "team-a", "running", body, fiber.StatusForbidden, false, ""},
		{"editor can save", "PUT", "editor", "team-a", "running", body, fiber.StatusOK, true, ""},
		{"other team read", "GET", "owner", "team-b", "running", "", fiber.StatusForbidden, false, ""},
		{"other team write", "PUT", "owner", "team-b", "running", body, fiber.StatusForbidden, false, ""},
		{"provisioning read", "GET", "owner", "team-a", "provisioning", "", fiber.StatusForbidden, false, ""},
		{"provisioning write", "PUT", "owner", "team-a", "provisioning", body, fiber.StatusForbidden, false, ""},
		{"missing schedule", "PUT", "editor", "team-a", "running", `{"enabled":true}`, fiber.StatusUnprocessableEntity, false, ""},
		{"malformed request", "PUT", "editor", "team-a", "running", `{`, fiber.StatusBadRequest, false, ""},
		{"invalid schedule", "PUT", "editor", "team-a", "running", `{"schedule":"24:00"}`, fiber.StatusBadRequest, false, ""},
		{"member can view logs", "GET", "member", "team-a", "running", "", fiber.StatusOK, true, "/logs"},
		{"other team cannot view logs", "GET", "owner", "team-b", "running", "", fiber.StatusForbidden, false, "/logs"},
		{"member cannot retry", "POST", "member", "team-a", "running", "", fiber.StatusForbidden, false, "/retry"},
		{"member cannot repair", "POST", "member", "team-a", "running", "", fiber.StatusForbidden, false, "/repair"},
		{"editor can retry", "POST", "editor", "team-a", "running", "", fiber.StatusOK, true, "/retry"},
		{"editor can repair", "POST", "editor", "team-a", "running", "", fiber.StatusOK, true, "/repair"},
		{"other team cannot retry", "POST", "owner", "team-b", "running", "", fiber.StatusForbidden, false, "/retry"},
		{"other team cannot repair", "POST", "owner", "team-b", "running", "", fiber.StatusForbidden, false, "/repair"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, db.Model(&server).Update("status", tc.status).Error)
			before := dispatcher.ExecutionCount()
			var request fasthttp.Request
			request.Header.SetMethod(tc.method)
			request.SetRequestURI(path + tc.suffix)
			request.SetBodyString(tc.body)
			request.Header.SetContentType("application/json")
			request.Header.Set("X-Team-ID", tc.team)
			request.Header.Set("X-Test-Role", tc.role)
			if tc.role != "anonymous" {
				request.Header.Set("Authorization", "test-session")
			}
			// Keep a live fasthttp context, as used by the production handler.
			var ctx fasthttp.RequestCtx
			ctx.Init(&request, nil, nil)
			router.Handler()(&ctx)
			require.Equal(t, tc.want, ctx.Response.StatusCode(), string(ctx.Response.Body()))
			wantCalls := 0
			if tc.remote {
				wantCalls = 1
			}
			require.Equal(t, wantCalls, dispatcher.ExecutionCount()-before)
		})
	}
	// Restart progress is a local read and remains available while the host is offline.
	for _, tc := range []struct {
		name, method, role, team, suffix string
		want                             int
	}{
		{"restart anonymous read", "GET", "anonymous", "team-a", "", fiber.StatusUnauthorized},
		{"restart member read", "GET", "member", "team-a", "", fiber.StatusOK},
		{"restart other team read", "GET", "admin", "team-b", "", fiber.StatusNotFound},
		{"restart editor blocked", "POST", "editor", "team-a", "", fiber.StatusForbidden},
		{"restart member verify blocked", "POST", "member", "team-a", "/verify", fiber.StatusForbidden},
		{"restart other team verify blocked", "POST", "admin", "team-b", "/verify", fiber.StatusNotFound},
		{"restart cannot verify without baseline", "POST", "admin", "team-a", "/verify", fiber.StatusConflict},
		{"restart admin allowed", "POST", "admin", "team-a", "", fiber.StatusOK},
		{"restart duplicate blocked", "POST", "admin", "team-a", "", fiber.StatusConflict},
		{"restart member reads queued progress", "GET", "member", "team-a", "", fiber.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, db.Model(&server).Updates(map[string]any{"status": "running", "connected": false}).Error)
			before := dispatcher.ExecutionCount()
			var request fasthttp.Request
			request.Header.SetMethod(tc.method)
			request.SetRequestURI("/servers/" + serverID + "/reboot" + tc.suffix)
			request.Header.Set("X-Team-ID", tc.team)
			request.Header.Set("X-Test-Role", tc.role)
			if tc.role != "anonymous" {
				request.Header.Set("Authorization", "test-session")
			}
			var ctx fasthttp.RequestCtx
			ctx.Init(&request, nil, nil)
			router.Handler()(&ctx)
			require.Equal(t, tc.want, ctx.Response.StatusCode(), string(ctx.Response.Body()))
			require.Equal(t, before, dispatcher.ExecutionCount())
		})
	}
}
