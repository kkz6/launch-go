package tables

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/staff/services"
	"github.com/kkz6/launch-go/internal/pkg/table"
)

// FailuresTable backs the DataTable on /admin/failures/table. It is a pure
// Resolver table: it owns its entire data fetch by delegating to the existing
// Failures service, which merges provision/task/deployment failures into one
// newest-first feed. Because it implements table.Resolver, the query service
// skips the model-driven path entirely — Config.Model returns nil and the
// declared Columns()/Filters() are used only for the /meta schema.
type FailuresTable struct {
	svc *services.Service
}

// NewFailuresTable wires the staff service whose Failures method backs Resolve.
func NewFailuresTable(svc *services.Service) *FailuresTable {
	return &FailuresTable{svc: svc}
}

func (t *FailuresTable) Config() table.Config {
	return table.Config{
		Name:           "failures",
		Model:          func() any { return nil }, // pure resolver: never dereferenced
		DefaultPerPage: 25,
		PerPageOptions: []int{25, 50, 100},
		StickyHeader:   true,
	}
}

func (t *FailuresTable) Columns() []table.Column {
	return []table.Column{
		table.NewBadgeColumn("kind", "Kind").Variants(failureKindVariants()),
		table.NewTextColumn("title", "Title"),
		table.NewDateTimeColumn("when", "When").Format("2006-01-02 15:04"),
		table.NewTextColumn("error", "Error"),
		// detail is large/truncated task output. The Resolve row map always
		// carries it; declaring it as a toggleable text column lets the frontend
		// surface it (e.g. in a "View" expander) without bloating the default view.
		table.NewTextColumn("detail", "Detail"),
	}
}

func (t *FailuresTable) Filters() []table.Filter {
	return []table.Filter{
		table.NewSetFilter("kind", "Kind").
			Single().
			WithoutClause().
			Options([]table.FilterOption{
				{Value: "provision", Label: "Provision"},
				{Value: "task", Label: "Task"},
				{Value: "deployment", Label: "Deployment"},
			}),
	}
}

// Actions returns nil: the failures table is read-only. The per-row "View"
// (which renders detail) is handled entirely frontend-side, so no action
// column is declared and the mutating /action route is never mounted.
func (t *FailuresTable) Actions() []*table.Action { return nil }

func (t *FailuresTable) EmptyState() *table.EmptyState {
	return table.NewEmptyState().
		Title("No failures").
		Message("Failed provisions, tasks and deployments will appear here.").
		Icon("circle-check")
}

// Resolve owns the full data fetch by delegating to the Failures service. The
// declared columns/filters drive only the /meta schema (via table.Render).
func (t *FailuresTable) Resolve(ctx context.Context, req table.Request) (*table.TableResponse, error) {
	perPage := req.PerPage
	if perPage <= 0 {
		perPage = 25
	}
	page := req.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * perPage

	kind := extractKindFilter(req)

	resp, total, err := t.svc.Failures(ctx, kind, perPage, offset)
	if err != nil {
		return nil, err
	}

	rows := make([]map[string]any, 0, len(resp.Failures))
	for i := range resp.Failures {
		f := resp.Failures[i]
		rows = append(rows, map[string]any{
			"id":        f.ID,
			"kind":      f.Kind,
			"title":     f.Title,
			"when":      f.When,
			"error":     f.Error,
			"detail":    f.Detail,
			"team_id":   f.TeamID,
			"server_id": f.ServerID,
		})
	}

	lastPage := int((total + int64(perPage) - 1) / int64(perPage))
	if lastPage < 1 {
		lastPage = 1
	}
	from := int64(0)
	to := int64(0)
	if total > 0 {
		from = int64((page-1)*perPage + 1)
		to = int64(page * perPage)
		if to > total {
			to = total
		}
	}

	return &table.TableResponse{
		Meta: table.Render(t),
		Data: rows,
		Pagination: table.PaginationData{
			Type:        table.PaginationFull,
			CurrentPage: page,
			LastPage:    lastPage,
			PerPage:     perPage,
			Total:       total,
			From:        from,
			To:          to,
		},
	}, nil
}

// extractKindFilter reads the kind filter out of the request, accepting any of
// the clauses the kind SetFilter may emit (equals/in). It returns the first
// non-empty provision|task|deployment value, or "" to merge all sources.
func extractKindFilter(req table.Request) string {
	clauseMap, ok := req.Filters["kind"]
	if !ok {
		return ""
	}
	for _, raw := range clauseMap {
		if s := firstKind(raw); s != "" {
			return s
		}
	}
	return ""
}

// firstKind normalises a single filter value (string, []any, or []string) into
// the first valid kind it contains.
func firstKind(raw any) string {
	switch v := raw.(type) {
	case string:
		if isValidKind(v) {
			return v
		}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && isValidKind(s) {
				return s
			}
		}
	case []string:
		for _, s := range v {
			if isValidKind(s) {
				return s
			}
		}
	}
	return ""
}

func isValidKind(s string) bool {
	switch s {
	case "provision", "task", "deployment":
		return true
	}
	return false
}

// failureKindVariants maps each failure kind to a badge variant for the kind
// pill.
func failureKindVariants() map[string]table.Variant {
	return map[string]table.Variant{
		"provision":  table.VariantWarning,
		"task":       table.VariantDestructive,
		"deployment": table.VariantInfo,
	}
}
