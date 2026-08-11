package fiber

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	sentryfiber "github.com/getsentry/sentry-go/fiber"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/pkg/apperror"
)

type recordingTransport struct {
	events []*sentry.Event
}

func (*recordingTransport) Configure(sentry.ClientOptions) {}
func (t *recordingTransport) SendEvent(event *sentry.Event) {
	t.events = append(t.events, event)
}
func (*recordingTransport) Flush(time.Duration) bool              { return true }
func (*recordingTransport) FlushWithContext(context.Context) bool { return true }
func (*recordingTransport) Close()                                {}

func TestIsServerError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "plain", err: errors.New("database failed"), want: true},
		{name: "app client", err: apperror.ErrBadRequest, want: false},
		{name: "app server", err: apperror.ErrInternal, want: true},
		{name: "fiber client", err: fiber.NewError(fiber.StatusBadRequest), want: false},
		{name: "fiber server", err: fiber.NewError(fiber.StatusBadGateway), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isServerError(tt.err))
		})
	}
}

func TestHandledServerErrorsAreCaptured(t *testing.T) {
	transport := &recordingTransport{}
	client, err := sentry.NewClient(sentry.ClientOptions{
		Dsn:       "https://public@example.com/1",
		Transport: transport,
	})
	require.NoError(t, err)
	hub := sentry.NewHub(client, sentry.NewScope())

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		sentryfiber.SetHubOnContext(c, hub)
		return c.Next()
	})
	app.Get("/handled", func(c *fiber.Ctx) error {
		return HandleError(c, errors.New("teams query failed"))
	})
	app.Get("/handled-internal", func(c *fiber.Ctx) error {
		return HandleErrorOrInternal(c, errors.New("server lookup failed"), "Failed to fetch server")
	})
	app.Get("/client", func(c *fiber.Ctx) error {
		return HandleError(c, fiber.NewError(fiber.StatusBadRequest, "bad request"))
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/handled", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Len(t, transport.events, 1)

	resp, err = app.Test(httptest.NewRequest(http.MethodGet, "/handled-internal", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Len(t, transport.events, 2)

	resp, err = app.Test(httptest.NewRequest(http.MethodGet, "/client", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
	require.Len(t, transport.events, 2)
}

func TestCaptureServerErrorWithoutHub(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		CaptureServerError(c, errors.New("database failed"))
		return c.SendStatus(fiber.StatusNoContent)
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNoContent, resp.StatusCode)
}
