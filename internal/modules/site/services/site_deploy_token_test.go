package services

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

func TestRegenerateDeployToken_FitsStorageAndReplacesOldToken(t *testing.T) {
	db := setupWebhookTestDB(t)
	logger := zerolog.Nop()
	svc := NewSiteService(&ServiceDeps{
		ModuleDeps: service.ModuleDeps[*repositories.Registry]{
			Dependencies: service.Dependencies{Logger: &logger},
			Repos:        repositories.NewRegistry(db),
		},
	})
	oldToken := "-test_TOKEN-with-hyphens_012345678"
	site := createTestSite(t, db, &oldToken, nil)
	ctx := context.Background()

	deployments := newTestDeploymentService(db)
	currentToken := oldToken
	// Sample multiple rotations so a Base64URL token that happens to contain
	// only letters and numbers cannot mask a regression in the generator.
	for range 32 {
		updated, err := svc.RegenerateDeployToken(ctx, site.ID, site.ServerID, site.TeamID)
		require.NoError(t, err)
		require.NotNil(t, updated.DeployToken)
		require.Len(t, *updated.DeployToken, 32, "token must fit the deploy_token VARCHAR(32) column")
		assert.Regexp(t, `^[A-Za-z0-9]{32}$`, *updated.DeployToken)
		assert.NotEqual(t, currentToken, *updated.DeployToken)

		var stored models.Site
		require.NoError(t, db.First(&stored, "id = ?", site.ID).Error)
		require.NotNil(t, stored.DeployToken)
		assert.Equal(t, *updated.DeployToken, *stored.DeployToken)
		assert.ErrorIs(t, deployments.DeployFromWebhook(ctx, site.ID, currentToken, nil), ErrInvalidDeployToken)
		currentToken = *updated.DeployToken
	}

	require.NoError(t, deployments.DeployFromWebhook(ctx, site.ID, currentToken, nil))
	var count int64
	require.NoError(t, db.Model(&models.Deployment{}).Where("site_id = ?", site.ID).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestDeployFromWebhook_PreservesURLSafeTokenCharacters(t *testing.T) {
	db := setupWebhookTestDB(t)
	svc := newTestDeploymentService(db)
	token := "-test_TOKEN-with-hyphens_012345678"
	site := createTestSite(t, db, &token, nil)
	ctx := context.Background()

	// Removing the leading hyphen or replacing underscores must not authenticate.
	assert.ErrorIs(t, svc.DeployFromWebhook(ctx, site.ID, token[1:], nil), ErrInvalidDeployToken)
	assert.ErrorIs(t, svc.DeployFromWebhook(ctx, site.ID, "-test-TOKEN-with-hyphens-012345678", nil), ErrInvalidDeployToken)
	require.NoError(t, svc.DeployFromWebhook(ctx, site.ID, token, nil))
	var count int64
	require.NoError(t, db.Model(&models.Deployment{}).Where("site_id = ?", site.ID).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}
