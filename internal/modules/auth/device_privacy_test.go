package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	sentryfiber "github.com/getsentry/sentry-go/fiber"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/middleware"
	"github.com/kkz6/launch-go/internal/modules/auth/services"
	appmodule "github.com/kkz6/launch-go/internal/pkg/app"
)

type devicePrivacyTransport struct {
	mu     sync.Mutex
	events []*sentry.Event
}

func (*devicePrivacyTransport) Configure(sentry.ClientOptions) {}
func (t *devicePrivacyTransport) SendEvent(event *sentry.Event) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, event)
}
func (*devicePrivacyTransport) Flush(time.Duration) bool              { return true }
func (*devicePrivacyTransport) FlushWithContext(context.Context) bool { return true }
func (*devicePrivacyTransport) Close()                                {}

func TestDeviceRoutesScrubSentryBodiesBeforeAuthenticationAndValidation(t *testing.T) {
	const jwtSecret = "device-privacy-test"
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user-1", "type": "access", "exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte(jwtSecret))
	require.NoError(t, err)
	deviceCode := strings.Repeat("a1", 32)
	for _, tt := range []struct {
		name, path, body, token string
		status                  int
	}{
		{"issuance error", "/auth/device/code", `{"device_code":"` + deviceCode + `","user_code":"87654321","code":"private-recovery-code"}`, "", 500},
		{"poll error", "/auth/device/token", `{"device_code":"` + deviceCode + `"}`, "", 500},
		{"poll validation", "/auth/device/token", `{"device_code":"invalid-private-device-code"}`, "", 422},
		{"approval authentication", "/auth/device/authorize", `{"user_code":"87654321","approve":true,"code":"private-recovery-code"}`, "", 401},
		{"approval validation", "/auth/device/authorize", `{"user_code":"invalid-private-user-code","approve":true,"code":"private-recovery-code"}`, accessToken, 422},
		{"unrelated route", "/ordinary", `{"name":"preserved"}`, "", 204},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Missing tables deliberately fail valid requests after parsing, so
			// the real error handler captures the request's Sentry scope.
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
			module := &Module{
				Base: appmodule.NewBase(ModuleName, appmodule.NewBuilder(appmodule.Deps{
					DB: db, Config: &config.Config{JWT: config.JWTConfig{Secret: jwtSecret}, App: config.AppConfig{FrontendURL: "https://launchctl.example"}},
				})),
				service: &services.Service{},
			}
			transport := &devicePrivacyTransport{}
			client, err := sentry.NewClient(sentry.ClientOptions{Dsn: "https://public@example.test/1", Transport: transport})
			require.NoError(t, err)
			app := fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler})
			app.Use(func(c *fiber.Ctx) error {
				sentryfiber.SetHubOnContext(c, sentry.NewHub(client, sentry.NewScope()))
				return c.Next()
			})
			app.Use(sentryfiber.New(sentryfiber.Options{}))
			app.Use(func(c *fiber.Ctx) error {
				err := c.Next()
				// Capture client rejections too, to prove the scope is safe before
				// validation/authentication, not only inside successful handlers.
				sentryfiber.GetHubFromContext(c).CaptureMessage("request completed")
				return err
			})
			module.registerDeviceRoutes(app.Group("/auth"), middleware.Auth(jwtSecret, db))
			app.Post("/ordinary", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusNoContent) })
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			response, err := app.Test(req)
			require.NoError(t, err)
			defer response.Body.Close()
			require.Equal(t, tt.status, response.StatusCode)
			transport.mu.Lock()
			defer transport.mu.Unlock()
			require.NotEmpty(t, transport.events)
			for _, event := range transport.events {
				require.NotNil(t, event.Request)
				require.Contains(t, event.Request.URL, tt.path)
				if tt.path == "/ordinary" {
					require.JSONEq(t, tt.body, event.Request.Data)
					continue
				}
				require.Equal(t, "no-store", response.Header.Get("Cache-Control"))
				require.Empty(t, event.Request.Data)
				encoded, err := json.Marshal(event)
				require.NoError(t, err)
				for _, secret := range []string{deviceCode, "87654321", "private-recovery-code", "invalid-private-device-code", "invalid-private-user-code", accessToken} {
					require.NotContains(t, string(encoded), secret)
				}
			}
		})
	}
}
