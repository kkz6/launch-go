// Package table is a declarative, GORM-backed table layer modelled on
// scouted's NestJS platform/table module. A consumer defines a struct that
// implements the Table interface (or builds one with TableBuilder) and the
// HTTP helper wires meta, data, action, and view endpoints.
//
// Ported feature set: columns, filters, sort, pagination, global search,
// per-row + bulk actions, saved views. Excluded for now: exports, SSE,
// nested-attribute eager loading.
package table

// Clause is a comparison verb a Filter understands. The string values mirror
// the scouted TypeScript enum so the frontend code can travel unchanged.
type Clause string

const (
	// Text clauses
	ClauseEquals        Clause = "equals"
	ClauseNotEquals     Clause = "not_equals"
	ClauseStartsWith    Clause = "starts_with"
	ClauseEndsWith      Clause = "ends_with"
	ClauseNotStartsWith Clause = "not_starts_with"
	ClauseNotEndsWith   Clause = "not_ends_with"
	ClauseContains      Clause = "contains"
	ClauseNotContains   Clause = "not_contains"

	// Boolean clauses
	ClauseIsTrue   Clause = "is_true"
	ClauseIsFalse  Clause = "is_false"
	ClauseIsSet    Clause = "is_set"
	ClauseIsNotSet Clause = "is_not_set"

	// Date clauses
	ClauseBefore        Clause = "before"
	ClauseEqualOrBefore Clause = "equal_or_before"
	ClauseAfter         Clause = "after"
	ClauseEqualOrAfter  Clause = "equal_or_after"
	ClauseBetween       Clause = "between"
	ClauseNotBetween    Clause = "not_between"

	// Numeric clauses
	ClauseGreaterThan        Clause = "greater_than"
	ClauseGreaterThanOrEqual Clause = "greater_than_or_equal"
	ClauseLessThan           Clause = "less_than"
	ClauseLessThanOrEqual    Clause = "less_than_or_equal"

	// Set clauses
	ClauseIn    Clause = "in"
	ClauseNotIn Clause = "not_in"

	// Trashed (soft-delete) pseudo-clauses
	ClauseWithTrashed    Clause = "with_trashed"
	ClauseOnlyTrashed    Clause = "only_trashed"
	ClauseWithoutTrashed Clause = "without_trashed"
)

// IsValueless reports whether the clause is satisfied without a comparison value.
func (c Clause) IsValueless() bool {
	switch c {
	case ClauseIsTrue, ClauseIsFalse, ClauseIsSet, ClauseIsNotSet,
		ClauseWithTrashed, ClauseOnlyTrashed, ClauseWithoutTrashed:
		return true
	}
	return false
}

// Alignment controls cell + header text alignment in the rendered table.
type Alignment string

const (
	AlignmentLeft   Alignment = "left"
	AlignmentCenter Alignment = "center"
	AlignmentRight  Alignment = "right"
)

// Variant maps to the shadcn-vue badge/button variant set so the frontend
// can render row/bulk actions in a consistent palette.
type Variant string

const (
	VariantDefault     Variant = "default"
	VariantInfo        Variant = "info"
	VariantSuccess     Variant = "success"
	VariantWarning     Variant = "warning"
	VariantDestructive Variant = "destructive"
	VariantSecondary   Variant = "secondary"
	VariantOutline     Variant = "outline"
	VariantGhost       Variant = "ghost"
	VariantLink        Variant = "link"
)

// ActionType picks the visual representation: a button that submits to the
// action endpoint, or an anchor with a resolved URL.
type ActionType string

const (
	ActionTypeButton ActionType = "button"
	ActionTypeLink   ActionType = "link"
)

// SortDirection is "asc" or "desc".
type SortDirection string

const (
	SortAsc  SortDirection = "asc"
	SortDesc SortDirection = "desc"
)

// PaginationType decides how the response pagination block is shaped.
type PaginationType string

const (
	PaginationFull   PaginationType = "full"
	PaginationSimple PaginationType = "simple"
	PaginationCursor PaginationType = "cursor"
)

// ScrollPosition tells the frontend where to scroll after a page change.
type ScrollPosition string

const (
	ScrollTopOfPage  ScrollPosition = "topOfPage"
	ScrollTopOfTable ScrollPosition = "topOfTable"
	ScrollPreserve   ScrollPosition = "preserve"
)

// ImageSize and ImagePosition are passed to the frontend ImageCell renderer.
type ImageSize string

const (
	ImageSmall      ImageSize = "small"
	ImageMedium     ImageSize = "medium"
	ImageLarge      ImageSize = "large"
	ImageExtraLarge ImageSize = "extra-large"
	ImageCustom     ImageSize = "custom"
)

type ImagePosition string

const (
	ImagePositionStart ImagePosition = "start"
	ImagePositionEnd   ImagePosition = "end"
)
