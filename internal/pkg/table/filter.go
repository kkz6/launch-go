package table

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Filter is what an applyable filter implements. Validate normalises the
// raw URL value (or rejects it) and Apply adds a WHERE clause to the
// supplied GORM query.
type Filter interface {
	Attribute() string
	Label() string
	Type() string
	Clauses() []Clause
	Validate(value any, clause Clause) (any, bool)
	Apply(db *gorm.DB, attribute string, clause Clause, value any) *gorm.DB
	IsNested() bool
	RelationshipName() string
	RelationshipColumn() string
	Serialize() FilterSerialized
	HasDefault() bool
	Default() FilterDefault
}

// filterBase keeps the shared mutable state. Variants embed it and override
// what they need.
type filterBase struct {
	attribute     string
	label         string
	clauses       []Clause
	hasDefault    bool
	defaultValue  any
	defaultClause Clause
	hidden        bool
}

func newFilterBase(attribute string, label ...string) filterBase {
	l := ""
	if len(label) > 0 {
		l = label[0]
	}
	return filterBase{attribute: attribute, label: l}
}

func (b *filterBase) Attribute() string { return b.attribute }
func (b *filterBase) Label() string {
	if b.label != "" {
		return b.label
	}
	return humanize(b.attribute)
}

func (b *filterBase) IsNested() bool {
	return strings.Contains(b.attribute, ".") && !strings.HasPrefix(b.attribute, "pivot.")
}

func (b *filterBase) RelationshipName() string {
	if i := strings.LastIndex(b.attribute, "."); i >= 0 {
		return b.attribute[:i]
	}
	return b.attribute
}

func (b *filterBase) RelationshipColumn() string {
	if i := strings.LastIndex(b.attribute, "."); i >= 0 {
		return b.attribute[i+1:]
	}
	return b.attribute
}

func (b *filterBase) HasDefault() bool { return b.hasDefault }
func (b *filterBase) Default() FilterDefault {
	return FilterDefault{Value: b.defaultValue, Clause: b.defaultClause}
}

func (b *filterBase) baseSerialize(typeName string, defaultClauses []Clause) FilterSerialized {
	clauses := b.clauses
	if len(clauses) == 0 {
		clauses = defaultClauses
	}
	out := FilterSerialized{
		Key:     b.attribute,
		Label:   b.Label(),
		Type:    typeName,
		Clauses: clauses,
		Hidden:  b.hidden,
	}
	if b.hasDefault {
		dc := b.defaultClause
		if dc == "" {
			dc = clauses[0]
		}
		out.Default = &FilterDefault{Value: b.defaultValue, Clause: dc}
	}
	return out
}

// resolveAttribute prepends the model's table-name alias when the attribute
// is not nested. Nested attributes (e.g. "department.name") are left alone
// and the query service is responsible for joining the relation.
func resolveAttribute(db *gorm.DB, attribute string) string {
	if strings.Contains(attribute, ".") {
		return attribute
	}
	if stmt := db.Statement; stmt != nil && stmt.Table != "" {
		return fmt.Sprintf(`"%s"."%s"`, stmt.Table, attribute)
	}
	return attribute
}

// applyValueless covers the universal IsSet / IsNotSet clauses; returns
// (db, true) when the clause was handled.
func applyValueless(db *gorm.DB, attribute string, clause Clause) (*gorm.DB, bool) {
	col := resolveAttribute(db, attribute)
	switch clause {
	case ClauseIsSet:
		return db.Where(col + " IS NOT NULL"), true
	case ClauseIsNotSet:
		return db.Where(col + " IS NULL"), true
	}
	return db, false
}

// ─── TextFilter ─────────────────────────────────────────────────────

var textDefaultClauses = []Clause{
	ClauseContains, ClauseNotContains, ClauseStartsWith, ClauseEndsWith,
	ClauseNotStartsWith, ClauseNotEndsWith, ClauseEquals, ClauseNotEquals,
}

type TextFilter struct{ filterBase }

func NewTextFilter(attribute string, label ...string) *TextFilter {
	return &TextFilter{filterBase: newFilterBase(attribute, label...)}
}

func (f *TextFilter) Type() string { return "text" }
func (f *TextFilter) Clauses() []Clause {
	if len(f.clauses) > 0 {
		return f.clauses
	}
	return textDefaultClauses
}

func (f *TextFilter) Validate(value any, _ Clause) (any, bool) {
	switch v := value.(type) {
	case string:
		if v == "" {
			return nil, false
		}
		return v, true
	case fmt.Stringer:
		return v.String(), true
	default:
		return fmt.Sprint(v), true
	}
}

func (f *TextFilter) Apply(db *gorm.DB, attribute string, clause Clause, value any) *gorm.DB {
	if next, done := applyValueless(db, attribute, clause); done {
		return next
	}
	col := resolveAttribute(db, attribute)
	s, _ := value.(string)
	switch clause {
	case ClauseContains:
		return db.Where(col+" ILIKE ?", "%"+s+"%")
	case ClauseNotContains:
		return db.Where(col+" NOT ILIKE ?", "%"+s+"%")
	case ClauseStartsWith:
		return db.Where(col+" ILIKE ?", s+"%")
	case ClauseEndsWith:
		return db.Where(col+" ILIKE ?", "%"+s)
	case ClauseNotStartsWith:
		return db.Where(col+" NOT ILIKE ?", s+"%")
	case ClauseNotEndsWith:
		return db.Where(col+" NOT ILIKE ?", "%"+s)
	case ClauseEquals:
		return db.Where(col+" = ?", s)
	case ClauseNotEquals:
		return db.Where(col+" <> ?", s)
	}
	return db
}

func (f *TextFilter) Serialize() FilterSerialized {
	return f.baseSerialize(f.Type(), textDefaultClauses)
}

// ─── SetFilter ──────────────────────────────────────────────────────

var setDefaultClauses = []Clause{ClauseIn, ClauseNotIn, ClauseEquals, ClauseNotEquals}

type SetFilter struct {
	filterBase
	options  []FilterOption
	multiple bool
}

func NewSetFilter(attribute string, label ...string) *SetFilter {
	return &SetFilter{filterBase: newFilterBase(attribute, label...), multiple: true}
}

func (f *SetFilter) Type() string { return "set" }
func (f *SetFilter) Clauses() []Clause {
	if len(f.clauses) > 0 {
		return f.clauses
	}
	return setDefaultClauses
}

func (f *SetFilter) Options(opts []FilterOption) *SetFilter { f.options = opts; return f }
func (f *SetFilter) Single() *SetFilter                     { f.multiple = false; return f }
func (f *SetFilter) WithoutClause() *SetFilter {
	f.clauses = []Clause{ClauseEquals}
	return f
}

func (f *SetFilter) Validate(value any, clause Clause) (any, bool) {
	switch clause {
	case ClauseIn, ClauseNotIn:
		arr, ok := value.([]any)
		if !ok {
			// Sometimes URL parsing yields a single string for "in" — accept and wrap.
			if s, ok := value.(string); ok && s != "" {
				return []string{s}, true
			}
			if ss, ok := value.([]string); ok {
				return ss, len(ss) > 0
			}
			return nil, false
		}
		out := make([]string, 0, len(arr))
		for _, v := range arr {
			if s, ok := v.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out, len(out) > 0
	}
	s, ok := value.(string)
	if !ok || s == "" {
		return nil, false
	}
	return s, true
}

func (f *SetFilter) Apply(db *gorm.DB, attribute string, clause Clause, value any) *gorm.DB {
	if next, done := applyValueless(db, attribute, clause); done {
		return next
	}
	col := resolveAttribute(db, attribute)
	switch clause {
	case ClauseIn:
		return db.Where(col+" IN ?", value)
	case ClauseNotIn:
		return db.Where(col+" NOT IN ?", value)
	case ClauseEquals:
		return db.Where(col+" = ?", value)
	case ClauseNotEquals:
		return db.Where(col+" <> ?", value)
	}
	return db
}

func (f *SetFilter) Serialize() FilterSerialized {
	s := f.baseSerialize(f.Type(), setDefaultClauses)
	s.Options = f.options
	m := f.multiple
	s.Multiple = &m
	return s
}

// ─── DateFilter ─────────────────────────────────────────────────────

var dateDefaultClauses = []Clause{
	ClauseBefore, ClauseAfter, ClauseEqualOrBefore, ClauseEqualOrAfter,
	ClauseEquals, ClauseNotEquals, ClauseBetween, ClauseNotBetween,
}

type DateFilter struct{ filterBase }

func NewDateFilter(attribute string, label ...string) *DateFilter {
	return &DateFilter{filterBase: newFilterBase(attribute, label...)}
}

func (f *DateFilter) Type() string { return "date" }
func (f *DateFilter) Clauses() []Clause {
	if len(f.clauses) > 0 {
		return f.clauses
	}
	return dateDefaultClauses
}

func parseDateISO(value any) (string, bool) {
	s, ok := value.(string)
	if !ok {
		if t, ok := value.(time.Time); ok {
			return t.UTC().Format("2006-01-02"), true
		}
		return "", false
	}
	// Try a few common shapes; emit canonical YYYY-MM-DD.
	for _, layout := range []string{"2006-01-02", "2006-01-02T15:04:05Z07:00", time.RFC3339, time.RFC1123} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC().Format("2006-01-02"), true
		}
	}
	return "", false
}

func (f *DateFilter) Validate(value any, clause Clause) (any, bool) {
	if clause == ClauseBetween || clause == ClauseNotBetween {
		arr, ok := value.([]any)
		if !ok || len(arr) != 2 {
			return nil, false
		}
		a, ok1 := parseDateISO(arr[0])
		b, ok2 := parseDateISO(arr[1])
		if !ok1 || !ok2 {
			return nil, false
		}
		return []string{a, b}, true
	}
	s, ok := parseDateISO(value)
	if !ok {
		return nil, false
	}
	return s, true
}

func (f *DateFilter) Apply(db *gorm.DB, attribute string, clause Clause, value any) *gorm.DB {
	if next, done := applyValueless(db, attribute, clause); done {
		return next
	}
	col := resolveAttribute(db, attribute)
	switch clause {
	case ClauseEquals:
		return db.Where("DATE("+col+") = ?", value)
	case ClauseNotEquals:
		return db.Where("DATE("+col+") <> ?", value)
	case ClauseBefore:
		return db.Where(col+" < ?", value)
	case ClauseAfter:
		return db.Where(col+" > ?", value)
	case ClauseEqualOrBefore:
		return db.Where(col+" <= ?", value)
	case ClauseEqualOrAfter:
		return db.Where(col+" >= ?", value)
	case ClauseBetween, ClauseNotBetween:
		bounds, _ := value.([]string)
		if len(bounds) != 2 {
			return db
		}
		if clause == ClauseBetween {
			return db.Where(col+" BETWEEN ? AND ?", bounds[0], bounds[1])
		}
		return db.Where(col+" NOT BETWEEN ? AND ?", bounds[0], bounds[1])
	}
	return db
}

func (f *DateFilter) Serialize() FilterSerialized {
	return f.baseSerialize(f.Type(), dateDefaultClauses)
}

// ─── NumericFilter ──────────────────────────────────────────────────

var numericDefaultClauses = []Clause{
	ClauseEquals, ClauseNotEquals,
	ClauseGreaterThan, ClauseGreaterThanOrEqual,
	ClauseLessThan, ClauseLessThanOrEqual,
	ClauseBetween, ClauseNotBetween,
}

type NumericFilter struct{ filterBase }

func NewNumericFilter(attribute string, label ...string) *NumericFilter {
	return &NumericFilter{filterBase: newFilterBase(attribute, label...)}
}

func (f *NumericFilter) Type() string { return "numeric" }
func (f *NumericFilter) Clauses() []Clause {
	if len(f.clauses) > 0 {
		return f.clauses
	}
	return numericDefaultClauses
}

func toFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		n, err := strconv.ParseFloat(v, 64)
		return n, err == nil
	}
	return 0, false
}

func (f *NumericFilter) Validate(value any, clause Clause) (any, bool) {
	if clause == ClauseBetween || clause == ClauseNotBetween {
		arr, ok := value.([]any)
		if !ok || len(arr) != 2 {
			return nil, false
		}
		a, ok1 := toFloat(arr[0])
		b, ok2 := toFloat(arr[1])
		if !ok1 || !ok2 {
			return nil, false
		}
		return []float64{a, b}, true
	}
	n, ok := toFloat(value)
	if !ok {
		return nil, false
	}
	return n, true
}

func (f *NumericFilter) Apply(db *gorm.DB, attribute string, clause Clause, value any) *gorm.DB {
	if next, done := applyValueless(db, attribute, clause); done {
		return next
	}
	col := resolveAttribute(db, attribute)
	switch clause {
	case ClauseEquals:
		return db.Where(col+" = ?", value)
	case ClauseNotEquals:
		return db.Where(col+" <> ?", value)
	case ClauseGreaterThan:
		return db.Where(col+" > ?", value)
	case ClauseGreaterThanOrEqual:
		return db.Where(col+" >= ?", value)
	case ClauseLessThan:
		return db.Where(col+" < ?", value)
	case ClauseLessThanOrEqual:
		return db.Where(col+" <= ?", value)
	case ClauseBetween, ClauseNotBetween:
		bounds, _ := value.([]float64)
		if len(bounds) != 2 {
			return db
		}
		if clause == ClauseBetween {
			return db.Where(col+" BETWEEN ? AND ?", bounds[0], bounds[1])
		}
		return db.Where(col+" NOT BETWEEN ? AND ?", bounds[0], bounds[1])
	}
	return db
}

func (f *NumericFilter) Serialize() FilterSerialized {
	return f.baseSerialize(f.Type(), numericDefaultClauses)
}

// ─── BooleanFilter ──────────────────────────────────────────────────

var boolDefaultClauses = []Clause{ClauseIsTrue, ClauseIsFalse}

type BooleanFilter struct{ filterBase }

func NewBooleanFilter(attribute string, label ...string) *BooleanFilter {
	return &BooleanFilter{filterBase: newFilterBase(attribute, label...)}
}

func (f *BooleanFilter) Type() string                         { return "boolean" }
func (f *BooleanFilter) Clauses() []Clause                    { return boolDefaultClauses }
func (f *BooleanFilter) Validate(_ any, _ Clause) (any, bool) { return nil, true }

func (f *BooleanFilter) Apply(db *gorm.DB, attribute string, clause Clause, _ any) *gorm.DB {
	col := resolveAttribute(db, attribute)
	switch clause {
	case ClauseIsTrue:
		return db.Where(col+" = ?", true)
	case ClauseIsFalse:
		return db.Where(col+" = ?", false)
	}
	return db
}

func (f *BooleanFilter) Serialize() FilterSerialized {
	return f.baseSerialize(f.Type(), boolDefaultClauses)
}

// ─── TrashedFilter ──────────────────────────────────────────────────

// TrashedFilter handles soft-delete inclusion. Auto-injected by Render when
// the table opts into SoftDeletes; consumers can also add it manually with
// a custom label.
type TrashedFilter struct{ filterBase }

func NewTrashedFilter() *TrashedFilter {
	f := &TrashedFilter{filterBase: newFilterBase("trashed", "Trashed")}
	f.clauses = []Clause{ClauseEquals}
	return f
}

func (f *TrashedFilter) Type() string      { return "set" }
func (f *TrashedFilter) Clauses() []Clause { return f.clauses }
func (f *TrashedFilter) Validate(value any, _ Clause) (any, bool) {
	s, ok := value.(string)
	if !ok {
		return nil, false
	}
	switch s {
	case "with_trashed", "only_trashed", "without_trashed", "all":
		return s, true
	}
	return nil, false
}

func (f *TrashedFilter) Apply(db *gorm.DB, _ string, _ Clause, value any) *gorm.DB {
	s, _ := value.(string)
	switch s {
	case "with_trashed", "all":
		return db.Unscoped()
	case "only_trashed":
		return db.Unscoped().Where("deleted_at IS NOT NULL")
	}
	// "without_trashed" → default GORM behaviour, no change.
	return db
}

func (f *TrashedFilter) Serialize() FilterSerialized {
	s := f.baseSerialize(f.Type(), f.clauses)
	s.Options = []FilterOption{
		{Value: "without_trashed", Label: "Without trashed"},
		{Value: "with_trashed", Label: "With trashed"},
		{Value: "only_trashed", Label: "Only trashed"},
	}
	mult := false
	s.Multiple = &mult
	return s
}
