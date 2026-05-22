package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/types"
)

// buildCredentialsMap is the seam between the UI's request payload and the
// providers/ package's Extract* helpers. The bug we fixed was the UI POSTing
// `api_token` while ExtractToken reads `token` — these tests pin that mapping.

func TestBuildCredentialsMap_TokenProviders(t *testing.T) {
	tokenProviders := []types.ServerProvider{
		types.ProviderDigitalOcean,
		types.ProviderHetzner,
		types.ProviderLinode,
		types.ProviderVultr,
	}
	for _, p := range tokenProviders {
		t.Run(string(p), func(t *testing.T) {
			req := &dto.CreateServerProviderRequest{
				Provider: string(p),
				Profile:  "test",
				APIToken: "secret-token",
			}
			creds, err := buildCredentialsMap(p, req)
			require.NoError(t, err)

			// providers/base.go ExtractToken reads credentials["token"], NOT api_token.
			assert.Equal(t, "secret-token", creds["token"], "%s must store api_token under the 'token' key", p)
			_, hasAPIToken := creds["api_token"]
			assert.False(t, hasAPIToken, "%s must not leak the UI's api_token key into the stored map", p)
		})
	}
}

func TestBuildCredentialsMap_AWS(t *testing.T) {
	req := &dto.CreateServerProviderRequest{
		Provider:  string(types.ProviderAWS),
		Profile:   "prod",
		AccessKey: "AKIA-test",
		SecretKey: "supersecret",
		Region:    "us-east-1",
	}
	creds, err := buildCredentialsMap(types.ProviderAWS, req)
	require.NoError(t, err)

	assert.Equal(t, "AKIA-test", creds["access_key"])
	assert.Equal(t, "supersecret", creds["secret_key"])
	assert.Equal(t, "us-east-1", creds["region"])
	_, hasToken := creds["token"]
	assert.False(t, hasToken, "AWS credentials must not include a 'token' key")
}

func TestBuildCredentialsMap_AWS_RequiresAllFields(t *testing.T) {
	cases := map[string]*dto.CreateServerProviderRequest{
		"missing_access_key": {Provider: "aws", Profile: "p", SecretKey: "s", Region: "us-east-1"},
		"missing_secret_key": {Provider: "aws", Profile: "p", AccessKey: "a", Region: "us-east-1"},
		"missing_region":     {Provider: "aws", Profile: "p", AccessKey: "a", SecretKey: "s"},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := buildCredentialsMap(types.ProviderAWS, req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "AWS")
		})
	}
}

func TestBuildCredentialsMap_TokenProvider_RejectsEmpty(t *testing.T) {
	_, err := buildCredentialsMap(types.ProviderDigitalOcean, &dto.CreateServerProviderRequest{
		Provider: "digitalocean",
		Profile:  "test",
		// APIToken intentionally empty
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "api_token")
}
