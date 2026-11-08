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
	"github.com/kkz6/launch-go/internal/modules/staff/handlers"
	"github.com/kkz6/launch-go/internal/modules/staff/repositories"
	"github.com/kkz6/launch-go/internal/modules/staff/services"
	stafftypes "github.com/kkz6/launch-go/internal/modules/staff/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
)

func staffRoleHTTPFixture(t *testing.T) (*fiber.App, *services.Service) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	connection, err := db.DB()
	require.NoError(t, err)
	connection.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, connection.Close()) })
	require.NoError(t, db.Exec(`CREATE TABLE users (id VARCHAR(26) PRIMARY KEY, staff_role VARCHAR(20), status VARCHAR(20), updated_at TIMESTAMP)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE teams (id VARCHAR(26) PRIMARY KEY, user_id VARCHAR(26))`).Error)
	require.NoError(t, db.AutoMigrate(&activity.ActivityLog{}))
	require.NoError(t, db.Exec(`INSERT INTO users (id,staff_role,status) VALUES ('admin','super_admin','active'),('target','support','active'),('regular',NULL,'active')`).Error)
	svc := services.NewService(repositories.NewRegistry(db))
	middleware.InitStaffMiddleware(func(id string) *stafftypes.StaffRole { return svc.StaffRoleForUser(context.Background(), id) })
	t.Cleanup(func() { middleware.InitStaffMiddleware(nil) })
	auth := func(c *fiber.Ctx) error {
		id := c.Get("X-Test-User")
		if id == "" {
			return fiberutil.Unauthorized()
		}
		c.Locals(fiberutil.KeyUserID, id)
		return c.Next()
	}
	app := fiber.New(fiber.Config{ErrorHandler: fiberutil.NewErrorHandler()})
	module := &Module{db: db, service: svc, handler: handlers.NewAdminHandler(svc)}
	module.RegisterRoutes(app, auth)
	app.Get("/staff-probe", auth, middleware.RequireStaff(stafftypes.StaffRoleSupport), func(c *fiber.Ctx) error { return c.SendStatus(http.StatusOK) })
	return app, svc
}

func TestAdminStaffRoleEndpointEnforcesRoleAndExplicitNullableField(t *testing.T) {
	for _, tc := range []struct {
		name, actor, target, body string
		status                    int
	}{
		{"anonymous", "", "target", `{"staff_role":null}`, 401},
		{"regular actor", "regular", "target", `{"staff_role":null,"actor_id":"admin"}`, 403},
		{"support actor", "target", "regular", `{"staff_role":"super_admin"}`, 403},
		{"self demotion", "admin", "admin", `{"staff_role":null}`, 409},
		{"missing target", "admin", "unknown", `{"staff_role":null}`, 404},
		{"missing role", "admin", "target", `{}`, 422},
		{"empty role", "admin", "target", `{"staff_role":""}`, 422},
		{"invalid role", "admin", "target", `{"staff_role":"owner"}`, 422},
		{"boolean role", "admin", "target", `{"staff_role":false}`, 422},
		{"invalid json", "admin", "target", `{`, 400},
		{"regular user", "admin", "target", `{"staff_role":null}`, 200},
		{"support", "admin", "regular", `{"staff_role":"support"}`, 200},
		{"super admin", "admin", "target", `{"staff_role":"super_admin"}`, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, _ := staffRoleHTTPFixture(t)
			request := httptest.NewRequest(http.MethodPatch, "/admin/users/"+tc.target+"/staff-role", strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Test-User", tc.actor)
			response, err := app.Test(request)
			require.NoError(t, err)
			defer response.Body.Close()
			require.Equal(t, tc.status, response.StatusCode)
			if tc.status == http.StatusOK {
				var result struct {
					Data map[string]any `json:"data"`
				}
				require.NoError(t, json.NewDecoder(response.Body).Decode(&result))
				require.Equal(t, tc.target, result.Data["id"])
				require.Contains(t, result.Data, "staff_role")
				if tc.name == "regular user" {
					require.Nil(t, result.Data["staff_role"])
				}
			}
		})
	}
}

func TestAdminStaffRoleDemotionRevokesAccessOnNextRequest(t *testing.T) {
	app, svc := staffRoleHTTPFixture(t)
	probe := func(wantStatus int) {
		request := httptest.NewRequest(http.MethodGet, "/staff-probe", nil)
		request.Header.Set("X-Test-User", "target")
		response, err := app.Test(request)
		require.NoError(t, err)
		require.Equal(t, wantStatus, response.StatusCode)
		require.NoError(t, response.Body.Close())
	}
	probe(http.StatusOK)
	request := httptest.NewRequest(http.MethodPatch, "/admin/users/target/staff-role", strings.NewReader(`{"staff_role":null}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Test-User", "admin")
	response, err := app.Test(request)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.NoError(t, response.Body.Close())
	probe(http.StatusForbidden)
	allowed, _, err := svc.CanDeleteUser(context.Background(), "target")
	require.NoError(t, err)
	require.True(t, allowed)
}
