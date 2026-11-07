package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/auth/models"
	"github.com/kkz6/launch-go/internal/modules/auth/repositories"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	appmodule "github.com/kkz6/launch-go/internal/pkg/app"
	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

func TestDeviceRoutesLeaveCodeAndPollingPublicButRequireApprovalAuthentication(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&models.DeviceAuthorization{}, &models.DeviceAuthorizationLimit{}))
	module := &Module{
		Base: appmodule.NewBase(ModuleName, appmodule.NewBuilder(appmodule.Deps{
			DB: db, Config: &config.Config{JWT: config.JWTConfig{Secret: "device-test"}, App: config.AppConfig{FrontendURL: "https://launchctl.example"}},
		})),
		service: &services.Service{},
		repos:   repositories.NewRegistry(db),
	}
	app := fiber.New(fiber.Config{ErrorHandler: fiberctx.NewErrorHandler()})
	module.RegisterPublicRoutes(app)
	for _, test := range []struct {
		path string
		body string
		want int
	}{
		{"/auth/device/code", `{}`, http.StatusOK},
		{"/auth/device/token", `{"device_code":"` + strings.Repeat("a", 64) + `"}`, http.StatusOK},
		{"/auth/device/authorize", `{"user_code":"12345678","approve":true}`, http.StatusUnauthorized},
	} {
		t.Run(test.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
			req.Header.Set("Content-Type", "application/json")
			response, err := app.Test(req)
			require.NoError(t, err)
			defer response.Body.Close()
			require.Equal(t, test.want, response.StatusCode)
		})
	}
}
