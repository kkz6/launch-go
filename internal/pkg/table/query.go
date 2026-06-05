package table

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"gorm.io/gorm"
)

// QueryService runs a Request against a Table and returns the rendered
// TableResponse. It owns search + filter application, sorting, pagination,
// and the per-row column mapping.
type QueryService struct {
	db *gorm.DB
}

// NewQueryService wires the service with the global GORM connection.
func NewQueryService(db *gorm.DB) *QueryService {
	return &QueryService{db: db}
}

// Execute runs a query and returns the table response.
func (s *QueryService) Execute(ctx context.Context, t Table, req Request) (*TableResponse, error) {
	// A Resolver table owns its entire data fetch — delegate before touching
	// cfg.Model(), which a pure Resolver table may leave nil.
	if r, ok := t.(Resolver); ok {
		return r.Resolve(ctx, req)
	}

	cfg := t.Config()
	model := cfg.Model()

	stmt := &gorm.Statement{DB: s.db, Context: ctx}
	if err := stmt.Parse(model); err != nil {
		return nil, fmt.Errorf("parse model: %w", err)
	}
	tableName := stmt.Table

	q := s.db.WithContext(ctx).Table(tableName)
	bq, hasBaseQuery := t.(BaseQueryProvider)
	if hasBaseQuery {
		q = bq.BaseQuery(s.db.WithContext(ctx))
	}

	// Apply search.
	if req.Search != "" {
		q = applySearch(q, t, tableName, req.Search)
	}

	// Apply filters.
	if len(req.Filters) > 0 {
		q = applyFilters(q, t, req.Filters)
	}

	// Default-value filters that were not overridden in the request.
	for _, f := range effectiveFilters(t) {
		if !f.HasDefault() {
			continue
		}
		if _, present := req.Filters[f.Attribute()]; present {
			continue
		}
		d := f.Default()
		val, ok := f.Validate(d.Value, d.Clause)
		if !ok && !d.Clause.IsValueless() {
			continue
		}
		q = f.Apply(q, f.Attribute(), d.Clause, val)
	}

	// Apply sort.
	q = applySort(q, t, req.Sort)

	// Count + paginate.
	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count: %w", err)
	}

	perPage := req.PerPage
	if perPage <= 0 {
		perPage = cfg.DefaultPerPage
	}
	if perPage <= 0 {
		perPage = 15
	}
	page := req.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * perPage

	fetchQuery := q.Offset(offset).Limit(perPage)

	// Secret-safety: pure model-driven tables SELECT only their declared
	// columns, so an undeclared column can never reach the response. Tables
	// that own their projection (BaseQueryProvider) or fetch (Resolver) are
	// exempt — the Resolver path already returned above.
	if !hasBaseQuery {
		if selectCols := projectionColumns(t, stmt, cfg.SoftDeletes); len(selectCols) > 0 {
			fetchQuery = fetchQuery.Select(selectCols)
		}
	}

	rows, err := fetchRows(fetchQuery, model, cfg.SoftDeletes)
	if err != nil {
		return nil, fmt.Errorf("fetch rows: %w", err)
	}

	data := make([]map[string]any, len(rows))
	for i, row := range rows {
		data[i] = transformRow(row, t)
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

	return &TableResponse{
		Meta: Render(t),
		Data: data,
		Pagination: PaginationData{
			Type:        choosePaginationType(cfg.Pagination),
			CurrentPage: page,
			LastPage:    lastPage,
			PerPage:     perPage,
			Total:       total,
			From:        from,
			To:          to,
		},
	}, nil
}

// projectionColumns builds the SELECT list for a pure model-driven table:
// every declared non-nested column's attribute, plus the model's primary key
// (so rows always carry an id for actions), plus deleted_at when soft-deletes
// are enabled (fetchRows/extractDeletedAt need it). The list is de-duplicated.
func projectionColumns(t Table, stmt *gorm.Statement, softDeletes bool) []string {
	cols := make([]string, 0)
	seen := map[string]struct{}{}
	add := func(name string) {
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		cols = append(cols, name)
	}

	pk := "id"
	if stmt.Schema != nil && stmt.Schema.PrioritizedPrimaryField != nil {
		pk = stmt.Schema.PrioritizedPrimaryField.DBName
	}
	add(pk)

	for _, c := range t.Columns() {
		if c.IsNested() {
			continue
		}
		if c.Attribute() == "_actions" {
			continue
		}
		add(c.Attribute())
	}

	if softDeletes {
		add("deleted_at")
	}

	return cols
}

func choosePaginationType(t PaginationType) PaginationType {
	if t == "" {
		return PaginationFull
	}
	return t
}

// applySearch ORs ILIKE clauses across both per-column .AsSearchable() opts
// and the Config.Searchable list.
func applySearch(db *gorm.DB, t Table, tableName, search string) *gorm.DB {
	fields := make([]string, 0)
	for _, c := range t.Columns() {
		if c.Searchable() {
			fields = append(fields, c.Attribute())
		}
	}
	fields = append(fields, t.Config().Searchable...)
	if len(fields) == 0 {
		return db
	}
	parts := make([]string, 0, len(fields))
	args := make([]any, 0, len(fields))
	for _, f := range fields {
		col := fmt.Sprintf(`"%s"."%s"`, tableName, f)
		parts = append(parts, col+" ILIKE ?")
		args = append(args, "%"+search+"%")
	}
	return db.Where(strings.Join(parts, " OR "), args...)
}

func applyFilters(db *gorm.DB, t Table, filters map[string]map[Clause]any) *gorm.DB {
	tableFilters := effectiveFilters(t)
	for _, f := range tableFilters {
		clauseMap, ok := filters[f.Attribute()]
		if !ok {
			continue
		}
		for clause, raw := range clauseMap {
			val, ok := f.Validate(raw, clause)
			if !ok && !clause.IsValueless() {
				continue
			}
			db = f.Apply(db, f.Attribute(), clause, val)
		}
	}
	return db
}

func applySort(db *gorm.DB, t Table, sort string) *gorm.DB {
	cfg := t.Config()
	if sort != "" {
		parts := strings.Split(sort, ":")
		key := parts[0]
		dir := "ASC"
		if len(parts) == 2 && strings.EqualFold(parts[1], "desc") {
			dir = "DESC"
		}
		// Only sortable columns are allowed.
		for _, c := range t.Columns() {
			if c.Attribute() == key && c.Sortable() {
				return db.Order(fmt.Sprintf(`"%s" %s`, key, dir))
			}
		}
	}
	if cfg.DefaultSort != nil {
		dir := "ASC"
		if cfg.DefaultSort.Direction == SortDesc {
			dir = "DESC"
		}
		return db.Order(fmt.Sprintf(`"%s" %s`, cfg.DefaultSort.Column, dir))
	}
	return db
}

// fetchRows runs the query and returns []map[string]any. We round-trip
// through GORM into the concrete model type (using reflection) then JSON
// re-marshal so the response shape exactly matches the model's struct tags
// — without the package needing to know individual model fields.
//
// When includeDeletedAt is set, we reflect the DeletedAt timestamp back onto
// each row as the `deleted_at` key (the BaseModel tags it `json:"-"` so it
// won't be there otherwise). Action visibility predicates use this to hide
// Restore/ForceDelete for live rows.
func fetchRows(db *gorm.DB, model any, includeDeletedAt bool) ([]map[string]any, error) {
	t := reflect.TypeOf(model)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	sliceType := reflect.SliceOf(t)
	slicePtr := reflect.New(sliceType)
	if err := db.Find(slicePtr.Interface()).Error; err != nil {
		return nil, err
	}
	// Marshal → unmarshal to convert struct tags + nested types to plain maps.
	raw, err := json.Marshal(slicePtr.Elem().Interface())
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if includeDeletedAt {
		slice := slicePtr.Elem()
		for i := 0; i < slice.Len(); i++ {
			elem := slice.Index(i)
			for elem.Kind() == reflect.Pointer {
				elem = elem.Elem()
			}
			if ts, ok := extractDeletedAt(elem); ok {
				out[i]["deleted_at"] = ts
			} else {
				out[i]["deleted_at"] = nil
			}
		}
	}
	return out, nil
}

// extractDeletedAt finds the embedded BaseModel's DeletedAt (a
// gorm.DeletedAt) and returns its time as an RFC3339 string if it's valid,
// since gorm.DeletedAt is a struct wrapping sql.NullTime.
func extractDeletedAt(v reflect.Value) (string, bool) {
	if v.Kind() != reflect.Struct {
		return "", false
	}
	f := v.FieldByName("DeletedAt")
	if !f.IsValid() {
		// Walk one level of embedded structs (BaseModel pattern).
		for i := 0; i < v.NumField(); i++ {
			sf := v.Type().Field(i)
			if sf.Anonymous && v.Field(i).Kind() == reflect.Struct {
				if r, ok := extractDeletedAt(v.Field(i)); ok {
					return r, true
				}
			}
		}
		return "", false
	}
	// f is gorm.DeletedAt {Time time.Time, Valid bool}
	valid := f.FieldByName("Valid")
	if !valid.IsValid() || !valid.Bool() {
		return "", false
	}
	tField := f.FieldByName("Time")
	if !tField.IsValid() {
		return "", false
	}
	if iface := tField.Interface(); iface != nil {
		if b, err := json.Marshal(iface); err == nil {
			s := strings.Trim(string(b), `"`)
			return s, true
		}
	}
	return "", false
}

// transformRow walks every column and produces the wire shape: for non-action
// columns the (possibly mapped) value goes under the column's attribute key;
// for the action column we expand the per-row Action metadata.
func transformRow(row map[string]any, t Table) map[string]any {
	out := map[string]any{"id": row["id"]}

	for _, c := range t.Columns() {
		if c.Attribute() == "_actions" {
			continue
		}
		raw := c.GetDataFrom(row)
		out[c.Attribute()] = c.MapForTable(raw, row)
	}

	if serialized := SerializeRowActions(t, row); len(serialized) > 0 {
		out["_actions"] = serialized
	}
	return out
}

// SerializeRowActions resolves a table's row actions against a single row,
// applying each action's Hidden/Disabled/URL resolver and returning the wire
// shape the frontend's RowActions component consumes under `_actions`.
//
// The model-driven query path applies this inside transformRow. Resolver
// tables — which own their entire data fetch and therefore skip transformRow —
// must call this themselves for each row they emit, otherwise row actions
// never reach the client.
func SerializeRowActions(t Table, row map[string]any) []map[string]any {
	rowActions := effectiveRowActions(t)
	if len(rowActions) == 0 {
		return nil
	}

	serialized := make([]map[string]any, 0, len(rowActions))
	for _, a := range rowActions {
		s := a.Serialize()
		if a.IsHiddenFor(row) {
			s.Hidden = true
		}
		if a.IsDisabledFor(row) {
			s.Disabled = true
		}
		if url := a.ResolveURL(row); url != nil {
			s.URL = url
		}
		b, _ := json.Marshal(s)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		serialized = append(serialized, m)
	}
	return serialized
}
