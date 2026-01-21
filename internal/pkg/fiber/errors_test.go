package fiber

import (
	"context"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	apperrors "github.com/kkz6/launch-go/internal/pkg/errors"
)

func TestHandleServiceError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		wantStatusCode int
	}{
		{
			name:           "nil error returns nil",
			err:            nil,
			wantStatusCode: 0, // no response sent
		},
		{
			name:           "ResourceError not found",
			err:            apperrors.ErrServerNotFound,
			wantStatusCode: 404,
		},
		{
			name:           "ResourceError bad request",
			err:            apperrors.BadRequest("invalid input"),
			wantStatusCode: 400,
		},
		{
			name:           "ResourceError conflict",
			err:            apperrors.Conflict("resource already exists"),
			wantStatusCode: 409,
		},
		{
			name:           "ResourceError forbidden",
			err:            apperrors.Forbidden("access denied"),
			wantStatusCode: 403,
		},
		{
			name:           "ResourceError unauthorized",
			err:            apperrors.Unauthorized("not authenticated"),
			wantStatusCode: 401,
		},
		{
			name:           "ResourceError internal",
			err:            apperrors.Internal("something went wrong"),
			wantStatusCode: 500,
		},
		{
			name:           "GORM record not found",
			err:            gorm.ErrRecordNotFound,
			wantStatusCode: 404,
		},
		{
			name:           "context canceled",
			err:            context.Canceled,
			wantStatusCode: 408,
		},
		{
			name:           "context deadline exceeded",
			err:            context.DeadlineExceeded,
			wantStatusCode: 504,
		},
		{
			name:           "base ErrNotFound",
			err:            apperrors.ErrNotFound,
			wantStatusCode: 404,
		},
		{
			name:           "base ErrUnauthorized",
			err:            apperrors.ErrUnauthorized,
			wantStatusCode: 401,
		},
		{
			name:           "base ErrForbidden",
			err:            apperrors.ErrForbidden,
			wantStatusCode: 403,
		},
		{
			name:           "unknown error returns 500",
			err:            io.EOF, // arbitrary error
			wantStatusCode: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()

			app.Get("/test", func(c *fiber.Ctx) error {
				return HandleServiceError(c, tt.err)
			})

			req := httptest.NewRequest("GET", "/test", nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test failed: %v", err)
			}

			if tt.err == nil {
				// nil error should return 200 (default)
				if resp.StatusCode != 200 {
					t.Errorf("expected 200 for nil error, got %d", resp.StatusCode)
				}
			} else {
				if resp.StatusCode != tt.wantStatusCode {
					t.Errorf("expected status %d, got %d", tt.wantStatusCode, resp.StatusCode)
				}
			}
		})
	}
}

func TestHandleServiceErrorWithMessage(t *testing.T) {
	app := fiber.New()

	customMsg := "Custom internal error message"

	app.Get("/test", func(c *fiber.Ctx) error {
		// Use an unknown error type to trigger the default case
		return HandleServiceErrorWithMessage(c, io.EOF, customMsg)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}

	if resp.StatusCode != 500 {
		t.Errorf("expected status 500, got %d", resp.StatusCode)
	}
}

func TestMustSucceed(t *testing.T) {
	app := fiber.New()

	app.Get("/success", func(c *fiber.Ctx) error {
		if err := MustSucceed(c, nil); err != nil {
			return err
		}
		return c.SendString("ok")
	})

	app.Get("/error", func(c *fiber.Ctx) error {
		if err := MustSucceed(c, apperrors.ErrNotFound); err != nil {
			return err
		}
		return c.SendString("ok")
	})

	// Test success case
	req := httptest.NewRequest("GET", "/success", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 for nil error, got %d", resp.StatusCode)
	}

	// Test error case
	req = httptest.NewRequest("GET", "/error", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != 404 {
		t.Errorf("expected 404 for ErrNotFound, got %d", resp.StatusCode)
	}
}
