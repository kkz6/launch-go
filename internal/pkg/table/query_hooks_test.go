package table

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// widget is a minimal model used by the hook tests.
type widget struct {
	ID   uint   `gorm:"primaryKey" json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

func (widget) TableName() string { return "widgets" }

func setupWidgetDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&widget{}))
	require.NoError(t, db.Create(&[]widget{
		{Name: "alpha", Kind: "a"},
		{Name: "beta", Kind: "a"},
		{Name: "gamma", Kind: "b"},
	}).Error)
	return db
}

// ─── shared minimal Table implementation ────────────────────────────

type baseWidgetTable struct {
	model func() any
}

func (b baseWidgetTable) Config() Config {
	return Config{Name: "widgets", Model: b.model}
}
func (b baseWidgetTable) Columns() []Column {
	return []Column{
		NewTextColumn("name"),
		NewTextColumn("kind"),
	}
}
func (b baseWidgetTable) Filters() []Filter       { return nil }
func (b baseWidgetTable) Actions() []*Action      { return nil }
func (b baseWidgetTable) EmptyState() *EmptyState { return nil }

// ─── 1. Resolver ────────────────────────────────────────────────────

type resolverTable struct {
	baseWidgetTable
	resp *TableResponse
}

func (r resolverTable) Resolve(_ context.Context, _ Request) (*TableResponse, error) {
	return r.resp, nil
}

func TestExecute_ResolverDelegatesAndSkipsModel(t *testing.T) {
	db := setupWidgetDB(t)
	svc := &QueryService{db: db}

	canned := &TableResponse{
		Data: []map[string]any{{"id": 99, "marker": "from-resolver"}},
		Pagination: PaginationData{
			Type:        PaginationFull,
			CurrentPage: 1,
			Total:       1,
		},
	}

	// Model returns nil — proves Execute never touches it on the Resolver path.
	rt := resolverTable{
		baseWidgetTable: baseWidgetTable{model: func() any { return nil }},
		resp:            canned,
	}

	got, err := svc.Execute(context.Background(), rt, Request{})
	require.NoError(t, err)
	require.Same(t, canned, got)
}

// ─── 2. BaseQueryProvider ───────────────────────────────────────────

type baseQueryTable struct {
	baseWidgetTable
}

func (baseQueryTable) BaseQuery(db *gorm.DB) *gorm.DB {
	return db.Model(&widget{}).Where("kind = ?", "a")
}

func TestExecute_BaseQueryProviderScopesQuery(t *testing.T) {
	db := setupWidgetDB(t)
	svc := &QueryService{db: db}

	bt := baseQueryTable{
		baseWidgetTable: baseWidgetTable{model: func() any { return &widget{} }},
	}

	got, err := svc.Execute(context.Background(), bt, Request{Page: 1, PerPage: 10})
	require.NoError(t, err)
	require.Len(t, got.Data, 2)
	require.EqualValues(t, 2, got.Pagination.Total)

	kinds := map[string]struct{}{}
	for _, row := range got.Data {
		kinds[row["kind"].(string)] = struct{}{}
	}
	require.Equal(t, map[string]struct{}{"a": {}}, kinds)
}

// ─── 3. Default model-driven path (no hooks) ────────────────────────

func TestExecute_DefaultPathReturnsAllRows(t *testing.T) {
	db := setupWidgetDB(t)
	svc := &QueryService{db: db}

	plain := baseWidgetTable{model: func() any { return &widget{} }}

	got, err := svc.Execute(context.Background(), plain, Request{Page: 1, PerPage: 10})
	require.NoError(t, err)
	require.Len(t, got.Data, 3)
	require.EqualValues(t, 3, got.Pagination.Total)
}
