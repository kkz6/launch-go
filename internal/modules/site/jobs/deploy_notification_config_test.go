package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/site/models"
)

func TestBuildDeployConfig_IncludesNotificationContext(t *testing.T) {
	installedAt := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	site := &models.Site{Address: "karti.dev"}
	site.ID = "site-1"
	site.ServerID = "server-1"
	site.InstalledAt = &installedAt

	deployment := &models.Deployment{}
	deployment.ID = "deployment-1"

	job := &DeployJob{
		Deps: &JobDeps{FrontendURL: "https://launchctl.io/"},
		server: &servermodels.Server{
			Name: "production-01",
		},
	}

	config := job.buildDeployConfig(context.Background(), site, deployment, "team-1")

	assert.Equal(t, "production-01", config.ServerName)
	assert.Equal(
		t,
		"https://launchctl.io/servers/server-1/sites/site-1?deployment=deployment-1&tab=deployments",
		config.DeploymentURL,
	)
}
