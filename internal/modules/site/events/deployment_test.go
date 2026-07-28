package events

import (
	"testing"

	"github.com/stretchr/testify/require"

	pkgevents "github.com/kkz6/launch-go/internal/pkg/events"
)

func TestNewDeploymentSucceeded(t *testing.T) {
	payload := DeploymentSucceeded{
		DeploymentID: "deployment-1",
		SiteID:       "site-1",
		ServerID:     "server-1",
		TeamID:       "team-1",
		TaskID:       "task-1",
	}

	event, err := NewDeploymentSucceeded(payload)
	require.NoError(t, err)
	require.Equal(t, payload.DeploymentID, event.ID)
	require.Equal(t, DeploymentSucceededName, event.Name)

	decoded, err := pkgevents.Decode[DeploymentSucceeded](event)
	require.NoError(t, err)
	require.Equal(t, payload, decoded)
}

func TestNewDeploymentSucceededRequiresDeploymentID(t *testing.T) {
	_, err := NewDeploymentSucceeded(DeploymentSucceeded{SiteID: "site-1"})

	require.ErrorContains(t, err, "event id is required")
}
