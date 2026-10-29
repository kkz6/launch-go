package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSiteBeforeCreate_GeneratesAlphanumericDeployToken(t *testing.T) {
	for _, emptyToken := range []bool{false, true} {
		seen := make(map[string]bool)
		for range 32 {
			site := &Site{}
			if emptyToken {
				empty := ""
				site.DeployToken = &empty
			}

			require.NoError(t, site.BeforeCreate(nil))
			require.NotNil(t, site.DeployToken)
			token := *site.DeployToken
			assert.Regexp(t, `^[A-Za-z0-9]{32}$`, token)
			assert.False(t, seen[token], "new sites must get distinct deploy tokens")
			seen[token] = true
		}
	}
}

func TestSiteBeforeCreate_PreservesExistingDeployToken(t *testing.T) {
	token := "-test_TOKEN-with-hyphens_012345678"
	site := &Site{DeployToken: &token}

	require.NoError(t, site.BeforeCreate(nil))
	require.NotNil(t, site.DeployToken)
	assert.Equal(t, token, *site.DeployToken)
}
