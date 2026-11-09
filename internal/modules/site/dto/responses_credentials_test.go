package dto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/site/models"
)

func TestSharedSiteResponseOmitsDeploymentCredentials(t *testing.T) {
	token := "test-deployment-secret"
	site := &models.Site{DeployToken: &token}
	site.ID = "test-site"

	// This DTO is used by lists, mutations, and team-wide socket events.
	data, err := json.Marshal(map[string]any{"site": ToSiteResponse(site)})
	require.NoError(t, err)
	require.NotContains(t, string(data), token)
	require.NotContains(t, string(data), "deploy_token")
	require.NotContains(t, string(data), "deploy_webhook_url")

	response := ToSiteResponseWithDeployCredentials(site)
	require.Equal(t, &token, response.DeployToken)
	require.Contains(t, response.DeployWebhookURL, "/deploy/test-site/"+token)
}

func TestDeploymentCredentialsRequireNonemptyToken(t *testing.T) {
	empty := ""
	for _, token := range []*string{nil, &empty} {
		response := ToSiteResponseWithDeployCredentials(&models.Site{DeployToken: token})
		require.Nil(t, response.DeployToken)
		require.Empty(t, response.DeployWebhookURL)
	}
}
