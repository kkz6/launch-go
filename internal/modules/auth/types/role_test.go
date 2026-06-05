package types

import "testing"

func TestTeamRole_Level_Ordering(t *testing.T) {
	// Higher privilege => higher level. owner > admin > editor > member.
	if !(TeamRoleOwner.Level() > TeamRoleAdmin.Level()) {
		t.Fatalf("owner(%d) should outrank admin(%d)", TeamRoleOwner.Level(), TeamRoleAdmin.Level())
	}
	if !(TeamRoleAdmin.Level() > TeamRoleEditor.Level()) {
		t.Fatalf("admin(%d) should outrank editor(%d)", TeamRoleAdmin.Level(), TeamRoleEditor.Level())
	}
	if !(TeamRoleEditor.Level() > TeamRoleMember.Level()) {
		t.Fatalf("editor(%d) should outrank member(%d)", TeamRoleEditor.Level(), TeamRoleMember.Level())
	}
}

func TestTeamRole_Level_InvalidIsZero(t *testing.T) {
	if got := TeamRole("nonsense").Level(); got != 0 {
		t.Fatalf("invalid role level = %d, want 0", got)
	}
	// An empty role (no team scope) must rank below every real role.
	if TeamRole("").Level() >= TeamRoleMember.Level() {
		t.Fatalf("empty role must rank below member")
	}
}

func TestTeamRole_AtLeast(t *testing.T) {
	cases := []struct {
		role TeamRole
		min  TeamRole
		want bool
	}{
		{TeamRoleOwner, TeamRoleAdmin, true},
		{TeamRoleAdmin, TeamRoleAdmin, true},
		{TeamRoleEditor, TeamRoleAdmin, false},
		{TeamRoleMember, TeamRoleEditor, false},
		{TeamRoleEditor, TeamRoleMember, true},
		{TeamRole(""), TeamRoleMember, false},
		{TeamRole("nonsense"), TeamRoleMember, false},
	}
	for _, c := range cases {
		if got := c.role.AtLeast(c.min); got != c.want {
			t.Errorf("%q.AtLeast(%q) = %v, want %v", c.role, c.min, got, c.want)
		}
	}
}
