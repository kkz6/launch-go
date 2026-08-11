package tasks

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetemplates "github.com/kkz6/launch-go/internal/modules/site/tasks/templates"
	tasktemplates "github.com/kkz6/launch-go/internal/pkg/taskrunner/templates"
)

func TestDeploySiteTask_PreservesNotificationContext(t *testing.T) {
	tasktemplates.Reset()
	require.NoError(t, tasktemplates.Register("site", sitetemplates.FS, &tasktemplates.RegisterOptions{UseLenientShellMode: true}))
	t.Cleanup(tasktemplates.Reset)

	installedAt := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	site := &models.Site{
		Address: "karti.dev",
		Path:    "/home/launcher/karti.dev",
		User:    "launcher",
	}
	site.ID = "site-1"
	site.ServerID = "server-1"
	site.InstalledAt = &installedAt

	deployment := &models.Deployment{}
	deployment.ID = "deployment-1"

	task := DeploySiteTask(DeployOptions{
		Site:          site,
		Deployment:    deployment,
		TeamID:        "team-1",
		ServerName:    "production-01",
		DeploymentURL: "https://launchctl.io/servers/server-1/sites/site-1?tab=deployments&deployment=deployment-1",
	})

	deployTask, ok := task.(*deploySiteTask)
	require.True(t, ok)
	assert.Equal(t, "production-01", deployTask.callback.ServerName)
	assert.Equal(t, "https://launchctl.io/servers/server-1/sites/site-1?tab=deployments&deployment=deployment-1", deployTask.callback.DeploymentURL)

	payload, err := deployTask.MarshalPayload()
	require.NoError(t, err)

	var restored callbackData
	require.NoError(t, json.Unmarshal(payload, &restored))
	assert.Equal(t, deployTask.callback.ServerName, restored.ServerName)
	assert.Equal(t, deployTask.callback.DeploymentURL, restored.DeploymentURL)
}
