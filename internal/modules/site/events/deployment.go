package events

import pkgevents "github.com/kkz6/launch-go/internal/pkg/events"

const (
	// DeploymentSucceededName is emitted after a deployment is durably marked
	// finished. The deployment ID is the event ID, making every deployment a
	// distinct, replay-safe occurrence.
	DeploymentSucceededName = "site.deployment.succeeded"

	// TypeProcessEvent is the durable worker that fans site events out to their
	// registered listeners.
	TypeProcessEvent = "site:process_event"
)

// DeploymentSucceeded is the typed payload for DeploymentSucceededName.
type DeploymentSucceeded struct {
	DeploymentID string `json:"deployment_id"`
	SiteID       string `json:"site_id"`
	ServerID     string `json:"server_id"`
	TeamID       string `json:"team_id"`
	TaskID       string `json:"task_id"`
}

// NewDeploymentSucceeded creates the durable domain-event envelope.
func NewDeploymentSucceeded(payload DeploymentSucceeded) (pkgevents.Event, error) {
	return pkgevents.New(payload.DeploymentID, DeploymentSucceededName, payload)
}
