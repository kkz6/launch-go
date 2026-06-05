package access

import (
	"context"
	"errors"
	"fmt"
)

// PolicyFunc decides a single ability for an actor against a resource. The
// resource may be nil for abilities with no instance (e.g. "server.create").
// Actor and resource are opaque; policies type-assert to their concrete types.
type PolicyFunc func(ctx context.Context, actor any, resource any) Response

// BeforeFunc runs ahead of the ability check. Returning handled=true
// short-circuits the decision with the returned Response (used for global rules
// like staff bypass or read-only freeze); handled=false falls through to the
// registered ability.
type BeforeFunc func(ctx context.Context, actor any, ability string) (resp Response, handled bool)

// Gate maps abilities to policies and evaluates authorization decisions. A
// single Gate is built at bootstrap and shared; module policies register into
// it via Define. The zero value is not usable — construct with New.
type Gate struct {
	abilities map[string]PolicyFunc
	before    []BeforeFunc
}

// New returns an empty Gate ready for policy registration.
func New() *Gate {
	return &Gate{abilities: make(map[string]PolicyFunc)}
}

// Define registers the policy for an ability. A later Define for the same
// ability replaces the earlier one.
func (g *Gate) Define(ability string, fn PolicyFunc) {
	g.abilities[ability] = fn
}

// Before registers a hook evaluated before any ability. Hooks run in
// registration order; the first to return handled=true wins.
func (g *Gate) Before(fn BeforeFunc) {
	g.before = append(g.before, fn)
}

// Inspect evaluates the decision and returns the full Response, including the
// denial reason. Before-hooks run first; an unknown ability is denied.
func (g *Gate) Inspect(ctx context.Context, actor any, ability string, resource any) Response {
	for _, hook := range g.before {
		if resp, handled := hook(ctx, actor, ability); handled {
			return resp
		}
	}

	fn, ok := g.abilities[ability]
	if !ok {
		return Deny(fmt.Sprintf("unknown ability: %s", ability))
	}

	return fn(ctx, actor, resource)
}

// Allows reports whether the action is permitted.
func (g *Gate) Allows(ctx context.Context, actor any, ability string, resource any) bool {
	return g.Inspect(ctx, actor, ability, resource).Allowed()
}

// Denies reports whether the action is refused.
func (g *Gate) Denies(ctx context.Context, actor any, ability string, resource any) bool {
	return !g.Allows(ctx, actor, ability, resource)
}

// Authorize returns nil when permitted, or a *DeniedError carrying the policy's
// denial reason when refused. Handlers map this to a 403 response.
func (g *Gate) Authorize(ctx context.Context, actor any, ability string, resource any) error {
	resp := g.Inspect(ctx, actor, ability, resource)
	if resp.Allowed() {
		return nil
	}
	return &DeniedError{Ability: ability, Reason: resp.Message()}
}

// DeniedError is returned by Authorize when a policy refuses an action.
type DeniedError struct {
	Ability string
	Reason  string
}

func (e *DeniedError) Error() string { return e.Reason }

// IsDenied reports whether err is an authorization denial from Authorize.
func IsDenied(err error) bool {
	var denied *DeniedError
	return errors.As(err, &denied)
}
