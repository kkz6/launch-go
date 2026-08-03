package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	pkgservice "github.com/kkz6/launch-go/internal/pkg/service"
)

func TestSiteConfigurationReservationRollsBackAllClaimedFields(t *testing.T) {
	service, db := sitePHPUpdateServiceFixture(t, "running")

	var site models.Site
	require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
	branch := "main"
	before := "before"
	after := "after"
	retention := 8
	site.RepositoryBranch = &branch
	site.HookBeforeUpdatingRepository = &before
	site.HookAfterUpdatingRepository = &after
	site.DeploymentReleasesRetention = retention
	site.QueueDeployments = false
	site.SharedDirectories = dbtype.JSONStringSlice{"storage"}
	site.SharedFiles = dbtype.JSONStringSlice{".env"}
	site.WriteableDirectories = dbtype.JSONStringSlice{"cache"}
	require.NoError(t, db.Save(&site).Error)

	newBranch := "release"
	newHook := "changed"
	updates := map[string]any{
		"web_folder":                      "web",
		"deployment_releases_retention":   20,
		"queue_deployments":               true,
		"repository_branch":               &newBranch,
		"hook_before_updating_repository": &newHook,
		"hook_after_updating_repository":  &newHook,
		"hook_before_making_current":      &newHook,
		"hook_after_making_current":       &newHook,
		"shared_directories":              dbtype.JSONStringSlice{"new-storage"},
		"shared_files":                    dbtype.JSONStringSlice{"new.env"},
		"writeable_directories":           dbtype.JSONStringSlice{"new-cache"},
	}

	claimedAt, err := service.reserveSiteConfigurationUpdate(
		context.Background(), &site, updates, sitetypes.PhpVersion84.String(), false,
	)
	require.NoError(t, err)
	assert.False(t, claimedAt.IsZero())

	_, err = service.reserveSiteConfigurationUpdate(
		context.Background(), &site, updates, sitetypes.PhpVersion84.String(), false,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already in progress")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service.rollbackSiteConfigurationUpdate(
		ctx, &site, updates, sitetypes.PhpVersion84.String(),
	)

	var persisted models.Site
	require.NoError(t, db.First(&persisted, "id = ?", site.ID).Error)
	assert.Nil(t, persisted.PendingCaddyfileUpdateSince)
	assert.Nil(t, persisted.PendingPhpVersion)
	assert.Equal(t, "public", persisted.WebFolder)
	assert.Equal(t, retention, persisted.DeploymentReleasesRetention)
	assert.False(t, persisted.QueueDeployments)
	require.NotNil(t, persisted.RepositoryBranch)
	assert.Equal(t, branch, *persisted.RepositoryBranch)
	assert.Equal(t, dbtype.JSONStringSlice{"storage"}, persisted.SharedDirectories)
	assert.Equal(t, dbtype.JSONStringSlice{".env"}, persisted.SharedFiles)
	assert.Equal(t, dbtype.JSONStringSlice{"cache"}, persisted.WriteableDirectories)
}

func TestSiteConfigurationReservationSupportsUnknownCurrentPHP(t *testing.T) {
	service, db := sitePHPUpdateServiceFixture(t, "running")
	require.NoError(t, db.Model(&models.Site{}).
		Where("id = ?", "site-1").Update("php_version", nil).Error)

	var site models.Site
	require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
	_, err := service.reserveSiteConfigurationUpdate(
		context.Background(), &site, nil, sitetypes.PhpVersion84.String(), true,
	)
	require.NoError(t, err)

	service.rollbackSiteConfigurationUpdate(
		context.Background(), &site, nil, sitetypes.PhpVersion84.String(),
	)
}

func TestSiteConfigurationHelpersHandleMissingInfrastructure(t *testing.T) {
	service := NewSiteService(&ServiceDeps{})
	_, err := service.reserveSiteConfigurationUpdate(
		context.Background(), &models.Site{}, nil, "", false,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database not configured")

	service.rollbackSiteConfigurationUpdate(
		context.Background(), &models.Site{}, nil, "",
	)
	require.ErrorIs(t, service.enqueueTaskStrict(nil), pkgservice.ErrQueueRequired)
}

func TestTLSRollbackRestoresCertificatesAndPreviousSetting(t *testing.T) {
	sslService, db, site := sslLifecycleFixture(t)
	now := time.Now().UTC()
	previous := &models.Certificate{
		BaseModel:  basemodels.BaseModel{ID: "cert-previous"},
		SiteScoped: basemodels.SiteScoped{SiteID: site.ID},
		TeamScoped: basemodels.TeamScoped{TeamID: site.TeamID},
		Type:       sitetypes.CertificateTypeCustom, IsActive: false,
	}
	replacement := &models.Certificate{
		BaseModel:  basemodels.BaseModel{ID: "cert-replacement"},
		SiteScoped: basemodels.SiteScoped{SiteID: site.ID},
		TeamScoped: basemodels.TeamScoped{TeamID: site.TeamID},
		Type:       sitetypes.CertificateTypeCustom, IsActive: true,
	}
	require.NoError(t, db.Create(previous).Error)
	require.NoError(t, db.Create(replacement).Error)
	require.NoError(t, db.Model(&models.Site{}).Where("id = ?", site.ID).Updates(map[string]any{
		"tls_setting":                            sitetypes.TLSSettingCustom,
		"pending_tls_update_since":               now,
		"pending_tls_previous_setting":           site.TLSSetting,
		"pending_tls_previous_certificate_ids":   dbtype.JSONStringSlice{previous.ID},
		"pending_tls_replacement_certificate_id": replacement.ID,
	}).Error)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := sslService.rollbackTLSReservation(
		ctx, site, sitetypes.TLSSettingCustom, replacement, []string{previous.ID},
	)
	require.NoError(t, err)

	var persisted models.Site
	require.NoError(t, db.First(&persisted, "id = ?", site.ID).Error)
	assert.Equal(t, sitetypes.TLSSettingAuto, persisted.TLSSetting)
	assert.Nil(t, persisted.PendingTLSUpdateSince)
	assert.Nil(t, persisted.PendingTLSPreviousSetting)
	assert.Nil(t, persisted.PendingTLSReplacementCertID)

	var restored models.Certificate
	require.NoError(t, db.First(&restored, "id = ?", previous.ID).Error)
	assert.True(t, restored.IsActive)
	assert.Error(t, db.First(&models.Certificate{}, "id = ?", replacement.ID).Error)

	certificates, err := sslService.ListCertificates(
		context.Background(), site.ID, site.ServerID, site.TeamID,
	)
	require.NoError(t, err)
	require.Len(t, certificates, 1)
	assert.Equal(t, previous.ID, certificates[0].ID)

	_, err = sslService.ListCertificates(
		context.Background(), site.ID, site.ServerID, "other-team",
	)
	require.Error(t, err)
}

func TestTLSRollbackWithoutReplacementAndLostReservation(t *testing.T) {
	sslService, db, site := sslLifecycleFixture(t)
	now := time.Now().UTC()
	require.NoError(t, db.Model(&models.Site{}).Where("id = ?", site.ID).Updates(map[string]any{
		"tls_setting":                  sitetypes.TLSSettingInternal,
		"pending_tls_update_since":     now,
		"pending_tls_previous_setting": site.TLSSetting,
	}).Error)

	require.NoError(t, sslService.rollbackTLSReservation(
		context.Background(), site, sitetypes.TLSSettingInternal, nil, nil,
	))

	err := sslService.rollbackTLSReservation(
		context.Background(), site, sitetypes.TLSSettingInternal, nil, nil,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reservation was lost")
}
