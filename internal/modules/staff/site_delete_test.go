package staff

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/middleware"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	sitemodels "github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/staff/dto"
	"github.com/kkz6/launch-go/internal/modules/staff/handlers"
	"github.com/kkz6/launch-go/internal/modules/staff/repositories"
	"github.com/kkz6/launch-go/internal/modules/staff/services"
	stafftypes "github.com/kkz6/launch-go/internal/modules/staff/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

type adminSiteDeleter struct {
	ids []string
	err error
}

func (d *adminSiteDeleter) DeleteSite(_ context.Context, id, serverID, teamID, actorID string) error {
	d.ids = []string{id, serverID, teamID, actorID}
	return d.err
}

func adminSiteDeleteFixture(t *testing.T) (*fiber.App, *gorm.DB, *adminSiteDeleter) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true, Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&servermodels.Server{}, &sitemodels.Site{}))
	server := servermodels.Server{BaseModel: basemodels.BaseModel{ID: "customer-server"}, TeamScoped: basemodels.TeamScoped{TeamID: "customer-team"}}
	require.NoError(t, db.Create(&server).Error)
	site := sitemodels.Site{BaseModel: basemodels.BaseModel{ID: "customer-site"}, ServerScoped: basemodels.ServerScoped{ServerID: server.ID}, TeamScoped: basemodels.TeamScoped{TeamID: server.TeamID}, Address: "customer.example"}
	require.NoError(t, db.Create(&site).Error)
	svc := services.NewService(repositories.NewRegistry(db))
	deleter := &adminSiteDeleter{}
	svc.SetSiteDeleter(deleter)
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

func TestAdminSiteDeleteEnforcesPermissionConfirmationAndOwnership(t *testing.T) {
	for _, tc := range []struct {
		name, user, path, body string
		status                 int
	}{
		{"anonymous", "", "customer-site", `{"confirmation_domain":"customer.example"}`, 401},
		{"customer", "customer", "customer-site", `{"confirmation_domain":"customer.example"}`, 403},
		{"support", "support", "customer-site", `{"confirmation_domain":"customer.example"}`, 403},
		{"incorrect domain", "admin", "customer-site", `{"confirmation_domain":"Customer.example"}`, 422},
		{"missing domain", "admin", "customer-site", `{}`, 422},
		{"malformed body", "admin", "customer-site", `{`, 400},
		{"unknown site", "admin", "missing", `{"confirmation_domain":"customer.example"}`, 404},
		{"confirmed", "admin", "customer-site", `{"confirmation_domain":"customer.example","server_id":"other-server","team_id":"other-team","user_id":"other-user"}`, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, _, deleter := adminSiteDeleteFixture(t)
			req := httptest.NewRequest(http.MethodDelete, "/admin/sites/"+tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Test-User", tc.user)
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()
			require.Equal(t, tc.status, resp.StatusCode)
			if tc.status != 200 {
				require.Empty(t, deleter.ids)
				return
			}
			require.Equal(t, []string{"customer-site", "customer-server", "customer-team", "admin"}, deleter.ids)
			var body struct {
				Data services.SiteDeletionResult `json:"data"`
			}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
			require.Equal(t, services.SiteDeletionResult{SiteID: "customer-site", Status: "deletion_requested"}, body.Data)
		})
	}
}

func TestAdminSiteDeleteRejectsBrokenOwnershipAndQueueFailure(t *testing.T) {
	for _, tc := range []struct {
		name   string
		setup  string
		err    error
		status int
	}{
		{"server team mismatch", "UPDATE servers SET team_id = 'other-team'", nil, 404},
		{"missing server", "DELETE FROM servers", nil, 404},
		{"queue unavailable", "", fiberutil.Internal("Unable to queue site deletion"), 500},
		{"already pending", "", fiberutil.Conflict("Site deletion is already in progress"), 409},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, db, deleter := adminSiteDeleteFixture(t)
			if tc.setup != "" {
				require.NoError(t, db.Exec(tc.setup).Error)
			}
			deleter.err = tc.err
			req := httptest.NewRequest(http.MethodDelete, "/admin/sites/customer-site", strings.NewReader(`{"confirmation_domain":"customer.example"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Test-User", "admin")
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()
			require.Equal(t, tc.status, resp.StatusCode)
			if tc.setup != "" {
				require.Empty(t, deleter.ids)
			}
			require.NoError(t, db.First(&sitemodels.Site{}, "id = ?", "customer-site").Error)
		})
	}
}

func TestAdminSiteSummaryIncludesDeletionStateWithoutSecrets(t *testing.T) {
	now := time.Now().UTC()
	secret := "private-deploy-token"
	site := sitemodels.Site{Address: "customer.example", DeployToken: &secret}
	site.UninstallationRequestedAt = &now
	site.UninstallationFailedAt = &now
	data, err := json.Marshal(dto.NewSiteSummary(site, nil))
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(data, &fields))
	require.NotNil(t, fields["uninstallation_requested_at"])
	require.NotNil(t, fields["uninstallation_failed_at"])
	require.NotContains(t, string(data), secret)
	require.NotContains(t, fields, "deploy_token")
}
