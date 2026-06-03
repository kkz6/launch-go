package table

import (
	"context"
	"strings"
	"unicode"

	"gorm.io/gorm"
)

// Config carries the static, per-table configuration. Equivalent to the
// scouted @TableConfig decorator; values are read by the query service to
// drive search, pagination, default sort, and soft-delete behavior.
type Config struct {
	// Name is the registry key; also used in URLs (/api/.../table/{name}/...).
	Name string

	// Model returns a fresh instance of the GORM model (e.g. &User{}) so the
	// query service can dispatch table-name + column reflection.
	Model func() any

	// DefaultSort applies when the request omits ?sort=.
	DefaultSort *SortClause

	Pagination     PaginationType
	PerPageOptions []int
	DefaultPerPage int

	// SoftDeletes makes the query service auto-add a TrashedFilter and the
	// row actions list auto-include restore + forceDelete.
	SoftDeletes bool

	// Searchable lists model fields used by global ?search= in addition to
	// any column.Searchable() opt-ins.
	Searchable []string

	StickyHeader   bool
	Debounce       int
	ScrollPosition ScrollPosition
}

// SortClause is one (column, direction) pair.
type SortClause struct {
	Column    string
	Direction SortDirection
}

// Table is the interface every consumer implements. Use Builder() to avoid
// implementing it by hand for simple cases.
type Table interface {
	Config() Config
	Columns() []Column
	Filters() []Filter
	Actions() []*Action
	EmptyState() *EmptyState
}

// BaseQueryProvider lets a table supply a scoped/joined base query instead of
// the default db.Model(cfg.Model()). The returned *gorm.DB should already have
// the model/table set (e.g. db.Model(&X{}).Joins(...).Where(...)). The query
// service then applies search/filter/sort/count/pagination on top.
type BaseQueryProvider interface {
	BaseQuery(db *gorm.DB) *gorm.DB
}

// Resolver lets a table fully own its data fetch (custom assembly, unions,
// pagination). When a Table implements Resolver, the query service delegates
// entirely and returns its TableResponse; the declared Columns()/Filters() are
// then used only for the /meta schema, not for querying.
type Resolver interface {
	Resolve(ctx context.Context, req Request) (*TableResponse, error)
}

// Render builds the meta payload for a table.
func Render(t Table) TableMeta {
	cfg := t.Config()
	cols := t.Columns()
	filters := effectiveFilters(t)
	row, bulk := splitActions(effectiveRowActions(t), t.Actions())

	// Dedupe across column-level .AsSearchable() opt-ins and Config.Searchable
	// (a field can appear in both — the placeholder shouldn't repeat it).
	seen := map[string]struct{}{}
	allSearch := make([]string, 0)
	addLabel := func(label string) {
		key := strings.ToLower(label)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		allSearch = append(allSearch, label)
	}
	for _, c := range cols {
		if c.Searchable() {
			addLabel(c.Header())
		}
	}
	for _, f := range cfg.Searchable {
		addLabel(humanize(f))
	}
	enabled := len(allSearch) > 0
	placeholder := ""
	if enabled {
		placeholder = "Search"
	}

	perPage := cfg.PerPageOptions
	if len(perPage) == 0 {
		perPage = []int{15, 30, 50, 100}
	}
	debounce := cfg.Debounce
	if debounce == 0 {
		debounce = 300
	}
	scroll := cfg.ScrollPosition
	if scroll == "" {
		scroll = ScrollTopOfPage
	}

	colSer := make([]ColumnSerialized, len(cols))
	for i, c := range cols {
		colSer[i] = c.Serialize()
	}
	filtSer := make([]FilterSerialized, len(filters))
	for i, f := range filters {
		filtSer[i] = f.Serialize()
	}
	rowSer := make([]ActionSerialized, len(row))
	for i, a := range row {
		rowSer[i] = a.Serialize()
	}
	bulkSer := make([]ActionSerialized, len(bulk))
	for i, a := range bulk {
		bulkSer[i] = a.Serialize()
	}

	var es *EmptyStateSerialized
	if t.EmptyState() != nil {
		v := t.EmptyState().Serialize()
		es = &v
	}

	return TableMeta{
		Columns:        colSer,
		Filters:        filtSer,
		Actions:        TableActions{Row: rowSer, Bulk: bulkSer},
		Search:         TableSearchMeta{Enabled: enabled, Placeholder: placeholder},
		PerPageOptions: perPage,
		SoftDeletes:    cfg.SoftDeletes,
		StickyHeader:   cfg.StickyHeader,
		Debounce:       debounce,
		ScrollPosition: scroll,
		Views:          []ViewSerialized{},
		EmptyState:     es,
		Exports:        []any{},
	}
}

// effectiveFilters auto-injects a TrashedFilter when the table opts into
// soft deletes, mirroring BaseTable.getFilters().
func effectiveFilters(t Table) []Filter {
	out := append([]Filter{}, t.Filters()...)
	if t.Config().SoftDeletes {
		has := false
		for _, f := range out {
			if _, ok := f.(*TrashedFilter); ok {
				has = true
				break
			}
		}
		if !has {
			out = append(out, NewTrashedFilter())
		}
	}
	return out
}

// effectiveRowActions returns the row actions, auto-adding restore +
// forceDelete when soft deletes are enabled.
func effectiveRowActions(t Table) []*Action {
	all := t.Actions()
	out := make([]*Action, 0, len(all))
	for _, a := range all {
		if !a.IsBulk() {
			out = append(out, a)
		}
	}
	if t.Config().SoftDeletes {
		hasRestore, hasForce := false, false
		for _, a := range out {
			if a.Name() == "restore" {
				hasRestore = true
			}
			if a.Name() == "forceDelete" {
				hasForce = true
			}
		}
		// Restore + ForceDelete only make sense for already-trashed rows. The
		// frontend ships `deleted_at` on the row payload when the trashed
		// filter pulls them in; hide both actions when the row isn't trashed
		// so live records don't show a useless Restore icon.
		onlyTrashed := func(row map[string]any) bool {
			v, ok := row["deleted_at"]
			if !ok || v == nil {
				return true
			}
			if s, ok := v.(string); ok && s == "" {
				return true
			}
			return false
		}
		if !hasRestore {
			out = append(out, NewAction("restore", "Restore").
				AsButton().
				Variant(VariantSuccess).
				Icon("rotate-ccw").
				Tooltip("Restore from trash").
				Hidden(onlyTrashed).
				Confirm(ActionConfirm{Title: "Restore this item?", Message: "This will restore the item from trash."}))
		}
		if !hasForce {
			out = append(out, NewAction("forceDelete", "Force Delete").
				AsButton().
				Variant(VariantDestructive).
				Icon("trash").
				Tooltip("Permanently delete").
				Hidden(onlyTrashed).
				Confirm(ActionConfirm{Title: "Permanently delete?", Message: "This action cannot be undone."}))
		}
	}
	return out
}

func splitActions(rowEffective, all []*Action) (row, bulk []*Action) {
	row = rowEffective
	bulk = make([]*Action, 0)
	for _, a := range all {
		if a.IsBulk() {
			bulk = append(bulk, a)
		}
	}
	return row, bulk
}

// humanize converts "first_name" → "First name", "createdAt" → "Created at",
// "department.name" → "Name". Used as a fallback header / label generator.
func humanize(attr string) string {
	seg := attr
	if i := strings.LastIndex(attr, "."); i >= 0 {
		seg = attr[i+1:]
	}
	var b strings.Builder
	prevLower := false
	for _, r := range seg {
		if r == '_' || r == '.' || r == '-' {
			_ = b.WriteByte(' ')
			prevLower = false
			continue
		}
		if unicode.IsUpper(r) && prevLower {
			_ = b.WriteByte(' ')
		}
		_, _ = b.WriteRune(unicode.ToLower(r))
		prevLower = unicode.IsLower(r) || unicode.IsDigit(r)
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return ""
	}
	return strings.ToUpper(out[:1]) + out[1:]
}
