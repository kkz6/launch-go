// Package access holds the auth-specific layer of the authorization gate: the
// Actor (the authenticated subject), the TeamGate helper that encodes
// team-membership + role-hierarchy rules, the Policy adapter that lets module
// policies receive a typed Actor, and global Before hooks.
//
// The generic engine lives in internal/pkg/access; this package supplies the
// domain knowledge (teams, roles) that the engine deliberately omits.
package access

import (
	"context"
	"strings"

	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	"github.com/kkz6/launch-go/internal/pkg/access"
)

// Actor is the authenticated subject of an authorization check, assembled from
// request context at the edge. It is plain data with no transport dependency.
type Actor struct {
	UserID   string
	TeamID   string
	TeamRole authtypes.TeamRole // "" when the request carries no team scope
	IsStaff  bool               // platform staff (admin panel)
	ReadOnly bool               // read-only impersonation ("spectate")
}

// TypedPolicy is a policy written against a concrete Actor. Module policies use
// this signature and wrap with Policy when registering, so they never assert
// the actor type themselves.
type TypedPolicy func(ctx context.Context, actor Actor, resource any) access.Response

// Policy adapts a TypedPolicy into the engine's PolicyFunc, asserting the
// opaque actor to a concrete Actor. A wrong actor type is a programming error
// and panics — actors are always built via this package.
func Policy(fn TypedPolicy) access.PolicyFunc {
	return func(ctx context.Context, actor any, resource any) access.Response {
		return fn(ctx, actor.(Actor), resource)
	}
}

// TeamGate is the workhorse policy helper: it permits the action when the actor
// belongs to the resource's team AND holds at least the minimum role. It is the
// single source of truth for team + role authorization, replacing ad-hoc checks
// scattered across handlers and middleware.
func TeamGate(a Actor, resourceTeamID string, minRole authtypes.TeamRole) access.Response {
	if a.TeamID == "" || a.TeamID != resourceTeamID {
		return access.Deny("resource does not belong to your team")
	}
	if !a.TeamRole.AtLeast(minRole) {
		return access.Deny("your team role lacks permission for this action")
	}
	return access.Allow()
}

// RequireRole returns a policy that permits the action when the actor holds at
// least the given team role. It deliberately ignores the resource: team
// ownership of the resource is already guaranteed upstream (TeamScope validates
// membership in the scoped team, and services load resources via
// FindByIDAndTeam), so the gate's remaining job is the per-action role check.
// This is the workhorse policy for the Can(ability) route middleware.
func RequireRole(minRole authtypes.TeamRole) TypedPolicy {
	return func(_ context.Context, a Actor, _ any) access.Response {
		if a.TeamRole.AtLeast(minRole) {
			return access.Allow()
		}
		return access.Deny("your team role (" + a.TeamRole.Label() + ") lacks permission for this action")
	}
}

// readVerbs are ability suffixes that only observe state and are therefore
// permitted under read-only impersonation.
var readVerbs = map[string]bool{
	"view": true, "list": true, "show": true, "index": true, "get": true,
}

// isMutating reports whether an ability ("resource.verb") changes state.
func isMutating(ability string) bool {
	verb := ability
	if i := strings.LastIndex(ability, "."); i >= 0 {
		verb = ability[i+1:]
	}
	return !readVerbs[verb]
}

// ReadOnlyFreeze is a gate Before hook that denies any state-changing ability
// for a read-only (spectate) actor. Read abilities fall through to the normal
// policy. This is defense-in-depth: the auth middleware already blocks mutating
// requests under read-only tokens at the chokepoint.
func ReadOnlyFreeze(_ context.Context, actor any, ability string) (access.Response, bool) {
	a, ok := actor.(Actor)
	if !ok || !a.ReadOnly {
		return access.Response{}, false
	}
	if isMutating(ability) {
		return access.Deny("read-only session cannot modify resources"), true
	}
	return access.Response{}, false
}
