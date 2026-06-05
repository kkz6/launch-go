package access_test

import (
	"context"
	"testing"

	authaccess "github.com/kkz6/launch-go/internal/modules/auth/access"
	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	"github.com/kkz6/launch-go/internal/pkg/access"
)

func TestTeamGate(t *testing.T) {
	const team = "team-1"
	cases := []struct {
		name      string
		actor     authaccess.Actor
		resTeam   string
		min       authtypes.TeamRole
		wantAllow bool
		wantInMsg string
	}{
		{"same team sufficient role", authaccess.Actor{TeamID: team, TeamRole: authtypes.TeamRoleAdmin}, team, authtypes.TeamRoleEditor, true, ""},
		{"same team exact role", authaccess.Actor{TeamID: team, TeamRole: authtypes.TeamRoleEditor}, team, authtypes.TeamRoleEditor, true, ""},
		{"same team insufficient role", authaccess.Actor{TeamID: team, TeamRole: authtypes.TeamRoleMember}, team, authtypes.TeamRoleEditor, false, "role"},
		{"different team", authaccess.Actor{TeamID: "other", TeamRole: authtypes.TeamRoleOwner}, team, authtypes.TeamRoleMember, false, "team"},
		{"no team scope", authaccess.Actor{TeamID: "", TeamRole: authtypes.TeamRoleOwner}, team, authtypes.TeamRoleMember, false, "team"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp := authaccess.TeamGate(c.actor, c.resTeam, c.min)
			if resp.Allowed() != c.wantAllow {
				t.Fatalf("allowed = %v, want %v (msg=%q)", resp.Allowed(), c.wantAllow, resp.Message())
			}
			if c.wantInMsg != "" && !contains(resp.Message(), c.wantInMsg) {
				t.Fatalf("message %q should mention %q", resp.Message(), c.wantInMsg)
			}
		})
	}
}

func TestPolicy_AdaptsTypedActor(t *testing.T) {
	// The adapter hands the policy a concrete Actor (asserted from the engine's
	// opaque `any`), so policy code never type-asserts the actor itself.
	fn := authaccess.Policy(func(_ context.Context, a authaccess.Actor, _ any) access.Response {
		if a.TeamRole == authtypes.TeamRoleOwner {
			return access.Allow()
		}
		return access.Deny("not owner")
	})

	owner := fn(context.Background(), authaccess.Actor{TeamRole: authtypes.TeamRoleOwner}, nil)
	if !owner.Allowed() {
		t.Fatal("owner should be allowed through the adapter")
	}
	member := fn(context.Background(), authaccess.Actor{TeamRole: authtypes.TeamRoleMember}, nil)
	if member.Allowed() {
		t.Fatal("member should be denied through the adapter")
	}
}

func TestReadOnlyFreeze(t *testing.T) {
	hook := authaccess.ReadOnlyFreeze

	// Read-only actor performing a mutating ability is frozen.
	resp, handled := hook(context.Background(), authaccess.Actor{ReadOnly: true}, "server.delete")
	if !handled || resp.Allowed() {
		t.Fatalf("read-only mutating should be denied+handled, got handled=%v allowed=%v", handled, resp.Allowed())
	}

	// Read-only actor reading is allowed to fall through to the policy.
	if _, handled := hook(context.Background(), authaccess.Actor{ReadOnly: true}, "server.view"); handled {
		t.Fatal("read-only view should fall through, not be handled")
	}

	// Non-read-only actor is never handled by this hook.
	if _, handled := hook(context.Background(), authaccess.Actor{ReadOnly: false}, "server.delete"); handled {
		t.Fatal("non-read-only should fall through")
	}
}

func TestRequireRole(t *testing.T) {
	// RequireRole checks only the actor's role (team-match is guaranteed upstream
	// by TeamScope + the service's FindByIDAndTeam), ignoring the resource.
	pol := authaccess.RequireRole(authtypes.TeamRoleAdmin)

	admin := pol(context.Background(), authaccess.Actor{TeamRole: authtypes.TeamRoleAdmin}, nil)
	if !admin.Allowed() {
		t.Fatal("admin should satisfy RequireRole(admin)")
	}
	owner := pol(context.Background(), authaccess.Actor{TeamRole: authtypes.TeamRoleOwner}, nil)
	if !owner.Allowed() {
		t.Fatal("owner outranks admin and should be allowed")
	}
	editor := pol(context.Background(), authaccess.Actor{TeamRole: authtypes.TeamRoleEditor}, nil)
	if editor.Allowed() {
		t.Fatal("editor should not satisfy RequireRole(admin)")
	}
	if editor.Message() == "" {
		t.Fatal("denial should carry a reason")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
