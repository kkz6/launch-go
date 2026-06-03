package table

import (
	"fmt"
	"strings"
	"time"
)

// Column is the interface every column variant satisfies. The base struct
// below implements most of it; variants embed Base and override Type +
// MapValue / Serialize where they extend the schema.
type Column interface {
	Attribute() string
	Header() string
	Type() string
	Sortable() bool
	Searchable() bool
	Toggleable() bool
	Visible() bool
	Stickable() bool
	IsNested() bool
	RelationshipName() string
	RelationshipColumn() string
	GetDataFrom(item any) any
	MapForTable(value any, item any) any
	Serialize() ColumnSerialized
}

// Base is the shared column state — embed it into each variant. Public
// fluent methods live on the variant types so the chainable builders return
// the correct concrete pointer.
type Base struct {
	attribute   string
	header      string
	sortable    bool
	toggleable  bool
	searchable  bool
	visible     bool
	alignment   Alignment
	wrap        bool
	truncate    any
	headerClass *string
	cellClass   *string
	stickable   bool
	meta        map[string]any
}

func newBase(attribute string, header ...string) Base {
	h := ""
	if len(header) > 0 && header[0] != "" {
		h = header[0]
	} else {
		h = humanize(attribute)
	}
	return Base{
		attribute:  attribute,
		header:     h,
		toggleable: true,
		visible:    true,
		alignment:  AlignmentLeft,
		truncate:   false,
	}
}

func (b *Base) Attribute() string    { return b.attribute }
func (b *Base) Header() string       { return b.header }
func (b *Base) Sortable() bool       { return b.sortable }
func (b *Base) Searchable() bool     { return b.searchable }
func (b *Base) Toggleable() bool     { return b.toggleable }
func (b *Base) Visible() bool        { return !b.toggleable || b.visible }
func (b *Base) Stickable() bool      { return b.stickable }
func (b *Base) Alignment() Alignment { return b.alignment }

func (b *Base) IsNested() bool {
	return strings.Contains(b.attribute, ".") && !strings.HasPrefix(b.attribute, "pivot.")
}

func (b *Base) RelationshipName() string {
	if i := strings.LastIndex(b.attribute, "."); i >= 0 {
		return b.attribute[:i]
	}
	return b.attribute
}

func (b *Base) RelationshipColumn() string {
	if i := strings.LastIndex(b.attribute, "."); i >= 0 {
		return b.attribute[i+1:]
	}
	return b.attribute
}

// GetDataFrom walks the attribute path on the supplied row. Rows are the
// map[string]any shapes the query service produces, so dotted attributes
// resolve through nested maps.
func (b *Base) GetDataFrom(item any) any {
	parts := strings.Split(b.attribute, ".")
	current := item
	for _, p := range parts {
		if current == nil {
			return nil
		}
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = m[p]
	}
	return current
}

// MapForTable is the default identity mapping; variants override it.
func (b *Base) MapForTable(value any, _ any) any { return value }

func (b *Base) baseSerialize(typeName string) ColumnSerialized {
	return ColumnSerialized{
		Type:        typeName,
		Key:         b.attribute,
		Header:      b.header,
		Sortable:    b.sortable,
		Searchable:  b.searchable,
		Toggleable:  b.toggleable,
		Visible:     b.Visible(),
		Alignment:   b.alignment,
		Wrap:        b.wrap,
		Truncate:    b.truncate,
		HeaderClass: b.headerClass,
		CellClass:   b.cellClass,
		Stickable:   b.stickable,
		Meta:        b.meta,
	}
}

// ─── TextColumn ─────────────────────────────────────────────────────

type TextColumn struct {
	Base
}

// NewTextColumn — header is optional, defaults to a humanized attribute.
func NewTextColumn(attribute string, header ...string) *TextColumn {
	return &TextColumn{Base: newBase(attribute, header...)}
}

func (c *TextColumn) Type() string                  { return "text" }
func (c *TextColumn) AsSortable() *TextColumn       { c.sortable = true; return c }
func (c *TextColumn) AsSearchable() *TextColumn     { c.searchable = true; return c }
func (c *TextColumn) NotToggleable() *TextColumn    { c.toggleable = false; return c }
func (c *TextColumn) Align(a Alignment) *TextColumn { c.alignment = a; return c }
func (c *TextColumn) Serialize() ColumnSerialized   { return c.baseSerialize(c.Type()) }

// ─── DateColumn / DateTimeColumn ────────────────────────────────────

type DateColumn struct {
	Base
	format   string
	withTime bool
}

// NewDateColumn produces a date-only column (default format YYYY-MM-DD).
func NewDateColumn(attribute string, header ...string) *DateColumn {
	return &DateColumn{Base: newBase(attribute, header...), format: "2006-01-02"}
}

// NewDateTimeColumn produces a date+time column (default RFC3339).
func NewDateTimeColumn(attribute string, header ...string) *DateColumn {
	return &DateColumn{Base: newBase(attribute, header...), format: "2006-01-02 15:04:05", withTime: true}
}

func (c *DateColumn) Type() string {
	if c.withTime {
		return "datetime"
	}
	return "date"
}

// Format takes a Go time layout (e.g., "2006-01-02"). For convenience the
// frontend doesn't need to know about Go layouts — it formats the ISO
// string we send. The Format we store is reflected in the meta payload
// for clients that want to render the same shape on the wire.
func (c *DateColumn) Format(layout string) *DateColumn {
	c.format = layout
	return c
}

func (c *DateColumn) AsSortable() *DateColumn { c.sortable = true; return c }

func (c *DateColumn) MapForTable(value any, _ any) any {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case time.Time:
		if v.IsZero() {
			return nil
		}
		return v.Format(c.format)
	case *time.Time:
		if v == nil || v.IsZero() {
			return nil
		}
		return v.Format(c.format)
	case string:
		// Values that traveled through JSON marshal arrive as ISO strings;
		// reparse so the column's Format() actually applies.
		if v == "" {
			return nil
		}
		for _, layout := range []string{
			time.RFC3339Nano, time.RFC3339,
			"2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02",
		} {
			if t, err := time.Parse(layout, v); err == nil {
				return t.Format(c.format)
			}
		}
		return v
	default:
		return fmt.Sprint(v)
	}
}

func (c *DateColumn) Serialize() ColumnSerialized {
	s := c.baseSerialize(c.Type())
	s.Format = c.format
	return s
}

// ─── NumericColumn ──────────────────────────────────────────────────

type NumericColumn struct {
	Base
}

func NewNumericColumn(attribute string, header ...string) *NumericColumn {
	c := &NumericColumn{Base: newBase(attribute, header...)}
	c.alignment = AlignmentRight
	return c
}

func (c *NumericColumn) Type() string                { return "numeric" }
func (c *NumericColumn) AsSortable() *NumericColumn  { c.sortable = true; return c }
func (c *NumericColumn) Serialize() ColumnSerialized { return c.baseSerialize(c.Type()) }

// ─── BooleanColumn ──────────────────────────────────────────────────

type BooleanColumn struct {
	Base
	trueLabel  string
	falseLabel string
	trueIcon   *string
	falseIcon  *string
}

func NewBooleanColumn(attribute string, header ...string) *BooleanColumn {
	return &BooleanColumn{
		Base:       newBase(attribute, header...),
		trueLabel:  "Yes",
		falseLabel: "No",
	}
}

func (c *BooleanColumn) Type() string               { return "boolean" }
func (c *BooleanColumn) AsSortable() *BooleanColumn { c.sortable = true; return c }

func (c *BooleanColumn) TrueLabel(s string) *BooleanColumn  { c.trueLabel = s; return c }
func (c *BooleanColumn) FalseLabel(s string) *BooleanColumn { c.falseLabel = s; return c }
func (c *BooleanColumn) TrueIcon(s string) *BooleanColumn   { c.trueIcon = &s; return c }
func (c *BooleanColumn) FalseIcon(s string) *BooleanColumn  { c.falseIcon = &s; return c }

func (c *BooleanColumn) MapForTable(value any, _ any) any {
	v, _ := value.(bool)
	if v {
		return c.trueLabel
	}
	return c.falseLabel
}

func (c *BooleanColumn) Serialize() ColumnSerialized {
	s := c.baseSerialize(c.Type())
	s.TrueLabel = &c.trueLabel
	s.FalseLabel = &c.falseLabel
	s.TrueIcon = c.trueIcon
	s.FalseIcon = c.falseIcon
	return s
}

// ─── BadgeColumn ────────────────────────────────────────────────────

type BadgeColumn struct {
	Base
	variants map[string]Variant
}

// NewBadgeColumn renders the value as a coloured pill. variants maps
// raw values → visual variant.
func NewBadgeColumn(attribute string, header ...string) *BadgeColumn {
	return &BadgeColumn{Base: newBase(attribute, header...)}
}

func (c *BadgeColumn) Type() string             { return "badge" }
func (c *BadgeColumn) AsSortable() *BadgeColumn { c.sortable = true; return c }

func (c *BadgeColumn) Variants(m map[string]Variant) *BadgeColumn {
	c.variants = m
	return c
}

func (c *BadgeColumn) MapForTable(value any, _ any) any {
	v := fmt.Sprint(value)
	var variant Variant
	if c.variants != nil {
		variant = c.variants[v]
	}
	return map[string]any{"value": v, "variant": string(variant)}
}

func (c *BadgeColumn) Serialize() ColumnSerialized {
	s := c.baseSerialize(c.Type())
	if c.variants != nil {
		// Re-shape into a string map for JSON friendliness.
		m := make(map[string]string, len(c.variants))
		for k, v := range c.variants {
			m[k] = string(v)
		}
		s.Meta = map[string]any{"variants": m}
	}
	return s
}

// ─── ImageColumn ────────────────────────────────────────────────────

type ImageColumn struct {
	Base
	imageSize     ImageSize
	imagePosition ImagePosition
	fallback      *string
	rounded       bool
}

func NewImageColumn(attribute string, header ...string) *ImageColumn {
	return &ImageColumn{
		Base:          newBase(attribute, header...),
		imageSize:     ImageMedium,
		imagePosition: ImagePositionStart,
	}
}

func (c *ImageColumn) Type() string                          { return "image" }
func (c *ImageColumn) Size(s ImageSize) *ImageColumn         { c.imageSize = s; return c }
func (c *ImageColumn) Position(p ImagePosition) *ImageColumn { c.imagePosition = p; return c }
func (c *ImageColumn) Fallback(url string) *ImageColumn      { c.fallback = &url; return c }
func (c *ImageColumn) Rounded() *ImageColumn                 { c.rounded = true; return c }

func (c *ImageColumn) Serialize() ColumnSerialized {
	s := c.baseSerialize(c.Type())
	s.ImageSize = c.imageSize
	s.ImagePosition = c.imagePosition
	s.FallbackImage = c.fallback
	s.Rounded = c.rounded
	return s
}

// ─── ActionColumn ───────────────────────────────────────────────────

type ActionColumn struct {
	Base
	asDropdown bool
}

// NewActionColumn pins the column key to "_actions" and right-aligns it.
func NewActionColumn(header ...string) *ActionColumn {
	c := &ActionColumn{Base: newBase("_actions", header...)}
	c.alignment = AlignmentRight
	c.toggleable = false
	return c
}

func (c *ActionColumn) Type() string              { return "action" }
func (c *ActionColumn) AsDropdown() *ActionColumn { c.asDropdown = true; return c }
func (c *ActionColumn) Attribute() string         { return "_actions" }

func (c *ActionColumn) Serialize() ColumnSerialized {
	s := c.baseSerialize(c.Type())
	s.AsDropdown = c.asDropdown
	return s
}
