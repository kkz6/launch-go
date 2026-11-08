package websocket

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestChannelAuthorizer(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE servers (id TEXT PRIMARY KEY, team_id TEXT NOT NULL)`,
		`CREATE TABLE sites (id TEXT PRIMARY KEY, team_id TEXT NOT NULL)`,
		`CREATE TABLE deployments (id TEXT PRIMARY KEY, team_id TEXT NOT NULL)`,
		`CREATE TABLE tasks (id TEXT PRIMARY KEY, server_id TEXT NOT NULL)`,
		`INSERT INTO servers VALUES ('srv-a', 'team-a'), ('srv-b', 'team-b')`,
		`INSERT INTO sites VALUES ('site-a', 'team-a')`,
		`INSERT INTO deployments VALUES ('dep-a', 'team-a')`,
		`INSERT INTO tasks VALUES ('task-a', 'srv-a'), ('task-b', 'srv-b')`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("execute %q: %v", statement, err)
		}
	}

	authorizer := newChannelAuthorizer(db)
	tests := map[string]bool{
		"team.team-a":        true,
		"user.user-a":        true,
		"server.srv-a":       true,
		"site.site-a":        true,
		"deployment.dep-a":   true,
		"task.task-a":        true,
		"team.team-b":        false,
		"user.user-b":        false,
		"server.srv-b":       false,
		"task.task-b":        false,
		"server.missing":     false,
		"unknown.anything":   false,
		"server.srv-a.extra": false,
		"malformed":          false,
		"":                   false,
	}
	for channel, want := range tests {
		t.Run(channel, func(t *testing.T) {
			if got := authorizer.AuthorizeChannel("user-a", "team-a", channel); got != want {
				t.Fatalf("AuthorizeChannel(%q) = %v, want %v", channel, got, want)
			}
		})
	}
}

func TestChannelAuthorizerFailsClosedWithoutDatabase(t *testing.T) {
	authorizer := newChannelAuthorizer(nil)
	if authorizer.AuthorizeChannel("user-a", "team-a", "server.srv-a") {
		t.Fatal("resource authorization succeeded without a database")
	}
}

func TestAdminChannelUsesFreshActiveStaffRole(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE users (id TEXT PRIMARY KEY, staff_role TEXT, status TEXT)`,
		`INSERT INTO users VALUES ('admin', 'super_admin', 'active'), ('support', 'support', 'active'), ('customer', NULL, 'active'), ('suspended', 'super_admin', 'suspended')`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	authorizer := newChannelAuthorizer(db)
	for user, allowed := range map[string]bool{"admin": true, "support": true, "customer": false, "suspended": false, "missing": false} {
		if got := authorizer.AuthorizeChannel(user, "team-a", "admin.resources"); got != allowed {
			t.Fatalf("%s access = %v, want %v", user, got, allowed)
		}
	}
	if err := db.Exec(`UPDATE users SET staff_role = NULL WHERE id = 'admin'`).Error; err != nil {
		t.Fatal(err)
	}
	if authorizer.AuthorizeChannel("admin", "team-a", "admin.resources") {
		t.Fatal("demoted staff retained access")
	}
	if authorizer.AuthorizeChannel("support", "team-a", "admin.unknown") {
		t.Fatal("unknown staff channel allowed")
	}
	if newChannelAuthorizer(nil).AuthorizeChannel("support", "team-a", "admin.resources") {
		t.Fatal("staff channel allowed without database")
	}
}
