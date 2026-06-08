package middleware

import (
	"context"
	"errors"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

// memCache is a tiny in-memory cache.Cache with TTL expiry for tests.
type memCache struct {
	mu sync.Mutex
	m  map[string]memEntry
}

type memEntry struct {
	val string
	exp time.Time
}

func newMemCache() *memCache { return &memCache{m: map[string]memEntry{}} }

func (c *memCache) Get(_ context.Context, k string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[k]
	if !ok || time.Now().After(e.exp) {
		return "", errors.New("not found")
	}
	return e.val, nil
}

func (c *memCache) Set(_ context.Context, k, v string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[k] = memEntry{val: v, exp: time.Now().Add(ttl)}
	return nil
}

func (c *memCache) Delete(_ context.Context, k string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.m, k)
	return nil
}

func (c *memCache) Exists(_ context.Context, k string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[k]
	return ok && !time.Now().After(e.exp), nil
}

func newRLApp(limit int, window time.Duration) *fiber.App {
	app := fiber.New()
	app.Use(GlobalRateLimit(limit, window))
	h := func(c *fiber.Ctx) error { return c.SendString("ok") }
	app.Get("/health", h)
	app.Get("/data", h)
	return app
}

func status(t *testing.T, app *fiber.App, path string) int {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest("GET", path, nil))
	require.NoError(t, err)
	return resp.StatusCode
}

func TestGlobalRateLimit_HealthAndLoopbackExempt(t *testing.T) {
	InitRateLimitMiddleware(newMemCache())
	app := newRLApp(2, time.Minute)

	// /health is exempt by path: never 429 no matter how many hits.
	for i := 0; i < 25; i++ {
		require.Equal(t, fiber.StatusOK, status(t, app, "/health"),
			"health probe must never be rate limited (req %d)", i)
	}
}

func TestGlobalRateLimit_BlocksOverLimit(t *testing.T) {
	InitRateLimitMiddleware(newMemCache())
	app := newRLApp(3, time.Minute)

	for i := 0; i < 3; i++ {
		require.Equal(t, fiber.StatusOK, status(t, app, "/data"), "request %d should pass", i)
	}
	require.Equal(t, fiber.StatusTooManyRequests, status(t, app, "/data"), "4th request over limit")
}

// TestGlobalRateLimit_WindowResets is the regression guard for the sliding-TTL
// bug: a key hit continuously within its window must still reset once the fixed
// window elapses (previously the TTL was refreshed on every request, so a
// busy key stayed at 429 forever).
func TestGlobalRateLimit_WindowResets(t *testing.T) {
	InitRateLimitMiddleware(newMemCache())
	window := 200 * time.Millisecond
	app := newRLApp(3, window)

	for i := 0; i < 3; i++ {
		require.Equal(t, fiber.StatusOK, status(t, app, "/data"))
	}
	require.Equal(t, fiber.StatusTooManyRequests, status(t, app, "/data"))

	// Keep hitting mid-window — must stay blocked and must NOT extend the window.
	time.Sleep(80 * time.Millisecond)
	require.Equal(t, fiber.StatusTooManyRequests, status(t, app, "/data"))

	// After the original window fully elapses, the bucket resets.
	time.Sleep(window)
	require.Equal(t, fiber.StatusOK, status(t, app, "/data"), "window should reset")
}

func TestDecodeRateLimit(t *testing.T) {
	// Round-trip.
	exp := time.UnixMilli(1_700_000_000_000)
	count, got, ok := decodeRateLimit(encodeRateLimit(7, exp))
	require.True(t, ok)
	require.Equal(t, 7, count)
	require.True(t, got.Equal(exp))

	// Legacy bare-integer value (pre-fix) is treated as not-ok → new window.
	_, _, ok = decodeRateLimit("42")
	require.False(t, ok)

	// Garbage.
	_, _, ok = decodeRateLimit("abc|def")
	require.False(t, ok)
}
