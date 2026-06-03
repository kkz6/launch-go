package table

import "context"

// actorIDKey is the unexported context key under which the action route stashes
// the authenticated user's id before invoking an action handler.
type actorIDKey struct{}

// WithActorID returns a context carrying the acting user's id, set by the
// action route before invoking an action handler.
func WithActorID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, actorIDKey{}, id)
}

// ActorID returns the acting user's id from the context (empty if unset).
func ActorID(ctx context.Context) string {
	v, _ := ctx.Value(actorIDKey{}).(string)
	return v
}
