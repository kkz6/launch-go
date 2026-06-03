package table

import (
	"context"
	"strings"
)

// ActionHandler runs server-side when the frontend posts to the action
// endpoint. For row actions it receives one id; for bulk actions a slice.
type ActionHandler func(ctx context.Context, ids []string) error

// URLResolver builds an href for an "as link" action from a row map.
type URLResolver func(row map[string]any) string

// HiddenFn / DisabledFn evaluate per row when transforming response items.
type HiddenFn func(row map[string]any) bool
type DisabledFn func(row map[string]any) bool

// Action is the builder for a row or bulk action. The frontend renders
// buttons / links from the meta payload; when the user invokes one, the
// posted name routes back to the handler attached here.
type Action struct {
	name           string
	label          string
	actionType     ActionType
	variant        Variant
	icon           *string
	tooltip        *string
	confirm        *ActionConfirm
	isBulk         bool
	download       bool
	meta           map[string]any
	dataAttributes map[string]string
	urlResolver    URLResolver
	hiddenFn       HiddenFn
	disabledFn     DisabledFn
	handler        ActionHandler
}

// NewAction starts the builder with a name and human label. Label defaults
// to the capitalised name when omitted.
func NewAction(name string, label ...string) *Action {
	l := ""
	if len(label) > 0 {
		l = label[0]
	} else if name != "" {
		l = strings.ToUpper(name[:1]) + name[1:]
	}
	return &Action{
		name:       name,
		label:      l,
		actionType: ActionTypeButton,
		variant:    VariantDefault,
	}
}

func (a *Action) AsButton() *Action               { a.actionType = ActionTypeButton; return a }
func (a *Action) AsLink() *Action                 { a.actionType = ActionTypeLink; return a }
func (a *Action) Variant(v Variant) *Action       { a.variant = v; return a }
func (a *Action) Icon(s string) *Action           { a.icon = &s; return a }
func (a *Action) Tooltip(s string) *Action        { a.tooltip = &s; return a }
func (a *Action) Confirm(c ActionConfirm) *Action { a.confirm = &c; return a }
func (a *Action) Bulk() *Action                   { a.isBulk = true; return a }
func (a *Action) Download() *Action               { a.download = true; return a }
func (a *Action) Meta(m map[string]any) *Action   { a.meta = m; return a }
func (a *Action) DataAttributes(m map[string]string) *Action {
	a.dataAttributes = m
	return a
}
func (a *Action) URL(r URLResolver) *Action       { a.urlResolver = r; return a }
func (a *Action) Hidden(fn HiddenFn) *Action      { a.hiddenFn = fn; return a }
func (a *Action) Disabled(fn DisabledFn) *Action  { a.disabledFn = fn; return a }
func (a *Action) Handle(fn ActionHandler) *Action { a.handler = fn; return a }

func (a *Action) Name() string  { return a.name }
func (a *Action) Label() string { return a.label }
func (a *Action) IsBulk() bool  { return a.isBulk }
func (a *Action) ResolveURL(row map[string]any) *string {
	if a.urlResolver == nil {
		return nil
	}
	s := a.urlResolver(row)
	return &s
}
func (a *Action) IsHiddenFor(row map[string]any) bool {
	if a.hiddenFn == nil {
		return false
	}
	return a.hiddenFn(row)
}
func (a *Action) IsDisabledFor(row map[string]any) bool {
	if a.disabledFn == nil {
		return false
	}
	return a.disabledFn(row)
}
func (a *Action) Handler() ActionHandler { return a.handler }

func (a *Action) Serialize() ActionSerialized {
	return ActionSerialized{
		Name:           a.name,
		Label:          a.label,
		Type:           a.actionType,
		Variant:        a.variant,
		Icon:           a.icon,
		Tooltip:        a.tooltip,
		Confirm:        a.confirm,
		Download:       a.download,
		Meta:           a.meta,
		DataAttributes: a.dataAttributes,
	}
}
