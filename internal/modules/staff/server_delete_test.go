package staff

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/middleware"
	authmodels "github.com/kkz6/launch-go/internal/modules/auth/models"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/modules/staff/handlers"
	"github.com/kkz6/launch-go/internal/modules/staff/repositories"
	"github.com/kkz6/launch-go/internal/modules/staff/services"
	stafftypes "github.com/kkz6/launch-go/internal/modules/staff/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

type adminServerDeleter struct {
	ids     []string
	err     error
	removed bool
	offline bool
}

func (d *adminServerDeleter) UninstallServer(_ context.Context, id, teamID, actorID string) (bool, error) {
	d.ids = []string{id, teamID, actorID}
	return d.offline, d.err
}

func (d *adminServerDeleter) DeleteServer(_ context.Context, id, teamID, actorID string) error {
	d.ids = []string{id, teamID, actorID}
	return d.err
}

func (d *adminServerDeleter) RemoveServerRecord(_ context.Context, id, teamID, actorID string) error {
	d.ids = []string{id, teamID, actorID}
	d.removed = true
	return d.err
}

func adminServerDeleteFixture(t *testing.T) (*fiber.App, *gorm.DB, *adminServerDeleter) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true, Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&servermodels.Server{}, &authmodels.User{}, &authmodels.Team{}))
	require.NoError(t, db.Exec("CREATE TABLE sites (id text PRIMARY KEY, server_id text)").Error)
	require.NoError(t, db.Exec("CREATE TABLE docker_projects (id text PRIMARY KEY, server_id text, deleted_at timestamp)").Error)
	server := servermodels.Server{BaseModel: basemodels.BaseModel{ID: "customer-server"}, TeamScoped: basemodels.TeamScoped{TeamID: "customer-team"}, Name: "Production", Provider: servertypes.ProviderCustom, Status: servertypes.ServerStatusRunning}
	require.NoError(t, db.Create(&server).Error)
	svc := services.NewService(repositories.NewRegistry(db))
	deleter := &adminServerDeleter{}
	svc.SetServerDeleter(deleter)
	m := &Module{db: db, service: svc, handler: handlers.NewAdminHandler(svc)}
	middleware.InitStaffMiddleware(func(id string) *stafftypes.StaffRole {
		var role stafftypes.StaffRole
		switch id {
		case "admin":
			role = stafftypes.StaffRoleSuperAdmin
		case "support":
			role = stafftypes.StaffRoleSupport
		default:
			return nil
		}
		return &role
	})
	t.Cleanup(func() { middleware.InitStaffMiddleware(nil) })
	app := fiber.New(fiber.Config{ErrorHandler: fiberutil.NewErrorHandler()})
	m.RegisterRoutes(app, func(c *fiber.Ctx) error {
		id := c.Get("X-Test-User")
		if id == "" {
			return fiberutil.Unauthorized()
		}
		c.Locals(fiberutil.KeyUserID, id)
		c.Locals(fiberutil.KeyTeamID, "staff-team")
		return c.Next()
	})
	return app, db, deleter
}

func TestAdminServerDeleteEnforcesRoleNameAndOwningTeam(t *testing.T) {
	for _, tc := range []struct {
		name, user, path, body string
		status                 int
		forwarded              bool
	}{
		{"anonymous", "", "customer-server", `{"confirmation_name":"Production"}`, 401, false},
		{"customer", "customer", "customer-server", `{"confirmation_name":"Production"}`, 403, false},
		{"support", "support", "customer-server", `{"confirmation_name":"Production"}`, 403, false},
		{"wrong name", "admin", "customer-server", `{"confirmation_name":"production"}`, 422, false},
		{"missing name", "admin", "customer-server", `{}`, 422, false},
		{"bad json", "admin", "customer-server", `{`, 400, false},
		{"missing server", "admin", "unknown", `{"confirmation_name":"Production"}`, 404, false},
		{"invalid mode", "admin", "customer-server", `{"confirmation_name":"Production","mode":"force"}`, 422, false},
		{"old route rejects removal mode", "admin", "customer-server", `{"confirmation_name":"Production","mode":"remove_record"}`, 422, false},
		{"old route rejects explicit delete mode", "admin", "customer-server", `{"confirmation_name":"Production","mode":"delete"}`, 422, false},
		{"anonymous local removal", "", "customer-server/record", `{"confirmation_name":"Production"}`, 401, false},
		{"support local removal", "support", "customer-server/record", `{"confirmation_name":"Production"}`, 403, false},
		{"customer local removal", "customer", "customer-server/record", `{"confirmation_name":"Production"}`, 403, false},
		{"local removal wrong name", "admin", "customer-server/record", `{"confirmation_name":"production"}`, 422, false},
		{"local removal missing name", "admin", "customer-server/record", `{}`, 422, false},
		{"local removal missing server", "admin", "unknown/record", `{"confirmation_name":"Production"}`, 404, false},
		{"local removal bad json", "admin", "customer-server/record", `{`, 400, false},
		{"confirmed", "admin", "customer-server", `{"confirmation_name":"Production","team_id":"attacker-team"}`, 200, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, _, deleter := adminServerDeleteFixture(t)
			req := httptest.NewRequest(http.MethodDelete, "/admin/servers/"+tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Test-User", tc.user)
			response, err := app.Test(req)
			require.NoError(t, err)
			defer response.Body.Close()
			require.Equal(t, tc.status, response.StatusCode)
			if tc.forwarded {
				require.False(t, deleter.removed, "default deletion must keep its provider semantics")
				require.Equal(t, []string{"customer-server", "customer-team", "admin"}, deleter.ids)
				var body struct {
					Data services.ServerDeletionResult `json:"data"`
				}
				require.NoError(t, json.NewDecoder(response.Body).Decode(&body))
				require.Equal(t, "deleted", body.Data.Status)
			} else {
				require.Empty(t, deleter.ids)
			}
		})
	}
}

func TestAdminServerRecordRemovalIsExplicitAndUsesOwningTeam(t *testing.T) {
	app, db, deleter := adminServerDeleteFixture(t)
	require.NoError(t, db.Model(&servermodels.Server{}).Where("id = ?", "customer-server").Update("provider", servertypes.ProviderDigitalOcean).Error)
	req := httptest.NewRequest(http.MethodDelete, "/admin/servers/customer-server/record", strings.NewReader(`{"confirmation_name":"Production","team_id":"attacker-team"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User", "admin")
	response, err := app.Test(req)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, 200, response.StatusCode)
	require.True(t, deleter.removed)
	require.Equal(t, []string{"customer-server", "customer-team", "admin"}, deleter.ids)
	var body struct {
		Data services.ServerDeletionResult `json:"data"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&body))
	require.Equal(t, "removed", body.Data.Status)
}

func TestAdminServerDeleteReportsQueueFailureAndCloudReceipt(t *testing.T) {
	for _, tc := range []struct {
		name string
		fail bool
	}{{"queue failure", true}, {"cloud receipt", false}} {
		t.Run(tc.name, func(t *testing.T) {
			app, db, deleter := adminServerDeleteFixture(t)
			require.NoError(t, db.Model(&servermodels.Server{}).Where("id = ?", "customer-server").Update("provider", servertypes.ProviderHetzner).Error)
			if tc.fail {
				deleter.err = fiberutil.Internal("Queue unavailable")
			}
			req := httptest.NewRequest(http.MethodDelete, "/admin/servers/customer-server", strings.NewReader(`{"confirmation_name":"Production"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Test-User", "admin")
			response, err := app.Test(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, response.Body.Close()) }()
			if tc.fail {
				require.Equal(t, 500, response.StatusCode)
				return
			}
			require.Equal(t, 200, response.StatusCode)
			var body struct {
				Data services.ServerDeletionResult `json:"data"`
			}
			require.NoError(t, json.NewDecoder(response.Body).Decode(&body))
			require.Equal(t, "deletion_requested", body.Data.Status)
		})
	}
}

func TestAdminServerUninstallReportsOfflineRemoval(t *testing.T) {
	app, _, deleter := adminServerDeleteFixture(t)
	deleter.offline = true
	for user, status := range map[string]int{"support": 403, "admin": 200} {
		req := httptest.NewRequest(http.MethodDelete, "/admin/servers/customer-server/uninstall", strings.NewReader(`{"confirmation_name":"Production"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-User", user)
		response, err := app.Test(req)
		require.NoError(t, err)
		defer response.Body.Close()
		require.Equal(t, status, response.StatusCode)
		if status == 200 {
			var body struct {
				Data services.ServerDeletionResult `json:"data"`
			}
			require.NoError(t, json.NewDecoder(response.Body).Decode(&body))
			require.Equal(t, "removed_offline", body.Data.Status)
			require.Equal(t, []string{"customer-server", "customer-team", "admin"}, deleter.ids)
		}
	}
}

func TestAdminServerDetailIncludesDeletionBlockers(t *testing.T) {
	app, db, _ := adminServerDeleteFixture(t)
	require.NoError(t, db.Exec("INSERT INTO sites VALUES ('site', 'customer-server')").Error)
	require.NoError(t, db.Exec("INSERT INTO docker_projects VALUES ('live', 'customer-server', NULL), ('old', 'customer-server', CURRENT_TIMESTAMP)").Error)
	req := httptest.NewRequest(http.MethodGet, "/admin/servers/customer-server", nil)
	req.Header.Set("X-Test-User", "support")
	response, err := app.Test(req)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, 200, response.StatusCode)
	var body struct {
		Data struct {
			SitesCount    int `json:"sites_count"`
			ProjectsCount int `json:"projects_count"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&body))
	require.Equal(t, 1, body.Data.SitesCount)
	require.Equal(t, 1, body.Data.ProjectsCount)
}
