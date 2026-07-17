package jobs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeploymentLogURL(t *testing.T) {
	assert.Equal(t,
		"https://launch.test/servers/server-1/sites/site-1?deployment=deployment-1&tab=deployments",
		deploymentLogURL("https://launch.test/", "server-1", "site-1", "deployment-1"),
	)
	assert.Empty(t, deploymentLogURL("", "server-1", "site-1", "deployment-1"))
}
