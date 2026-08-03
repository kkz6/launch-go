package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/config"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	queuepkg "github.com/kkz6/launch-go/internal/pkg/queue"
	pkgservice "github.com/kkz6/launch-go/internal/pkg/service"
)

func TestUpdateSiteQueuesPHPVersionLifecycle(t *testing.T) {
	siteService, db := sitePHPUpdateServiceFixture(t, servertypes.ServiceStatusRunning)
	attachSiteLifecycleQueue(t, siteService)
	target := sitetypes.PhpVersion84.String()
	webFolder := "web"

	response, err := siteService.Update(
		context.Background(),
		"site-1",
		"server-1",
		"team-1",
		"user-1",
		&dto.UpdateSiteRequest{
			PhpVersion: &target,
			WebFolder:  &webFolder,
		},
	)

	require.NoError(t, err)
	assert.Equal(t, webFolder, response.WebFolder)

	var persisted models.Site
	require.NoError(t, db.First(&persisted, "id = ?", "site-1").Error)
	require.NotNil(t, persisted.PhpVersion)
	assert.Equal(t, sitetypes.PhpVersion83, *persisted.PhpVersion)
	require.NotNil(t, persisted.PendingPhpVersion)
	assert.Equal(t, sitetypes.PhpVersion84, *persisted.PendingPhpVersion)
	assert.NotNil(t, persisted.PendingCaddyfileUpdateSince)
	assert.Equal(t, webFolder, persisted.WebFolder)
}

func TestUpdateSiteResolvesUnknownCurrentPHPVersion(t *testing.T) {
	siteService, db := sitePHPUpdateServiceFixture(t, servertypes.ServiceStatusRunning)
	attachSiteLifecycleQueue(t, siteService)
	require.NoError(t, db.Model(&models.Site{}).
		Where("id = ?", "site-1").
		Update("php_version", nil).Error)

	reader := siteService.serverReader.(*phpUpdateServerReader)
	reader.server.Services = append(reader.server.Services, servermodels.InstalledService{
		Type:      servertypes.ServiceTypePhp,
		Software:  servertypes.SoftwarePhp83.String(),
		Version:   "8.3.27",
		Status:    servertypes.ServiceStatusRunning,
		IsDefault: true,
	})
	target := sitetypes.PhpVersion84.String()

	_, err := siteService.Update(
		context.Background(),
		"site-1",
		"server-1",
		"team-1",
		"user-1",
		&dto.UpdateSiteRequest{PhpVersion: &target},
	)

	require.NoError(t, err)
	var persisted models.Site
	require.NoError(t, db.First(&persisted, "id = ?", "site-1").Error)
	assert.Nil(t, persisted.PhpVersion)
	require.NotNil(t, persisted.PendingPhpVersion)
	assert.Equal(t, sitetypes.PhpVersion84, *persisted.PendingPhpVersion)
}

func TestUpdateSiteQueuesReservedCaddyfileChange(t *testing.T) {
	siteService, db := sitePHPUpdateServiceFixture(t, servertypes.ServiceStatusRunning)
	attachSiteLifecycleQueue(t, siteService)
	webFolder := "web"

	response, err := siteService.Update(
		context.Background(),
		"site-1",
		"server-1",
		"team-1",
		"user-1",
		&dto.UpdateSiteRequest{WebFolder: &webFolder},
	)

	require.NoError(t, err)
	assert.Equal(t, webFolder, response.WebFolder)
	var persisted models.Site
	require.NoError(t, db.First(&persisted, "id = ?", "site-1").Error)
	assert.Equal(t, webFolder, persisted.WebFolder)
	assert.NotNil(t, persisted.PendingCaddyfileUpdateSince)
	assert.Nil(t, persisted.PendingPhpVersion)
}

func TestUpdateSiteWebFolderChangeRequiresQueue(t *testing.T) {
	siteService, db := sitePHPUpdateServiceFixture(t, servertypes.ServiceStatusRunning)
	webFolder := "web"

	_, err := siteService.Update(
		context.Background(),
		"site-1",
		"server-1",
		"team-1",
		"user-1",
		&dto.UpdateSiteRequest{WebFolder: &webFolder},
	)

	require.ErrorIs(t, err, pkgservice.ErrQueueRequired)
	var persisted models.Site
	require.NoError(t, db.First(&persisted, "id = ?", "site-1").Error)
	assert.Equal(t, "public", persisted.WebFolder)
	assert.Nil(t, persisted.PendingCaddyfileUpdateSince)
}

func TestUpdateSitePersistsNonLifecycleFields(t *testing.T) {
	siteService, db := sitePHPUpdateServiceFixture(t, servertypes.ServiceStatusRunning)
	retention := 12
	queueDeployments := true

	response, err := siteService.Update(
		context.Background(),
		"site-1",
		"server-1",
		"team-1",
		"user-1",
		&dto.UpdateSiteRequest{
			DeploymentReleasesRetention: &retention,
			QueueDeployments:            &queueDeployments,
		},
	)

	require.NoError(t, err)
	assert.Equal(t, retention, response.DeploymentReleasesRetention)
	assert.True(t, response.QueueDeployments)
	var persisted models.Site
	require.NoError(t, db.First(&persisted, "id = ?", "site-1").Error)
	assert.Equal(t, retention, persisted.DeploymentReleasesRetention)
	assert.True(t, persisted.QueueDeployments)
	assert.Nil(t, persisted.PendingCaddyfileUpdateSince)
}

func TestUpdateSiteWithoutChangesReloadsSite(t *testing.T) {
	siteService, _ := sitePHPUpdateServiceFixture(t, servertypes.ServiceStatusRunning)

	response, err := siteService.Update(
		context.Background(),
		"site-1",
		"server-1",
		"team-1",
		"user-1",
		&dto.UpdateSiteRequest{},
	)

	require.NoError(t, err)
	assert.Equal(t, "site-1", response.ID)

	_, err = siteService.Update(
		context.Background(),
		"missing",
		"server-1",
		"team-1",
		"user-1",
		&dto.UpdateSiteRequest{},
	)
	require.Error(t, err)
}

func TestUpdateSitePHPVersionRejectsUnavailableDependencies(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*SiteService, *models.Site)
		contains  string
	}{
		{
			name: "configuration update already running",
			configure: func(_ *SiteService, site *models.Site) {
				now := time.Now()
				site.PendingCaddyfileUpdateSince = &now
			},
			contains: "already in progress",
		},
		{
			name: "TLS update already running",
			configure: func(_ *SiteService, site *models.Site) {
				now := time.Now()
				site.PendingTLSUpdateSince = &now
			},
			contains: "already in progress",
		},
		{
			name: "server reader missing",
			configure: func(service *SiteService, _ *models.Site) {
				service.SetServerReader(nil)
			},
			contains: "server reader not configured",
		},
		{
			name: "server lookup fails",
			configure: func(service *SiteService, _ *models.Site) {
				service.SetServerReader(&phpUpdateServerReader{err: errors.New("lookup failed")})
			},
			contains: "failed to fetch server: lookup failed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			siteService, db := sitePHPUpdateServiceFixture(t, servertypes.ServiceStatusRunning)
			var site models.Site
			require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
			test.configure(siteService, &site)
			require.NoError(t, db.Save(&site).Error)
			target := sitetypes.PhpVersion84.String()

			_, err := siteService.Update(
				context.Background(),
				"site-1",
				"server-1",
				"team-1",
				"user-1",
				&dto.UpdateSiteRequest{PhpVersion: &target},
			)

			require.Error(t, err)
			assert.Contains(t, err.Error(), test.contains)
		})
	}
}

func TestUpdateSiteRejectsLostPendingReservation(t *testing.T) {
	siteService, db := sitePHPUpdateServiceFixture(t, servertypes.ServiceStatusRunning)
	attachSiteLifecycleQueue(t, siteService)
	pendingVersion := sitetypes.PhpVersion82
	require.NoError(t, db.Model(&models.Site{}).
		Where("id = ?", "site-1").
		Update("pending_php_version", pendingVersion).Error)
	target := sitetypes.PhpVersion84.String()

	_, err := siteService.Update(
		context.Background(),
		"site-1",
		"server-1",
		"team-1",
		"user-1",
		&dto.UpdateSiteRequest{PhpVersion: &target},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "already in progress")
}

func TestUpdateSiteRejectsConflictingCaddyfileReservation(t *testing.T) {
	siteService, db := sitePHPUpdateServiceFixture(t, servertypes.ServiceStatusRunning)
	attachSiteLifecycleQueue(t, siteService)
	pendingVersion := sitetypes.PhpVersion82
	require.NoError(t, db.Model(&models.Site{}).
		Where("id = ?", "site-1").
		Update("pending_php_version", pendingVersion).Error)
	webFolder := "web"

	_, err := siteService.Update(
		context.Background(),
		"site-1",
		"server-1",
		"team-1",
		"user-1",
		&dto.UpdateSiteRequest{WebFolder: &webFolder},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "already in progress")
}

func TestUpdateSiteRollsBackReservationsWhenQueueFails(t *testing.T) {
	tests := []struct {
		name    string
		request func() *dto.UpdateSiteRequest
	}{
		{
			name: "PHP lifecycle",
			request: func() *dto.UpdateSiteRequest {
				target := sitetypes.PhpVersion84.String()
				return &dto.UpdateSiteRequest{PhpVersion: &target}
			},
		},
		{
			name: "Caddyfile lifecycle",
			request: func() *dto.UpdateSiteRequest {
				webFolder := "web"
				return &dto.UpdateSiteRequest{WebFolder: &webFolder}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			siteService, db := sitePHPUpdateServiceFixture(t, servertypes.ServiceStatusRunning)
			redis := attachSiteLifecycleQueue(t, siteService)
			redis.Close()

			_, err := siteService.Update(
				context.Background(),
				"site-1",
				"server-1",
				"team-1",
				"user-1",
				test.request(),
			)

			require.Error(t, err)
			var persisted models.Site
			require.NoError(t, db.First(&persisted, "id = ?", "site-1").Error)
			assert.Equal(t, "public", persisted.WebFolder)
			assert.Nil(t, persisted.PendingCaddyfileUpdateSince)
			assert.Nil(t, persisted.PendingPhpVersion)
		})
	}
}

func TestEnqueueTaskStrictRejectsDuplicateTaskID(t *testing.T) {
	siteService, _ := sitePHPUpdateServiceFixture(t, servertypes.ServiceStatusRunning)
	attachSiteLifecycleQueue(t, siteService)
	task := asynq.NewTask("test:site-lifecycle", nil, asynq.TaskID("site-lifecycle-id"))
	require.NoError(t, siteService.enqueueTaskStrict(task))

	err := siteService.enqueueTaskStrict(task)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "already queued")
}

func TestResolveServerPHPVersionUsesFirstActiveFallback(t *testing.T) {
	server := &servermodels.Server{Services: []servermodels.InstalledService{
		{
			Type:     servertypes.ServiceTypeMySQL,
			Software: servertypes.SoftwareMySQL80.String(),
			Status:   servertypes.ServiceStatusRunning,
		},
		{
			Type:     servertypes.ServiceTypePhp,
			Software: "invalid-php",
			Status:   servertypes.ServiceStatusRunning,
		},
		{
			Type:     servertypes.ServiceTypePhp,
			Software: servertypes.SoftwarePhp83.String(),
			Status:   servertypes.ServiceStatusRunning,
		},
		{
			Type:     servertypes.ServiceTypePhp,
			Software: servertypes.SoftwarePhp84.String(),
			Status:   servertypes.ServiceStatusRunning,
		},
	}}

	version, err := resolveServerPHPVersion(server)

	require.NoError(t, err)
	assert.Equal(t, servertypes.SoftwarePhp83.String(), version)
}

func TestValidateActiveServerPHPRejectsInvalidSoftware(t *testing.T) {
	server := &servermodels.Server{}
	for _, phpVersion := range []string{"not-a-service", servertypes.SoftwareMySQL80.String()} {
		err := validateActiveServerPHP(server, phpVersion)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid PHP version")
	}
}

func TestSiteConfigurationReservationReturnsDatabaseError(t *testing.T) {
	siteService, db := sitePHPUpdateServiceFixture(t, servertypes.ServiceStatusRunning)
	var site models.Site
	require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	_, err = siteService.reserveSiteConfigurationUpdate(
		context.Background(),
		&site,
		nil,
		"",
		false,
	)

	require.Error(t, err)
	siteService.rollbackSiteConfigurationUpdate(
		context.Background(),
		&site,
		nil,
		"",
	)
}

func TestSiteConfigurationRollbackHandlesLostReservation(t *testing.T) {
	siteService, db := sitePHPUpdateServiceFixture(t, servertypes.ServiceStatusRunning)
	var site models.Site
	require.NoError(t, db.First(&site, "id = ?", "site-1").Error)

	siteService.rollbackSiteConfigurationUpdate(
		context.Background(),
		&site,
		nil,
		sitetypes.PhpVersion84.String(),
	)
}

func attachSiteLifecycleQueue(t *testing.T, service *SiteService) *miniredis.Miniredis {
	t.Helper()
	redis := miniredis.RunT(t)
	client := queuepkg.NewClient(config.RedisConfig{Address: redis.Addr()})
	service.Queue = client
	t.Cleanup(func() {
		err := client.Close()
		if err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			require.NoError(t, err)
		}
	})
	return redis
}
