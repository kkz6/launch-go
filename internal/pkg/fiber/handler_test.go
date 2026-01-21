package fiber

import (
	"bytes"
	"net/http/httptest"
	"testing"

	gofiber "github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

func TestNewBaseHandler(t *testing.T) {
	logger := zerolog.Nop()
	base := NewBaseHandler(&logger)

	if base.Logger() == nil {
		t.Error("Expected logger to be set")
	}
}

func TestLogRequest(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	base := NewBaseHandler(&logger)

	app := gofiber.New()
	app.Get("/test", func(c *gofiber.Ctx) error {
		base.LogRequest(c, "test_action")
		return c.SendString("OK")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	_, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	// Verify log was written
	if buf.Len() == 0 {
		t.Error("Expected log output")
	}

	logOutput := buf.String()
	if !contains(logOutput, "test_action") {
		t.Errorf("Expected log to contain action, got: %s", logOutput)
	}
}

func TestLogError(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	base := NewBaseHandler(&logger)

	app := gofiber.New()
	app.Get("/test", func(c *gofiber.Ctx) error {
		base.LogError(c, gofiber.ErrNotFound, "test error message")
		return c.SendString("OK")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	_, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	logOutput := buf.String()
	if !contains(logOutput, "test error message") {
		t.Errorf("Expected log to contain error message, got: %s", logOutput)
	}
	if !contains(logOutput, "error") {
		t.Errorf("Expected log to contain error level, got: %s", logOutput)
	}
}

func TestGetTraceID(t *testing.T) {
	logger := zerolog.Nop()
	base := NewBaseHandler(&logger)

	tests := []struct {
		name       string
		setupLocal bool
		localValue string
		header     string
		expected   string
	}{
		{
			name:       "from locals",
			setupLocal: true,
			localValue: "trace-from-local",
			expected:   "trace-from-local",
		},
		{
			name:     "from header",
			header:   "trace-from-header",
			expected: "trace-from-header",
		},
		{
			name:     "empty when not set",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := gofiber.New()
			var result string

			app.Get("/test", func(c *gofiber.Ctx) error {
				if tt.setupLocal {
					c.Locals("traceID", tt.localValue)
				}
				result = base.GetTraceID(c)
				return c.SendString("OK")
			})

			req := httptest.NewRequest("GET", "/test", nil)
			if tt.header != "" {
				req.Header.Set("X-Trace-ID", tt.header)
			}

			_, err := app.Test(req)
			if err != nil {
				t.Fatalf("Failed to make request: %v", err)
			}

			if result != tt.expected {
				t.Errorf("Expected trace ID %q, got %q", tt.expected, result)
			}
		})
	}
}

func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
