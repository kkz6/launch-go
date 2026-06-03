package table

// EmptyState is rendered when the result set is empty. Fluent so callers can
// chain title/message/icon/action declaratively.
type EmptyState struct {
	title       string
	message     string
	icon        string
	actionLabel string
	actionURL   string
}

func NewEmptyState() *EmptyState { return &EmptyState{} }

func (e *EmptyState) Title(s string) *EmptyState   { e.title = s; return e }
func (e *EmptyState) Message(s string) *EmptyState { e.message = s; return e }
func (e *EmptyState) Icon(s string) *EmptyState    { e.icon = s; return e }
func (e *EmptyState) Action(label, url string) *EmptyState {
	e.actionLabel, e.actionURL = label, url
	return e
}

func (e *EmptyState) Serialize() EmptyStateSerialized {
	out := EmptyStateSerialized{Title: e.title, Message: e.message, Icon: e.icon}
	if e.actionLabel != "" {
		out.Action = &EmptyStateAction{Label: e.actionLabel, URL: e.actionURL}
	}
	return out
}
