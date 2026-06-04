package access_test

import (
	"context"
	"testing"

	"github.com/kkz6/launch-go/internal/pkg/access"
)

// testActor is a stand-in subject; the engine treats actors as opaque `any`.
type testActor struct {
	role string
}

func allowEditor(_ context.Context, actor any, _ any) access.Response {
	if actor.(testActor).role == "editor" {
		return access.Allow()
	}
	return access.Deny("must be editor")
}

func TestGate_Allows(t *testing.T) {
	g := access.New()
	g.Define("post.update", allowEditor)

	if !g.Allows(context.Background(), testActor{role: "editor"}, "post.update", nil) {
		t.Fatal("editor should be allowed")
	}
	if g.Allows(context.Background(), testActor{role: "viewer"}, "post.update", nil) {
		t.Fatal("viewer should be denied")
	}
}

func TestGate_Inspect_CarriesDenyMessage(t *testing.T) {
	g := access.New()
	g.Define("post.update", allowEditor)

	resp := g.Inspect(context.Background(), testActor{role: "viewer"}, "post.update", nil)
	if resp.Allowed() {
		t.Fatal("expected deny")
	}
	if resp.Message() != "must be editor" {
		t.Fatalf("message = %q, want %q", resp.Message(), "must be editor")
	}
}

func TestGate_UnknownAbilityDenied(t *testing.T) {
	g := access.New()
	resp := g.Inspect(context.Background(), testActor{}, "does.not.exist", nil)
	if resp.Allowed() {
		t.Fatal("unknown ability must be denied")
	}
	if resp.Message() == "" {
		t.Fatal("unknown ability deny should explain why")
	}
}

func TestGate_BeforeHook_ShortCircuitsAllow(t *testing.T) {
	g := access.New()
	g.Define("post.update", allowEditor) // would deny a viewer...
	g.Before(func(_ context.Context, actor any, _ string) (access.Response, bool) {
		if actor.(testActor).role == "superadmin" {
			return access.Allow(), true // ...but superadmin bypasses
		}
		return access.Response{}, false
	})

	if !g.Allows(context.Background(), testActor{role: "superadmin"}, "post.update", nil) {
		t.Fatal("before-hook should allow superadmin")
	}
	// Non-matching actor still falls through to the ability.
	if g.Allows(context.Background(), testActor{role: "viewer"}, "post.update", nil) {
		t.Fatal("viewer should still be denied")
	}
}

func TestGate_BeforeHook_ShortCircuitsDeny(t *testing.T) {
	g := access.New()
	g.Define("post.update", allowEditor) // would allow an editor...
	g.Before(func(_ context.Context, _ any, _ string) (access.Response, bool) {
		return access.Deny("frozen"), true // ...but global freeze wins
	})

	resp := g.Inspect(context.Background(), testActor{role: "editor"}, "post.update", nil)
	if resp.Allowed() {
		t.Fatal("before-hook deny should win over ability allow")
	}
	if resp.Message() != "frozen" {
		t.Fatalf("message = %q, want %q", resp.Message(), "frozen")
	}
}

func TestGate_Authorize(t *testing.T) {
	g := access.New()
	g.Define("post.update", allowEditor)

	if err := g.Authorize(context.Background(), testActor{role: "editor"}, "post.update", nil); err != nil {
		t.Fatalf("allowed actor should get nil error, got %v", err)
	}

	err := g.Authorize(context.Background(), testActor{role: "viewer"}, "post.update", nil)
	if err == nil {
		t.Fatal("denied actor should get an error")
	}
	if err.Error() != "must be editor" {
		t.Fatalf("error = %q, want %q", err.Error(), "must be editor")
	}
}
