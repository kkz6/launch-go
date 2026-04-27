package fiber

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	gofiber "github.com/gofiber/fiber/v2"
)

// ---------- Index ----------

func TestIndex_Success(t *testing.T) {
	app := gofiber.New(gofiber.Config{ErrorHandler: NewErrorHandler()})
	app.Get("/items", func(c *gofiber.Ctx) error {
		SetTeamContext(c, "team-1", "owner")
		return c.Next()
	}, Index("items retrieved", func(ctx context.Context, teamID string) ([]string, error) {
		if teamID != "team-1" {
			t.Fatalf("expected team-1, got %s", teamID)
		}
		return []string{"a", "b"}, nil
	}))

	resp, err := app.Test(httptest.NewRequest("GET", "/items", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body := readBody(t, resp.Body)
	if !strings.Contains(body, `"items retrieved"`) || !strings.Contains(body, `"a"`) {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestIndex_MissingTeam(t *testing.T) {
	app := gofiber.New(gofiber.Config{ErrorHandler: NewErrorHandler()})
	app.Get("/items", Index("items retrieved", func(ctx context.Context, teamID string) ([]string, error) {
		t.Fatal("service fn should not be called")
		return nil, nil
	}))

	resp, err := app.Test(httptest.NewRequest("GET", "/items", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestIndex_ServiceError(t *testing.T) {
	app := gofiber.New(gofiber.Config{ErrorHandler: NewErrorHandler()})
	app.Get("/items", func(c *gofiber.Ctx) error {
		SetTeamContext(c, "team-1", "owner")
		return c.Next()
	}, Index("ok", func(ctx context.Context, teamID string) (any, error) {
		return nil, NotFound("not here")
	}))

	resp, err := app.Test(httptest.NewRequest("GET", "/items", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// ---------- Show ----------

func TestShow_Success(t *testing.T) {
	app := gofiber.New(gofiber.Config{ErrorHandler: NewErrorHandler()})
	app.Get("/items/:id", func(c *gofiber.Ctx) error {
		SetTeamContext(c, "team-1", "owner")
		return c.Next()
	}, Show("retrieved", func(ctx context.Context, id, teamID string) (map[string]string, error) {
		if id != "abc" || teamID != "team-1" {
			t.Fatalf("unexpected params: id=%s team=%s", id, teamID)
		}
		return map[string]string{"id": id}, nil
	}))

	resp, err := app.Test(httptest.NewRequest("GET", "/items/abc", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body := readBody(t, resp.Body)
	if !strings.Contains(body, `"abc"`) {
		t.Fatalf("body missing id: %s", body)
	}
}

func TestShow_NotFound(t *testing.T) {
	app := gofiber.New(gofiber.Config{ErrorHandler: NewErrorHandler()})
	app.Get("/items/:id", func(c *gofiber.Ctx) error {
		SetTeamContext(c, "team-1", "owner")
		return c.Next()
	}, Show("ok", func(ctx context.Context, id, teamID string) (any, error) {
		return nil, NotFound("missing")
	}))

	resp, err := app.Test(httptest.NewRequest("GET", "/items/x", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// ---------- Create ----------

type createReq struct {
	Name string `json:"name" validate:"required"`
}

func TestCreate_Success(t *testing.T) {
	app := gofiber.New(gofiber.Config{ErrorHandler: NewErrorHandler()})
	app.Post("/items", func(c *gofiber.Ctx) error {
		SetTeamContext(c, "team-1", "owner")
		SetUserContext(c, "user-1", nil)
		return c.Next()
	}, Create("created", func(ctx context.Context, teamID, userID string, req *createReq) (map[string]string, error) {
		if teamID != "team-1" || userID != "user-1" || req.Name != "alpha" {
			t.Fatalf("unexpected: team=%s user=%s name=%s", teamID, userID, req.Name)
		}
		return map[string]string{"id": "new", "name": req.Name}, nil
	}))

	body := strings.NewReader(`{"name":"alpha"}`)
	req := httptest.NewRequest("POST", "/items", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	if !strings.Contains(readBody(t, resp.Body), `"alpha"`) {
		t.Fatalf("missing payload")
	}
}

func TestCreate_ValidationError(t *testing.T) {
	app := gofiber.New(gofiber.Config{ErrorHandler: NewErrorHandler()})
	app.Post("/items", func(c *gofiber.Ctx) error {
		SetTeamContext(c, "team-1", "owner")
		SetUserContext(c, "user-1", nil)
		return c.Next()
	}, Create("ok", func(ctx context.Context, teamID, userID string, req *createReq) (any, error) {
		t.Fatal("service fn must not be called when validation fails")
		return nil, nil
	}))

	req := httptest.NewRequest("POST", "/items", strings.NewReader(`{"name":""}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 422 {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

// ---------- Update ----------

type updateReq struct {
	Name string `json:"name" validate:"required"`
}

func TestUpdate_Success(t *testing.T) {
	app := gofiber.New(gofiber.Config{ErrorHandler: NewErrorHandler()})
	app.Patch("/items/:id", func(c *gofiber.Ctx) error {
		SetTeamContext(c, "team-1", "owner")
		SetUserContext(c, "user-1", nil)
		return c.Next()
	}, Update("updated", func(ctx context.Context, id, teamID, userID string, req *updateReq) (map[string]string, error) {
		if id != "abc" || req.Name != "beta" || userID != "user-1" {
			t.Fatalf("unexpected: id=%s user=%s name=%s", id, userID, req.Name)
		}
		return map[string]string{"id": id, "name": req.Name}, nil
	}))

	req := httptest.NewRequest("PATCH", "/items/abc", strings.NewReader(`{"name":"beta"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(readBody(t, resp.Body), `"beta"`) {
		t.Fatalf("missing payload")
	}
}

// ---------- Delete ----------

func TestDelete_Success(t *testing.T) {
	app := gofiber.New(gofiber.Config{ErrorHandler: NewErrorHandler()})
	called := false
	app.Delete("/items/:id", func(c *gofiber.Ctx) error {
		SetTeamContext(c, "team-1", "owner")
		SetUserContext(c, "user-1", nil)
		return c.Next()
	}, Delete(func(ctx context.Context, id, teamID, userID string) error {
		if id != "abc" || teamID != "team-1" || userID != "user-1" {
			t.Fatalf("unexpected: id=%s team=%s user=%s", id, teamID, userID)
		}
		called = true
		return nil
	}))

	resp, err := app.Test(httptest.NewRequest("DELETE", "/items/abc", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 204 {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	if !called {
		t.Fatal("service fn not invoked")
	}
}

func TestDelete_PropagatesError(t *testing.T) {
	app := gofiber.New(gofiber.Config{ErrorHandler: NewErrorHandler()})
	app.Delete("/items/:id", func(c *gofiber.Ctx) error {
		SetTeamContext(c, "team-1", "owner")
		SetUserContext(c, "user-1", nil)
		return c.Next()
	}, Delete(func(ctx context.Context, id, teamID, userID string) error {
		return NotFound("missing")
	}))

	resp, err := app.Test(httptest.NewRequest("DELETE", "/items/abc", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// ---------- Action ----------

func TestAction_Success(t *testing.T) {
	app := gofiber.New(gofiber.Config{ErrorHandler: NewErrorHandler()})
	called := false
	app.Post("/items/:id/sync", func(c *gofiber.Ctx) error {
		SetTeamContext(c, "team-1", "owner")
		SetUserContext(c, "user-1", nil)
		return c.Next()
	}, Action("synced", func(ctx context.Context, id, teamID, userID string) error {
		if id != "abc" || userID != "user-1" {
			t.Fatalf("unexpected id=%s user=%s", id, userID)
		}
		called = true
		return nil
	}))

	resp, err := app.Test(httptest.NewRequest("POST", "/items/abc/sync", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(readBody(t, resp.Body), `"synced"`) {
		t.Fatal("missing message")
	}
	if !called {
		t.Fatal("service fn not invoked")
	}
}

// ---------- IndexNested ----------

func TestIndexNested_Success(t *testing.T) {
	app := gofiber.New(gofiber.Config{ErrorHandler: NewErrorHandler()})
	app.Get("/parents/:id/items", func(c *gofiber.Ctx) error {
		SetTeamContext(c, "team-1", "owner")
		return c.Next()
	}, IndexNested("id", "items retrieved", func(ctx context.Context, parentID, teamID string) ([]string, error) {
		if parentID != "p1" || teamID != "team-1" {
			t.Fatalf("unexpected: parent=%s team=%s", parentID, teamID)
		}
		return []string{"x"}, nil
	}))

	resp, err := app.Test(httptest.NewRequest("GET", "/parents/p1/items", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// ---------- CreateNested ----------

type nestedCreateReq struct {
	Name string `json:"name" validate:"required"`
}

func TestCreateNested_Success(t *testing.T) {
	app := gofiber.New(gofiber.Config{ErrorHandler: NewErrorHandler()})
	app.Post("/parents/:id/items", func(c *gofiber.Ctx) error {
		SetTeamContext(c, "team-1", "owner")
		SetUserContext(c, "user-1", nil)
		return c.Next()
	}, CreateNested("id", "created", func(ctx context.Context, parentID, teamID, userID string, req *nestedCreateReq) (map[string]string, error) {
		if parentID != "p1" || req.Name != "alpha" || userID != "user-1" {
			t.Fatalf("unexpected parent=%s user=%s name=%s", parentID, userID, req.Name)
		}
		return map[string]string{"parent": parentID, "name": req.Name}, nil
	}))

	req := httptest.NewRequest("POST", "/parents/p1/items", strings.NewReader(`{"name":"alpha"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
}

// ---------- UpdateNested ----------

func TestUpdateNested_Success(t *testing.T) {
	app := gofiber.New(gofiber.Config{ErrorHandler: NewErrorHandler()})
	app.Patch("/parents/:parentId/items/:itemId", func(c *gofiber.Ctx) error {
		SetTeamContext(c, "team-1", "owner")
		SetUserContext(c, "user-1", nil)
		return c.Next()
	}, UpdateNested("parentId", "itemId", "updated", func(ctx context.Context, id, parentID, teamID, userID string, req *updateReq) (map[string]string, error) {
		if id != "i1" || parentID != "p1" || userID != "user-1" {
			t.Fatalf("unexpected parent=%s id=%s user=%s", parentID, id, userID)
		}
		return map[string]string{"id": id, "name": req.Name}, nil
	}))

	r := httptest.NewRequest("PATCH", "/parents/p1/items/i1", strings.NewReader(`{"name":"beta"}`))
	r.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(r)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// ---------- DeleteNested ----------

func TestDeleteNested_Success(t *testing.T) {
	app := gofiber.New(gofiber.Config{ErrorHandler: NewErrorHandler()})
	called := false
	app.Delete("/parents/:parentId/items/:itemId", func(c *gofiber.Ctx) error {
		SetTeamContext(c, "team-1", "owner")
		SetUserContext(c, "user-1", nil)
		return c.Next()
	}, DeleteNested("parentId", "itemId", func(ctx context.Context, id, parentID, teamID, userID string) error {
		if id != "i1" || parentID != "p1" || userID != "user-1" {
			t.Fatalf("unexpected parent=%s id=%s user=%s", parentID, id, userID)
		}
		called = true
		return nil
	}))

	resp, err := app.Test(httptest.NewRequest("DELETE", "/parents/p1/items/i1", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 204 {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	if !called {
		t.Fatal("service fn not called")
	}
}

// ---------- ShowNested ----------

func TestShowNested_Success(t *testing.T) {
	app := gofiber.New(gofiber.Config{ErrorHandler: NewErrorHandler()})
	app.Get("/parents/:parentId/items/:itemId", func(c *gofiber.Ctx) error {
		SetTeamContext(c, "team-1", "owner")
		return c.Next()
	}, ShowNested("parentId", "itemId", "retrieved", func(ctx context.Context, id, parentID, teamID string) (map[string]string, error) {
		if id != "i1" || parentID != "p1" {
			t.Fatalf("unexpected: parent=%s id=%s", parentID, id)
		}
		return map[string]string{"parent": parentID, "id": id}, nil
	}))

	resp, err := app.Test(httptest.NewRequest("GET", "/parents/p1/items/i1", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// ---------- ActionNested ----------

func TestActionNested_Success(t *testing.T) {
	app := gofiber.New(gofiber.Config{ErrorHandler: NewErrorHandler()})
	called := false
	app.Post("/parents/:parentId/items/sync", func(c *gofiber.Ctx) error {
		SetTeamContext(c, "team-1", "owner")
		SetUserContext(c, "user-1", nil)
		return c.Next()
	}, ActionNested("parentId", "synced", func(ctx context.Context, parentID, teamID, userID string) error {
		if parentID != "p1" || userID != "user-1" {
			t.Fatalf("unexpected: parent=%s user=%s", parentID, userID)
		}
		called = true
		return nil
	}))

	resp, err := app.Test(httptest.NewRequest("POST", "/parents/p1/items/sync", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !called {
		t.Fatal("service fn not invoked")
	}
}

// ---------- ActionItemNested ----------

func TestActionItemNested_Success(t *testing.T) {
	app := gofiber.New(gofiber.Config{ErrorHandler: NewErrorHandler()})
	called := false
	app.Post("/parents/:parentId/items/:itemId/run", func(c *gofiber.Ctx) error {
		SetTeamContext(c, "team-1", "owner")
		SetUserContext(c, "user-1", nil)
		return c.Next()
	}, ActionItemNested("parentId", "itemId", "started", func(ctx context.Context, id, parentID, teamID, userID string) error {
		if id != "i1" || parentID != "p1" || userID != "user-1" {
			t.Fatalf("unexpected: parent=%s id=%s user=%s", parentID, id, userID)
		}
		called = true
		return nil
	}))

	resp, err := app.Test(httptest.NewRequest("POST", "/parents/p1/items/i1/run", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !called {
		t.Fatal("service fn not invoked")
	}
}

// ---------- helpers ----------

func readBody(t *testing.T, r io.ReadCloser) string {
	t.Helper()
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}
