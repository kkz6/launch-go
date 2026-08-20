package table

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	fiberctx "github.com/kkz6/launch-go/internal/pkg/fiber"
	"github.com/kkz6/launch-go/internal/pkg/i18n"
)

// Mount registers /meta, /data, /action, /bulk-action, /views/* onto the
// given router group for the supplied Table. The action handlers run with
// the request's context so they pick up cancellation and tracing.
//
// Each module typically calls Mount inside its own RegisterRoutes:
//
//	users := router.Group("/users", authMw)
//	table.Mount(users.Group("/table"), m.table, m.tableQuery, m.tableViews)
//
// Optional actionMiddleware runs only on the mutating POST /action/:name
// route, in front of the action handler. This lets a module keep /meta and
// /data on the group's tier (e.g. support) while gating mutations behind a
// stricter role (e.g. super_admin). Omitting it leaves the action route on
// the group's middleware chain unchanged.
func Mount(router fiber.Router, t Table, query *QueryService, views *ViewService, actionMiddleware ...fiber.Handler) {
	h := &handler{table: t, query: query, views: views}
	router.Get("/meta", h.meta)
	router.Get("/data", h.data)

	actionHandlers := append(append([]fiber.Handler{}, actionMiddleware...), h.action)
	router.Post("/action/:name", actionHandlers...)

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
	return fiberctx.OK(c, "Table meta", localizeTableMeta(c, Render(h.table)))
}

func (h *handler) data(c *fiber.Ctx) error {
	req := ParseRequest(fiberRequestSource{c})
	res, err := h.query.Execute(c.Context(), h.table, req)
	if err != nil {
		return err
	}
	if res == nil {
		return fiberctx.OK(c, "Table data", nil)
	}
	localized := *res
	localized.Meta = localizeTableMeta(c, res.Meta)
	return fiberctx.OK(c, "Table data", &localized)
}

// localizeTableMeta copies and translates only server-owned display copy.
// Machine identifiers, option values, URLs, icons, formats, and arbitrary row
// or user content remain untouched.
func localizeTableMeta(c *fiber.Ctx, meta TableMeta) TableMeta {
	localized := meta

	localized.Columns = append([]ColumnSerialized(nil), meta.Columns...)
	for index := range localized.Columns {
		column := &localized.Columns[index]
		column.Header = i18n.T(c, column.Header)
		column.TrueLabel = localizedString(c, column.TrueLabel)
		column.FalseLabel = localizedString(c, column.FalseLabel)
	}

	localized.Filters = append([]FilterSerialized(nil), meta.Filters...)
	for index := range localized.Filters {
		filter := &localized.Filters[index]
		filter.Label = i18n.T(c, filter.Label)
		filter.Options = append([]FilterOption(nil), filter.Options...)
		for optionIndex := range filter.Options {
			filter.Options[optionIndex].Label = i18n.T(c, filter.Options[optionIndex].Label)
		}
	}

	localized.Actions.Row = localizeActions(c, meta.Actions.Row)
	localized.Actions.Bulk = localizeActions(c, meta.Actions.Bulk)
	localized.Search.Placeholder = i18n.T(c, meta.Search.Placeholder)

	if meta.EmptyState != nil {
		empty := *meta.EmptyState
		empty.Title = i18n.T(c, empty.Title)
		empty.Message = i18n.T(c, empty.Message)
		if meta.EmptyState.Action != nil {
			action := *meta.EmptyState.Action
			action.Label = i18n.T(c, action.Label)
			empty.Action = &action
		}
		localized.EmptyState = &empty
	}

	return localized
}

func localizeActions(c *fiber.Ctx, actions []ActionSerialized) []ActionSerialized {
	localized := append([]ActionSerialized(nil), actions...)
	for index := range localized {
		action := &localized[index]
		action.Label = i18n.T(c, action.Label)
		action.Tooltip = localizedString(c, action.Tooltip)
		if action.Confirm != nil {
			confirm := *action.Confirm
			confirm.Title = i18n.T(c, confirm.Title)
			confirm.Message = i18n.T(c, confirm.Message)
			confirm.ConfirmLabel = i18n.T(c, confirm.ConfirmLabel)
			confirm.CancelLabel = i18n.T(c, confirm.CancelLabel)
			action.Confirm = &confirm
		}
	}
	return localized
}

func localizedString(c *fiber.Ctx, value *string) *string {
	if value == nil {
		return nil
	}
	localized := i18n.T(c, *value)
	return &localized
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
		return fiber.NewError(fiber.StatusNotFound, i18n.T(c, "Action not found: %s", name))
	}
	if target.Handler() == nil {
		return fiber.NewError(fiber.StatusBadRequest, i18n.T(c, "Action %s has no handler", name))
	}
	userID, _ := fiberctx.GetUserID(c)
	actionCtx := WithActorID(c.Context(), userID)
	if err := target.Handler()(actionCtx, req.IDs); err != nil {
		// Surface business-rule failures (e.g. self-guards) as a 422 carrying
		// the message, so the DataTable can toast the real reason instead of a
		// blank 500. Handlers that already return a typed fiber error keep
		// their chosen status.
		if _, ok := err.(*fiber.Error); ok {
			return err
		}
		return fiber.NewError(fiber.StatusUnprocessableEntity, err.Error())
	}
	return fiberctx.OK(c, "Action executed", nil)
}

func (h *handler) listViews(c *fiber.Ctx) error {
	userID, _ := fiberctx.GetUserID(c)
	_, err := h.views.List(c.Context(), h.table.Config().Name, userID)
	return err
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
	// ViewService.Create is currently stubbed to always error (saved views not
	// enabled); the handler is correct for the real implementation.
	out, err := h.views.Create(c.Context(), h.table.Config().Name, userID, *req) //nolint:staticcheck // SA4023: stubbed Create always errors
	//lint:ignore SA4023 stubbed Create always returns a non-nil error
	if err != nil { //nolint:staticcheck // SA4023: stubbed Create always errors
		return err
	}
	return fiberctx.Created(c, "View saved", out)
}

func (h *handler) deleteView(c *fiber.Ctx) error {
	userID, _ := fiberctx.GetUserID(c)
	return h.views.Delete(c.Context(), h.table.Config().Name, userID, c.Params("id"))
}
