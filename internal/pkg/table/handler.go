package table

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
)

// Mount registers /meta, /data, /action, /bulk-action, /views/* onto the
// given router group for the supplied Table. The action handlers run with
// the request's context so they pick up cancellation and tracing.
//
// Each module typically calls Mount inside its own RegisterRoutes:
//
//	users := router.Group("/users", authMw)
//	table.Mount(users.Group("/table"), m.table, m.tableQuery, m.tableViews)
func Mount(router fiber.Router, t Table, query *QueryService, views *ViewService) {
	h := &handler{table: t, query: query, views: views}
	router.Get("/meta", h.meta)
	router.Get("/data", h.data)
	router.Post("/action/:name", h.action)

	v := router.Group("/views")
	v.Get("/", h.listViews)
	v.Post("/", h.storeView)
	v.Delete("/:id", h.deleteView)
}

type handler struct {
	table Table
	query *QueryService
	views *ViewService
}

// fiberRequestSource adapts *fiber.Ctx to RequestSource.
type fiberRequestSource struct{ c *fiber.Ctx }

func (s fiberRequestSource) Query(key string, def ...string) string {
	if v := s.c.Query(key); v != "" {
		return v
	}
	if len(def) > 0 {
		return def[0]
	}
	return ""
}

func (s fiberRequestSource) Queries() map[string]string { return s.c.Queries() }

func (h *handler) meta(c *fiber.Ctx) error {
	return fiberctx.OK(c, "Table meta", Render(h.table))
}

func (h *handler) data(c *fiber.Ctx) error {
	req := ParseRequest(fiberRequestSource{c})
	res, err := h.query.Execute(c.Context(), h.table, req)
	if err != nil {
		return err
	}
	return fiberctx.OK(c, "Table data", res)
}

// actionRequest is the body shape for both row and bulk actions: provide
// `ids` (one or many).
type actionRequest struct {
	IDs []string `json:"ids" validate:"required,min=1,dive,len=26"`
}

func (h *handler) action(c *fiber.Ctx) error {
	name := c.Params("name")
	req, err := fiberctx.MustParseAndValidate[actionRequest](c)
	if err != nil {
		return err
	}

	allActions := append([]*Action{}, effectiveRowActions(h.table)...)
	for _, a := range h.table.Actions() {
		if a.IsBulk() {
			allActions = append(allActions, a)
		}
	}
	var target *Action
	for _, a := range allActions {
		if a.Name() == name {
			target = a
			break
		}
	}
	if target == nil {
		return fiber.NewError(fiber.StatusNotFound, "Action not found: "+name)
	}
	if target.Handler() == nil {
		return fiber.NewError(fiber.StatusBadRequest, "Action "+name+" has no handler")
	}
	if err := target.Handler()(c.Context(), req.IDs); err != nil {
		return err
	}
	return fiberctx.OK(c, "Action executed", nil)
}

func (h *handler) listViews(c *fiber.Ctx) error {
	userID, _ := fiberctx.GetUserID(c)
	out, err := h.views.List(c.Context(), h.table.Config().Name, userID)
	if err != nil {
		return err
	}
	return fiberctx.OK(c, "Views retrieved", out)
}

func (h *handler) storeView(c *fiber.Ctx) error {
	userID, _ := fiberctx.GetUserID(c)
	req, err := fiberctx.MustParseAndValidate[StoreViewRequest](c)
	if err != nil {
		return err
	}
	if strings.TrimSpace(req.Title) == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Title is required")
	}
	out, err := h.views.Create(c.Context(), h.table.Config().Name, userID, *req)
	//lint:ignore SA4023 ViewService.Create is currently stubbed to always error (saved views not enabled); the handler is correct for the real implementation.
	if err != nil {
		return err
	}
	return fiberctx.Created(c, "View saved", out)
}

func (h *handler) deleteView(c *fiber.Ctx) error {
	userID, _ := fiberctx.GetUserID(c)
	if err := h.views.Delete(c.Context(), h.table.Config().Name, userID, c.Params("id")); err != nil {
		return err
	}
	return fiberctx.OK(c, "View deleted", nil)
}
