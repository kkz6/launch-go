package tables

import (
	"context"

	"github.com/kkz6/launch-go/internal/modules/staff/services"
	"github.com/kkz6/launch-go/internal/pkg/table"
)

// FailuresTable backs the DataTable on /admin/failures/table. It is a pure
// Resolver table: it owns its entire data fetch by delegating to the existing
// Failures service, which merges provision / site-installation /
// service-installation failures into one newest-first feed. Because it
// implements table.Resolver, the query service
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
		table.NewBadgeColumn("kind", "Kind").
			Variants(failureKindVariants()).
			Labels(failureKindLabels()),
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
				{Value: services.KindProvision, Label: "Provision"},
				{Value: services.KindSiteInstallation, Label: "Site installation"},
				{Value: services.KindServiceInstallation, Label: "Service installation"},
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
		Message("Failed provisions, site installations and service installations will appear here.").
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
		row := map[string]any{
			"id":        f.ID,
			"kind":      f.Kind,
			"title":     f.Title,
			"when":      f.When,
			"error":     f.Error,
			"detail":    f.Detail,
			"team_id":   f.TeamID,
			"server_id": f.ServerID,
			// kind_raw preserves the unmapped kind string for the frontend's
			// "View log" action (which calls the log endpoint with ?kind=…),
			// since MapRow rewrites the declared "kind" column into a badge
			// {value,variant} object for display.
			"kind_raw": f.Kind,
		}
		// Apply declared-column mapping so the kind renders as a labelled,
		// coloured badge ({value,variant}) like a model-driven table would.
		table.MapRow(t, row)
		rows = append(rows, row)
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
// non-empty provision | site_installation | service_installation value, or ""
// to merge all sources.
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
	case services.KindProvision, services.KindSiteInstallation, services.KindServiceInstallation:
		return true
	}
	return false
}

// failureKindVariants maps each failure kind to a badge variant for the kind
// pill.
func failureKindVariants() map[string]table.Variant {
	return map[string]table.Variant{
		services.KindProvision:           table.VariantWarning,
		services.KindSiteInstallation:    table.VariantInfo,
		services.KindServiceInstallation: table.VariantDestructive,
	}
}

// failureKindLabels maps each failure kind to its human display label for the
// kind pill (e.g. "service_installation" → "Service installation").
func failureKindLabels() map[string]string {
	return map[string]string{
		services.KindProvision:           "Provision",
		services.KindSiteInstallation:    "Site installation",
		services.KindServiceInstallation: "Service installation",
	}
}
