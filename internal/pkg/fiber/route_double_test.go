package fiber

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	gofiber "github.com/gofiber/fiber/v2"
)

// Doubly-nested helpers cover URL patterns with two parent ids:
//   /grandparent/:gid/parent/:pid/items[/:id][/action]
//
// Examples in the codebase:
//   /servers/:serverId/sites/:id/queues          (Index/Create double-nested)
//   /servers/:serverId/sites/:id/queues/:queueId (Update/Delete double-nested)
//   /servers/:serverId/sites/:id/queues/sync     (Action double-nested)
//   /servers/:id/upstreams/:upstreamId/backends  (Index/Create double-nested)

func TestIndexDoubleNested_Success(t *testing.T) {
	app := gofiber.New(gofiber.Config{ErrorHandler: NewErrorHandler()})
	app.Get("/g/:gid/p/:pid/items", func(c *gofiber.Ctx) error {
		SetTeamContext(c, "team-1", "owner")
		return c.Next()
	}, IndexDoubleNested("gid", "pid", "items retrieved", func(ctx context.Context, parentID, grandparentID, teamID string) ([]string, error) {
		if grandparentID != "g1" || parentID != "p1" {
			t.Fatalf("unexpected: g=%s p=%s", grandparentID, parentID)
		}
		return []string{"a"}, nil
	}))

	resp, err := app.Test(httptest.NewRequest("GET", "/g/g1/p/p1/items", nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestShowDoubleNested_Success(t *testing.T) {
	app := gofiber.New(gofiber.Config{ErrorHandler: NewErrorHandler()})
	app.Get("/g/:gid/p/:pid/items/:itemId", func(c *gofiber.Ctx) error {
		SetTeamContext(c, "team-1", "owner")
		return c.Next()
	}, ShowDoubleNested("gid", "pid", "itemId", "ok", func(ctx context.Context, id, parentID, grandparentID, teamID string) (map[string]string, error) {
		if id != "i1" || parentID != "p1" || grandparentID != "g1" {
			t.Fatalf("unexpected: g=%s p=%s id=%s", grandparentID, parentID, id)
		}
		return map[string]string{"id": id}, nil
	}))

	resp, err := app.Test(httptest.NewRequest("GET", "/g/g1/p/p1/items/i1", nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

type dnCreateReq struct {
	Name string `json:"name" validate:"required"`
}

func TestCreateDoubleNested_Success(t *testing.T) {
	app := gofiber.New(gofiber.Config{ErrorHandler: NewErrorHandler()})
	app.Post("/g/:gid/p/:pid/items", func(c *gofiber.Ctx) error {
		SetTeamContext(c, "team-1", "owner")
		SetUserContext(c, "user-1", nil)
		return c.Next()
	}, CreateDoubleNested("gid", "pid", "created", func(ctx context.Context, parentID, grandparentID, teamID, userID string, req *dnCreateReq) (map[string]string, error) {
		if grandparentID != "g1" || parentID != "p1" || userID != "user-1" || req.Name != "alpha" {
			t.Fatalf("unexpected: g=%s p=%s user=%s name=%s", grandparentID, parentID, userID, req.Name)
		}
		return map[string]string{"name": req.Name}, nil
	}))

	r := httptest.NewRequest("POST", "/g/g1/p/p1/items", strings.NewReader(`{"name":"alpha"}`))
	r.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(r)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
}

func TestUpdateDoubleNested_Success(t *testing.T) {
	app := gofiber.New(gofiber.Config{ErrorHandler: NewErrorHandler()})
	app.Patch("/g/:gid/p/:pid/items/:itemId", func(c *gofiber.Ctx) error {
		SetTeamContext(c, "team-1", "owner")
		SetUserContext(c, "user-1", nil)
		return c.Next()
	}, UpdateDoubleNested("gid", "pid", "itemId", "updated", func(ctx context.Context, id, parentID, grandparentID, teamID, userID string, req *dnCreateReq) (map[string]string, error) {
		if id != "i1" || parentID != "p1" || grandparentID != "g1" {
			t.Fatalf("unexpected ids")
		}
		return map[string]string{"id": id, "name": req.Name}, nil
	}))

	r := httptest.NewRequest("PATCH", "/g/g1/p/p1/items/i1", strings.NewReader(`{"name":"beta"}`))
	r.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(r)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestDeleteDoubleNested_Success(t *testing.T) {
	app := gofiber.New(gofiber.Config{ErrorHandler: NewErrorHandler()})
	called := false
	app.Delete("/g/:gid/p/:pid/items/:itemId", func(c *gofiber.Ctx) error {
		SetTeamContext(c, "team-1", "owner")
		SetUserContext(c, "user-1", nil)
		return c.Next()
	}, DeleteDoubleNested("gid", "pid", "itemId", func(ctx context.Context, id, parentID, grandparentID, teamID, userID string) error {
		if id != "i1" || parentID != "p1" || grandparentID != "g1" {
			t.Fatalf("unexpected ids")
		}
		called = true
		return nil
	}))

	resp, err := app.Test(httptest.NewRequest("DELETE", "/g/g1/p/p1/items/i1", nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != 204 {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	if !called {
		t.Fatal("service fn not invoked")
	}
}

func TestActionDoubleNested_Success(t *testing.T) {
	app := gofiber.New(gofiber.Config{ErrorHandler: NewErrorHandler()})
	called := false
	app.Post("/g/:gid/p/:pid/items/sync", func(c *gofiber.Ctx) error {
		SetTeamContext(c, "team-1", "owner")
		SetUserContext(c, "user-1", nil)
		return c.Next()
	}, ActionDoubleNested("gid", "pid", "synced", func(ctx context.Context, parentID, grandparentID, teamID, userID string) error {
		called = true
		return nil
	}))

	resp, err := app.Test(httptest.NewRequest("POST", "/g/g1/p/p1/items/sync", nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !called {
		t.Fatal("service fn not invoked")
	}
}

func TestActionItemDoubleNested_Success(t *testing.T) {
	app := gofiber.New(gofiber.Config{ErrorHandler: NewErrorHandler()})
	called := false
	app.Post("/g/:gid/p/:pid/items/:itemId/restart", func(c *gofiber.Ctx) error {
		SetTeamContext(c, "team-1", "owner")
		SetUserContext(c, "user-1", nil)
		return c.Next()
	}, ActionItemDoubleNested("gid", "pid", "itemId", "started", func(ctx context.Context, id, parentID, grandparentID, teamID, userID string) error {
		called = true
		return nil
	}))

	resp, err := app.Test(httptest.NewRequest("POST", "/g/g1/p/p1/items/i1/restart", nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !called {
		t.Fatal("service fn not invoked")
	}
}
