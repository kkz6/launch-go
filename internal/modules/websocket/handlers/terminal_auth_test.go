package handlers

import (
	"net"
	"net/url"
	"testing"
	"time"

	wsclient "github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/middleware"
	authaccess "github.com/kkz6/launch-go/internal/modules/auth/access"
	authtypes "github.com/kkz6/launch-go/internal/modules/auth/types"
	scriptpolicies "github.com/kkz6/launch-go/internal/modules/script/policies"
	serverpolicies "github.com/kkz6/launch-go/internal/modules/server/policies"
	"github.com/kkz6/launch-go/internal/pkg/access"
	"github.com/kkz6/launch-go/internal/pkg/cache"
	launchcache "github.com/kkz6/launch-go/internal/pkg/launch/cache"
)

const terminalTestSecret = "terminal-auth-test-secret"

func TestTerminalAuthorizesIdentityAndRoleBeforeSSH(t *testing.T) {
	for _, tc := range []struct {
		name      string
		role      string
		claims    jwt.MapClaims
		updateSQL string
		serverID  string
		omitGate  bool
		want      string
	}{
		{name: "editor", role: "editor", want: "No SSH key configured"},
		{name: "admin", role: "admin", want: "No SSH key configured"},
		{name: "owner", role: "owner", want: "No SSH key configured"},
		{name: "member", role: "member", want: "Permission denied"},
		{name: "read-only owner", role: "owner", claims: jwt.MapClaims{
			"read_only": true, "impersonation_sid": "spectate-session", "impersonator_id": "staff-user",
		}, want: "Permission denied"},
		{name: "revoked session", role: "owner", updateSQL: "DELETE FROM sessions", want: "Authentication failed"},
		{name: "refresh token", role: "owner", claims: jwt.MapClaims{"type": "refresh"}, want: "Authentication failed"},
		{name: "suspended user", role: "owner", updateSQL: "UPDATE users SET status = 'suspended'", want: "Authentication failed"},
		{name: "expired token", role: "owner", claims: jwt.MapClaims{"exp": time.Now().Add(-time.Hour).Unix()}, want: "Authentication failed"},
		{name: "outsider", want: "Authentication failed"},
		{name: "cross-team server", role: "owner", serverID: "other-server", want: "Server not found"},
		{name: "missing authorization gate", role: "owner", omitGate: true, want: "Permission denied"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			endpoint, db := socketAuthFixture(t, tc.role, !tc.omitGate)
			if tc.updateSQL != "" {
				require.NoError(t, db.Exec(tc.updateSQL).Error)
			}
			serverID := tc.serverID
			if serverID == "" {
				serverID = "server-1"
			}
			query := url.Values{"serverId": {serverID}, "username": {"root"}}
			connection := dialAuthenticatedSocket(t, endpoint+"/terminal/ws", query, tc.claims)
			_, message, err := connection.ReadMessage()
			require.NoError(t, err)
			require.Contains(t, string(message), tc.want)
			if tc.want != "No SSH key configured" {
				require.NotContains(t, string(message), "No SSH key configured", "rejection must happen before SSH setup")
			}
		})
	}
}

func socketAuthFixture(t *testing.T, role string, withGate bool) (string, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	for _, statement := range []string{
		"CREATE TABLE users (id TEXT PRIMARY KEY, status TEXT)",
		"CREATE TABLE sessions (id TEXT PRIMARY KEY, user_id TEXT)",
		"CREATE TABLE teams (id TEXT PRIMARY KEY, user_id TEXT)",
		"CREATE TABLE team_user (team_id TEXT, user_id TEXT, role TEXT)",
		"CREATE TABLE servers (id TEXT PRIMARY KEY, team_id TEXT, name TEXT, provider TEXT, connected BOOLEAN, public_ipv4 TEXT, private_key TEXT)",
		"INSERT INTO users VALUES ('user-1', 'active')",
		"INSERT INTO sessions VALUES ('session-1', 'user-1')",
		"INSERT INTO teams VALUES ('team-1', 'other-owner')",
		"INSERT INTO servers VALUES ('server-1', 'team-1', 'Test server', 'custom', 1, '127.0.0.1', '')",
		"INSERT INTO servers VALUES ('other-server', 'other-team', 'Other server', 'custom', 1, '127.0.0.1', '')",
	} {
		require.NoError(t, db.Exec(statement).Error)
	}
	if role == "owner" {
		require.NoError(t, db.Exec("UPDATE teams SET user_id = 'user-1'").Error)
	} else if role != "" {
		require.NoError(t, db.Exec("INSERT INTO team_user VALUES ('team-1', 'user-1', ?)", role).Error)
	}

	// Use the same identity hooks as API startup, backed by the fixture DB.
	middleware.InitSessionValidator(func(sessionID string) bool {
		var count int64
		return db.Table("sessions").Where("id = ?", sessionID).Count(&count).Error == nil && count == 1
	})
	middleware.InitUserStatus(func(userID string) authtypes.UserStatus {
		var user struct{ Status authtypes.UserStatus }
		_ = db.Table("users").Where("id = ?", userID).Take(&user).Error
		return user.Status
	})
	t.Cleanup(func() {
		middleware.InitSessionValidator(nil)
		middleware.InitUserStatus(nil)
	})
	membership := launchcache.NewTeamMembershipCache(cache.NewNoopCache(), db)
	base := NewBase(db, terminalTestSecret, zerolog.Nop(), membership)
	if withGate {
		base.Gate = access.New()
		base.Gate.Before(authaccess.ReadOnlyFreeze)
		serverpolicies.Register(base.Gate)
		scriptpolicies.Register(base.Gate)
	}
	application := fiber.New(fiber.Config{DisableStartupMessage: true})
	application.Get("/terminal/ws", NewTerminalHandler(base).Handler())
	application.Get("/scripts/execute", NewScriptExecutionHandler(base).Handler())
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	stopped := make(chan error, 1)
	go func() { stopped <- application.Listener(listener) }()
	t.Cleanup(func() {
		require.NoError(t, application.ShutdownWithTimeout(3*time.Second))
		select {
		case err := <-stopped:
			require.NoError(t, err)
		case <-time.After(3 * time.Second):
			t.Fatal("terminal test server did not stop")
		}
	})
	return "ws://" + listener.Addr().String(), db
}

func dialAuthenticatedSocket(t *testing.T, endpoint string, query url.Values, overrides jwt.MapClaims) *wsclient.Conn {
	t.Helper()
	claims := jwt.MapClaims{
		"sub": "user-1", "type": "access", "session_id": "session-1",
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	for key, value := range overrides {
		claims[key] = value
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(terminalTestSecret))
	require.NoError(t, err)
	query.Set("token", token)
	query.Set("team_id", "team-1")
	dialer := wsclient.Dialer{HandshakeTimeout: 3 * time.Second}
	connection, _, err := dialer.Dial(endpoint+"?"+query.Encode(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = connection.Close() })
	require.NoError(t, connection.SetReadDeadline(time.Now().Add(3*time.Second)))
	return connection
}
