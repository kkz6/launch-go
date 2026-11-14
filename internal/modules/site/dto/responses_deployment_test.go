package dto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/site/models"
)

func TestDeploymentResponsePreservesReleaseIdentity(t *testing.T) {
	release := "20261110120000-original-deployment"
	for _, name := range []*string{nil, &release} {
		data, err := json.Marshal(ToDeploymentResponse(&models.Deployment{ReleaseName: name}))
		require.NoError(t, err)
		var response map[string]any
		require.NoError(t, json.Unmarshal(data, &response))
		if name == nil {
			require.NotContains(t, response, "release_name")
		} else {
			require.Equal(t, release, response["release_name"])
		}
	}
}
