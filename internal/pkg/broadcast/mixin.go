package broadcast

import "github.com/kkz6/launch-go/internal/pkg/models"

// Mixin provides nil-safe broadcasting methods that can be embedded
// in services, jobs, and other types that need to broadcast events.
//
// This eliminates the need for repeated nil-check wrapper methods
// across the codebase.
//
// Usage:
//
//	type MyService struct {
//	    broadcast.Mixin
//	    // ... other fields
//	}
//
//	func NewMyService(ws broadcast.Broadcaster) *MyService {
//	    s := &MyService{}
//	    s.SetBroadcaster(ws)
//	    return s
//	}
//
//	func (s *MyService) DoSomething(teamID string) {
//	    s.BroadcastToTeam(teamID, "event.name", data)
//	}
type Mixin struct {
	ws Broadcaster
}

// SetBroadcaster sets the broadcaster for this mixin.
// This should be called during initialization.
func (m *Mixin) SetBroadcaster(ws Broadcaster) {
	m.ws = ws
}

// GetBroadcaster returns the underlying broadcaster, or nil if not set.
func (m *Mixin) GetBroadcaster() Broadcaster {
	return m.ws
}

// HasBroadcaster returns true if a broadcaster is configured.
func (m *Mixin) HasBroadcaster() bool {
	return m.ws != nil
}

// Broadcast sends an event to a specific channel.
// This is a no-op if the broadcaster is nil.
func (m *Mixin) Broadcast(channel, event string, data any) {
	if m.ws != nil {
		m.ws.Broadcast(channel, event, data)
	}
}

// BroadcastToTeam sends an event to the team channel (team.{teamID}).
// This is a no-op if the broadcaster is nil.
//
// The frontend's `useChannelEvents` composable filters incoming events by
// comparing `eventData.team_id` against the subscribed channel — so we
// inject `team_id` here unless the caller already set it. Without this,
// every team-scoped event was silently filtered out client-side, which
// is what caused "the WebSocket isn't updating" symptoms after server
// provisioning failed.
func (m *Mixin) BroadcastToTeam(teamID, event string, data any) {
	if m.ws != nil {
		m.ws.BroadcastToTeam(teamID, event, ensureRoutingField(data, "team_id", teamID))
	}
}

// BroadcastToServer sends an event to the server channel (server.{serverID}).
// This is a no-op if the broadcaster is nil. Like BroadcastToTeam, this
// injects `server_id` into the payload so client-side filters work.
func (m *Mixin) BroadcastToServer(serverID, event string, data any) {
	if m.ws != nil {
		m.ws.BroadcastToServer(serverID, event, ensureRoutingField(data, "server_id", serverID))
	}
}

// BroadcastToSite sends an event to the site channel (site.{siteID}).
// This is a no-op if the broadcaster is nil.
func (m *Mixin) BroadcastToSite(siteID, event string, data any) {
	if m.ws != nil {
		m.ws.BroadcastToSite(siteID, event, ensureRoutingField(data, "site_id", siteID))
	}
}

// BroadcastToDeployment sends an event to the deployment channel (deployment.{deploymentID}).
// This is a no-op if the broadcaster is nil.
func (m *Mixin) BroadcastToDeployment(deploymentID, event string, data any) {
	if m.ws != nil {
		m.ws.BroadcastToDeployment(deploymentID, event, ensureRoutingField(data, "deployment_id", deploymentID))
	}
}

// BroadcastToUser sends an event to the user channel (user.{userID}).
// This is a no-op if the broadcaster is nil.
func (m *Mixin) BroadcastToUser(userID, event string, data any) {
	if m.ws != nil {
		m.ws.BroadcastToUser(userID, event, ensureRoutingField(data, "user_id", userID))
	}
}

// EnsureRoutingField guarantees the routing field (e.g. team_id, server_id)
// is present on the broadcast payload. Existing values are preserved so
// callers that already set the field keep their data. Non-map payloads are
// wrapped so a routing field can be attached without losing the original.
//
// Exported so the websocket layer can call it on the broadcaster side too —
// the Mixin only covers callers that opt in to broadcast.Mixin, whereas the
// worker's broadcast path goes RedisBroadcaster → publish directly and
// would otherwise miss the field. Calling this at both layers is safe; it's
// idempotent.
func EnsureRoutingField(data any, key, value string) any {
	if value == "" {
		return data
	}
	m, ok := data.(map[string]any)
	if !ok {
		return map[string]any{
			key:       value,
			"payload": data,
		}
	}
	if _, exists := m[key]; !exists {
		m[key] = value
	}
	return m
}

// ensureRoutingField is the internal alias used by the Mixin. It exists so
// existing callers in this file don't have to change; new callers (from
// outside the package) should use the exported EnsureRoutingField.
func ensureRoutingField(data any, key, value string) any {
	return EnsureRoutingField(data, key, value)
}

// ModelMixin extends Mixin with model-specific broadcasting methods.
// Embed this in services that need to broadcast model CRUD events.
//
// Usage:
//
//	type ServerService struct {
//	    broadcast.ModelMixin
//	    // ... other fields
//	}
//
//	func (s *ServerService) Create(server *models.Server) {
//	    // ... create server
//	    s.BroadcastModelCreated(server)
//	}
type ModelMixin struct {
	Mixin
	modelWs ModelBroadcaster
}

// SetModelBroadcaster sets the model broadcaster for this mixin.
// This also sets the base broadcaster for compatibility.
func (m *ModelMixin) SetModelBroadcaster(ws ModelBroadcaster) {
	m.modelWs = ws
	m.Mixin.ws = ws // ModelBroadcaster extends Broadcaster
}

// GetModelBroadcaster returns the underlying model broadcaster, or nil if not set.
func (m *ModelMixin) GetModelBroadcaster() ModelBroadcaster {
	return m.modelWs
}

// HasModelBroadcaster returns true if a model broadcaster is configured.
func (m *ModelMixin) HasModelBroadcaster() bool {
	return m.modelWs != nil
}

// BroadcastModelCreated broadcasts a model creation event to the team channel.
// The model must implement the Broadcastable interface.
// This is a no-op if the broadcaster is nil.
func (m *ModelMixin) BroadcastModelCreated(model models.Broadcastable) {
	if m.modelWs == nil {
		return
	}
	m.modelWs.BroadcastModelCreated(
		model.GetTeamID(),
		model.BroadcastName(),
		model.BroadcastPayload()["id"].(string),
		model.BroadcastPayload(),
	)
}

// BroadcastModelUpdated broadcasts a model update event to the team channel.
// The model must implement the Broadcastable interface.
// This is a no-op if the broadcaster is nil.
func (m *ModelMixin) BroadcastModelUpdated(model models.Broadcastable) {
	if m.modelWs == nil {
		return
	}
	m.modelWs.BroadcastModelUpdated(
		model.GetTeamID(),
		model.BroadcastName(),
		model.BroadcastPayload()["id"].(string),
		model.BroadcastPayload(),
	)
}

// BroadcastModelDeleted broadcasts a model deletion event to the team channel.
// The model must implement the Broadcastable interface.
// This is a no-op if the broadcaster is nil.
func (m *ModelMixin) BroadcastModelDeleted(model models.Broadcastable) {
	if m.modelWs == nil {
		return
	}
	m.modelWs.BroadcastModelDeleted(
		model.GetTeamID(),
		model.BroadcastName(),
		model.BroadcastPayload()["id"].(string),
		model.BroadcastPayload(),
	)
}
