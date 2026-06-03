package table

// ColumnSerialized is what the meta endpoint emits for one column. The
// frontend's CellRenderer branches on `type` to pick a renderer; everything
// else is forwarded straight to the cell components.
type ColumnSerialized struct {
	Type        string    `json:"type"`
	Key         string    `json:"key"`
	Header      string    `json:"header"`
	Sortable    bool      `json:"sortable"`
	Searchable  bool      `json:"searchable"`
	Toggleable  bool      `json:"toggleable"`
	Visible     bool      `json:"visible"`
	Alignment   Alignment `json:"alignment"`
	Wrap        bool      `json:"wrap"`
	Truncate    any       `json:"truncate"` // int chars or false
	HeaderClass *string   `json:"headerClass,omitempty"`
	CellClass   *string   `json:"cellClass,omitempty"`
	Stickable   bool      `json:"stickable"`
	Meta        any       `json:"meta,omitempty"`

	// BooleanColumn extras
	TrueIcon   *string `json:"trueIcon,omitempty"`
	FalseIcon  *string `json:"falseIcon,omitempty"`
	TrueLabel  *string `json:"trueLabel,omitempty"`
	FalseLabel *string `json:"falseLabel,omitempty"`

	// ImageColumn extras
	ImageSize     ImageSize     `json:"imageSize,omitempty"`
	ImagePosition ImagePosition `json:"imagePosition,omitempty"`
	FallbackImage *string       `json:"fallbackImage,omitempty"`
	Rounded       bool          `json:"rounded,omitempty"`

	// ActionColumn extras
	AsDropdown bool `json:"asDropdown,omitempty"`

	// DateColumn / DateTimeColumn extras
	Format string `json:"format,omitempty"`
}

// FilterOption is one item in a SetFilter's options list.
type FilterOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// FilterDefault is the default {value, clause} a filter is pre-populated with.
type FilterDefault struct {
	Value  any    `json:"value"`
	Clause Clause `json:"clause"`
}

// FilterSerialized is one filter as emitted in the meta payload.
type FilterSerialized struct {
	Key      string         `json:"key"`
	Label    string         `json:"label"`
	Type     string         `json:"type"`
	Clauses  []Clause       `json:"clauses"`
	Options  []FilterOption `json:"options,omitempty"`
	Multiple *bool          `json:"multiple,omitempty"`
	Hidden   bool           `json:"hidden,omitempty"`
	Default  *FilterDefault `json:"default"`
}

// ActionConfirm describes the optional confirmation dialog shown before an
// action runs.
type ActionConfirm struct {
	Title        string `json:"title"`
	Message      string `json:"message,omitempty"`
	ConfirmLabel string `json:"confirmLabel,omitempty"`
	CancelLabel  string `json:"cancelLabel,omitempty"`
}

// ActionSerialized is one action in the meta payload. On row-level payloads
// the server may also write per-row Disabled / Hidden / URL.
type ActionSerialized struct {
	Name           string            `json:"name"`
	Label          string            `json:"label"`
	Type           ActionType        `json:"type"`
	Variant        Variant           `json:"variant"`
	Icon           *string           `json:"icon,omitempty"`
	Tooltip        *string           `json:"tooltip,omitempty"`
	Confirm        *ActionConfirm    `json:"confirm,omitempty"`
	Disabled       bool              `json:"disabled,omitempty"`
	Hidden         bool              `json:"hidden,omitempty"`
	URL            *string           `json:"url,omitempty"`
	Download       bool              `json:"download,omitempty"`
	Meta           map[string]any    `json:"meta,omitempty"`
	DataAttributes map[string]string `json:"dataAttributes,omitempty"`
}

// EmptyStateSerialized is rendered when the table has zero rows.
type EmptyStateSerialized struct {
	Title   string            `json:"title"`
	Message string            `json:"message,omitempty"`
	Icon    string            `json:"icon,omitempty"`
	Action  *EmptyStateAction `json:"action,omitempty"`
}

type EmptyStateAction struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// ViewSerialized is a saved filter/sort/column preset.
type ViewSerialized struct {
	ID             string         `json:"id"`
	Title          string         `json:"title"`
	RequestPayload map[string]any `json:"requestPayload"`
}

// TableMeta is the full schema the frontend uses to render the toolbar,
// columns, and action surface.
type TableMeta struct {
	Columns        []ColumnSerialized    `json:"columns"`
	Filters        []FilterSerialized    `json:"filters"`
	Actions        TableActions          `json:"actions"`
	Search         TableSearchMeta       `json:"search"`
	PerPageOptions []int                 `json:"perPageOptions"`
	SoftDeletes    bool                  `json:"softDeletes"`
	StickyHeader   bool                  `json:"stickyHeader"`
	Debounce       int                   `json:"debounce"`
	ScrollPosition ScrollPosition        `json:"scrollPosition"`
	Views          []ViewSerialized      `json:"views"`
	EmptyState     *EmptyStateSerialized `json:"emptyState"`
	Exports        []any                 `json:"exports"` // reserved
}

type TableActions struct {
	Row  []ActionSerialized `json:"row"`
	Bulk []ActionSerialized `json:"bulk"`
}

type TableSearchMeta struct {
	Enabled     bool   `json:"enabled"`
	Placeholder string `json:"placeholder"`
}

// PaginationData is the per-request pagination block.
type PaginationData struct {
	Type           PaginationType `json:"type"`
	CurrentPage    int            `json:"currentPage"`
	LastPage       int            `json:"lastPage"`
	PerPage        int            `json:"perPage"`
	Total          int64          `json:"total"`
	From           int64          `json:"from"`
	To             int64          `json:"to"`
	NextCursor     *string        `json:"nextCursor,omitempty"`
	PreviousCursor *string        `json:"previousCursor,omitempty"`
}

// TableResponse is the full /data response: meta is included on every
// request so the frontend doesn't need a separate round-trip after a refresh.
type TableResponse struct {
	Meta       TableMeta        `json:"meta"`
	Data       []map[string]any `json:"data"`
	Pagination PaginationData   `json:"pagination"`
}
