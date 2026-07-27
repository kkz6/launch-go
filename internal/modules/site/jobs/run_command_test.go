package jobs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
)

func TestCommandEventDataIncludesRoutingAndCurrentState(t *testing.T) {
	t.Parallel()

	command := &models.Command{
		Command: "php artisan migrate --force",
		Status:  sitetypes.CommandStatusFinished,
	}
	command.ID = "command-1"
	command.SiteID = "site-1"
	command.TeamID = "team-1"

	data := commandEventData(command)

	require.Equal(t, "command-1", data["command_id"])
	require.Equal(t, "site-1", data["site_id"])
	require.Equal(t, "finished", data["status"])

	response, ok := data["command"].(dto.CommandResponse)
	require.True(t, ok)
	require.Equal(t, "command-1", response.ID)
	require.Equal(t, "site-1", response.SiteID)
	require.Equal(t, "finished", response.Status)
}
