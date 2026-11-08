package broadcast

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminInvalidationContainsOnlyRoutingIDs(t *testing.T) {
	change := AdminChangeForTeamEvent("team-a", "site.deleted", map[string]any{
		"site_id": "site-a", "server_id": "server-a", "user_id": "owner-a",
		"team_id": "untrusted-team", "address": "private.example", "password": "secret",
		"output": "private task output", "data": map[string]any{"api_key": "secret"},
	})
	encoded, err := json.Marshal(change)
	require.NoError(t, err)
	require.JSONEq(t, `{"resource":"site","action":"deleted","team_id":"team-a","user_id":"owner-a","server_id":"server-a","site_id":"site-a"}`, string(encoded))
	server := AdminChangeForTeamEvent("team-a", "server.updated", struct {
		ID string `json:"id"`
	}{ID: "server-b"})
	require.Equal(t, "server-b", server.ServerID)
}

func TestAdminInvalidationOmitsStreamsAndUnrelatedEvents(t *testing.T) {
	for _, event := range []string{"task.output", "task.progress", "task.step_started", "server.provision_progress", "deployment.gha_steps", "server.metrics", "notification.created", "unknown.updated", "malformed"} {
		t.Run(event, func(t *testing.T) { require.Nil(t, AdminChangeForTeamEvent("team-a", event, nil)) })
	}
	for _, event := range []string{"site.deletion_failed", "site.deleted", "server.deletion_failed", "server.updated", "docker.project.deleted", "task.updated", "deployment.finished"} {
		t.Run(event, func(t *testing.T) { require.NotNil(t, AdminChangeForTeamEvent("team-a", event, nil)) })
	}
	require.Nil(t, AdminChangeForTeamEvent("", "site.deleted", nil))
}

func TestAdminServerDeletionFailureOnlyCopiesAllowlistedCodes(t *testing.T) {
	for _, tc := range []struct{ event, code, expected string }{
		{"server.deletion_failed", "provider_unauthorized", "provider_unauthorized"},
		{"server.deletion_failed", "provider_forbidden", "provider_forbidden"},
		{"server.deletion_failed", "private-provider-error", ""},
		{"server.updated", "provider_unauthorized", ""},
		{"site.deletion_failed", "provider_unauthorized", ""},
	} {
		change := AdminChangeForTeamEvent("team-a", tc.event, map[string]any{
			"server_id": "server-a", "error_code": tc.code, "error": "secret-token-and-provider-body",
		})
		require.Equal(t, tc.expected, change.ErrorCode)
		encoded, err := json.Marshal(change)
		require.NoError(t, err)
		require.NotContains(t, string(encoded), "secret-token")
		require.NotContains(t, string(encoded), "private-provider-error")
	}
}
