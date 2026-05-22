package broadcast

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingBroadcaster captures the (channel, event, data) tuples the mixin
// hands off so the test can assert exactly what hit the wire.
type recordingBroadcaster struct {
	NopBroadcaster
	calls []recorded
}

type recorded struct {
	method string
	id     string
	event  string
	data   any
}

func (r *recordingBroadcaster) BroadcastToTeam(teamID, event string, data any) {
	r.calls = append(r.calls, recorded{"team", teamID, event, data})
}

func (r *recordingBroadcaster) BroadcastToServer(serverID, event string, data any) {
	r.calls = append(r.calls, recorded{"server", serverID, event, data})
}

func (r *recordingBroadcaster) BroadcastToSite(siteID, event string, data any) {
	r.calls = append(r.calls, recorded{"site", siteID, event, data})
}

func (r *recordingBroadcaster) BroadcastToDeployment(deploymentID, event string, data any) {
	r.calls = append(r.calls, recorded{"deployment", deploymentID, event, data})
}

func (r *recordingBroadcaster) BroadcastToUser(userID, event string, data any) {
	r.calls = append(r.calls, recorded{"user", userID, event, data})
}

func newMixin(t *testing.T) (*Mixin, *recordingBroadcaster) {
	t.Helper()
	rec := &recordingBroadcaster{}
	m := &Mixin{}
	m.SetBroadcaster(rec)
	return m, rec
}

// The whole reason this fix exists: without team_id in the payload the
// frontend's useChannelEvents filter drops the event. These tests pin the
// guarantee so that regression can't sneak back in.

func TestBroadcastToTeam_InjectsTeamID(t *testing.T) {
	m, rec := newMixin(t)
	m.BroadcastToTeam("team-abc", "server.create_failed", map[string]any{
		"server_id": "srv-1",
		"error":     "boom",
	})

	require.Len(t, rec.calls, 1)
	got, ok := rec.calls[0].data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "team-abc", got["team_id"], "team_id must be injected so frontend filters match")
	assert.Equal(t, "srv-1", got["server_id"])
	assert.Equal(t, "boom", got["error"])
}

func TestBroadcastToTeam_PreservesCallerTeamID(t *testing.T) {
	// If a caller already set team_id (e.g. cross-team broadcast) we must
	// not stomp on it. The mixin should fill in only when missing.
	m, rec := newMixin(t)
	m.BroadcastToTeam("team-abc", "evt", map[string]any{
		"team_id": "team-explicit",
	})

	got := rec.calls[0].data.(map[string]any)
	assert.Equal(t, "team-explicit", got["team_id"])
}

func TestBroadcastToTeam_WrapsNonMapPayload(t *testing.T) {
	// Some callers pass typed structs/strings. The mixin should still
	// attach the routing field without losing the original payload.
	m, rec := newMixin(t)
	type custom struct{ X int }
	m.BroadcastToTeam("team-abc", "evt", custom{X: 7})

	got := rec.calls[0].data.(map[string]any)
	assert.Equal(t, "team-abc", got["team_id"])
	assert.Equal(t, custom{X: 7}, got["payload"])
}

func TestBroadcastToServer_InjectsServerID(t *testing.T) {
	m, rec := newMixin(t)
	m.BroadcastToServer("srv-99", "server.updated", map[string]any{})

	got := rec.calls[0].data.(map[string]any)
	assert.Equal(t, "srv-99", got["server_id"])
}

func TestBroadcastToSite_InjectsSiteID(t *testing.T) {
	m, rec := newMixin(t)
	m.BroadcastToSite("site-7", "site.updated", map[string]any{})

	got := rec.calls[0].data.(map[string]any)
	assert.Equal(t, "site-7", got["site_id"])
}

func TestBroadcastToDeployment_InjectsDeploymentID(t *testing.T) {
	m, rec := newMixin(t)
	m.BroadcastToDeployment("dep-3", "deployment.started", map[string]any{})

	got := rec.calls[0].data.(map[string]any)
	assert.Equal(t, "dep-3", got["deployment_id"])
}

func TestBroadcastToUser_InjectsUserID(t *testing.T) {
	m, rec := newMixin(t)
	m.BroadcastToUser("u-1", "user.notified", map[string]any{})

	got := rec.calls[0].data.(map[string]any)
	assert.Equal(t, "u-1", got["user_id"])
}

func TestBroadcastNilDataStillRoutes(t *testing.T) {
	// nil payload is allowed (some lifecycle events carry no extra data).
	// The mixin should produce a routable wrapper rather than silently
	// passing nil through, which would otherwise break client filters.
	m, rec := newMixin(t)
	m.BroadcastToTeam("team-abc", "evt", nil)

	got := rec.calls[0].data.(map[string]any)
	assert.Equal(t, "team-abc", got["team_id"])
}
