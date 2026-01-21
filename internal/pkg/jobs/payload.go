package jobs

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

// BasePayload provides common fields for all job payloads.
// Embed this struct in your payload to get standard fields like
// UserID and TeamID that are commonly needed across jobs.
//
// Example usage:
//
//	type InstallCronPayload struct {
//	    jobs.BasePayload
//	    CronID string `json:"cron_id"`
//	}
//
//	// Create with user context:
//	payload := InstallCronPayload{
//	    BasePayload: jobs.BasePayload{UserID: userID},
//	    CronID:      cron.ID,
//	}
type BasePayload struct {
	// UserID is the ID of the user who initiated the job (optional).
	// Used for activity logging and auditing.
	UserID *string `json:"user_id,omitempty"`

	// TeamID is the ID of the team context for the job (optional).
	// Used for multi-tenancy and broadcasting events.
	TeamID string `json:"team_id,omitempty"`
}

// HasUser returns true if the payload has a user ID set.
func (p BasePayload) HasUser() bool {
	return p.UserID != nil && *p.UserID != ""
}

// HasTeam returns true if the payload has a team ID set.
func (p BasePayload) HasTeam() bool {
	return p.TeamID != ""
}

// GetUserID returns the user ID or empty string if not set.
func (p BasePayload) GetUserID() string {
	if p.UserID == nil {
		return ""
	}
	return *p.UserID
}

// WithUser returns a new BasePayload with the user ID set.
// This is useful for creating payloads in a fluent style.
func (p BasePayload) WithUser(userID string) BasePayload {
	p.UserID = &userID
	return p
}

// WithTeam returns a new BasePayload with the team ID set.
func (p BasePayload) WithTeam(teamID string) BasePayload {
	p.TeamID = teamID
	return p
}

// NewBasePayload creates a BasePayload with optional user ID.
func NewBasePayload(userID ...string) BasePayload {
	p := BasePayload{}
	if len(userID) > 0 && userID[0] != "" {
		p.UserID = &userID[0]
	}
	return p
}

// ServerPayload extends BasePayload with server-specific fields.
// Use this for jobs that operate on a specific server.
//
// Example usage:
//
//	type RebootServerPayload struct {
//	    jobs.ServerPayload
//	}
type ServerPayload struct {
	BasePayload
	ServerID string `json:"server_id"`
}

// NewServerPayload creates a ServerPayload with the given server ID.
func NewServerPayload(serverID string, userID ...string) ServerPayload {
	return ServerPayload{
		BasePayload: NewBasePayload(userID...),
		ServerID:    serverID,
	}
}

// SitePayload extends BasePayload with site-specific fields.
// Use this for jobs that operate on a specific site.
//
// Example usage:
//
//	type DeployPayload struct {
//	    jobs.SitePayload
//	    DeploymentID string `json:"deployment_id"`
//	}
type SitePayload struct {
	BasePayload
	SiteID   string `json:"site_id"`
	ServerID string `json:"server_id,omitempty"` // Often needed along with site
}

// NewSitePayload creates a SitePayload with the given site ID.
func NewSitePayload(siteID string, userID ...string) SitePayload {
	return SitePayload{
		BasePayload: NewBasePayload(userID...),
		SiteID:      siteID,
	}
}

// WithServer adds a server ID to the SitePayload.
func (p SitePayload) WithServer(serverID string) SitePayload {
	p.ServerID = serverID
	return p
}

// PayloadBuilder provides a fluent interface for building job payloads.
// This reduces boilerplate when creating payloads with common fields.
//
// Example usage:
//
//	payload := jobs.NewPayloadBuilder().
//	    WithUser(userID).
//	    WithTeam(teamID).
//	    WithField("cron_id", cronID).
//	    Build()
type PayloadBuilder struct {
	fields map[string]any
}

// NewPayloadBuilder creates a new PayloadBuilder.
func NewPayloadBuilder() *PayloadBuilder {
	return &PayloadBuilder{
		fields: make(map[string]any),
	}
}

// WithUser adds a user ID to the payload.
func (b *PayloadBuilder) WithUser(userID string) *PayloadBuilder {
	if userID != "" {
		b.fields["user_id"] = userID
	}
	return b
}

// WithTeam adds a team ID to the payload.
func (b *PayloadBuilder) WithTeam(teamID string) *PayloadBuilder {
	if teamID != "" {
		b.fields["team_id"] = teamID
	}
	return b
}

// WithServer adds a server ID to the payload.
func (b *PayloadBuilder) WithServer(serverID string) *PayloadBuilder {
	b.fields["server_id"] = serverID
	return b
}

// WithSite adds a site ID to the payload.
func (b *PayloadBuilder) WithSite(siteID string) *PayloadBuilder {
	b.fields["site_id"] = siteID
	return b
}

// WithField adds a custom field to the payload.
func (b *PayloadBuilder) WithField(key string, value any) *PayloadBuilder {
	b.fields[key] = value
	return b
}

// Build returns the payload as a map.
func (b *PayloadBuilder) Build() map[string]any {
	result := make(map[string]any, len(b.fields))
	for k, v := range b.fields {
		result[k] = v
	}
	return result
}

// BuildJSON returns the payload as JSON bytes.
func (b *PayloadBuilder) BuildJSON() ([]byte, error) {
	return json.Marshal(b.fields)
}

// MustBuildJSON returns the payload as JSON bytes, panicking on error.
func (b *PayloadBuilder) MustBuildJSON() []byte {
	data, err := b.BuildJSON()
	if err != nil {
		panic(fmt.Sprintf("failed to marshal payload: %v", err))
	}
	return data
}

// ToTask creates an asynq.Task with the built payload.
func (b *PayloadBuilder) ToTask(taskType string, opts ...asynq.Option) (*asynq.Task, error) {
	data, err := b.BuildJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	opts = append([]asynq.Option{asynq.MaxRetry(DefaultMaxRetry)}, opts...)
	return asynq.NewTask(taskType, data, opts...), nil
}
