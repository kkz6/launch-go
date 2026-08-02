package services

import (
	"context"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

type phpUpdateServerReader struct {
	server *servermodels.Server
	err    error
}

func (r *phpUpdateServerReader) FindServerByID(
	_ context.Context,
	_ string,
) (*servermodels.Server, error) {
	return r.server, r.err
}

func (r *phpUpdateServerReader) FindServicesByServer(
	_ context.Context,
	_ string,
) ([]servermodels.InstalledService, error) {
	if r.server == nil {
		return nil, r.err
	}
	return r.server.Services, r.err
}

func TestSitePHPUpdateDoesNotPersistVersionWhenDispatchFails(t *testing.T) {
	siteService, db := sitePHPUpdateServiceFixture(t, servertypes.ServiceStatusRunning)
	target := sitetypes.PhpVersion84.String()
	webFolder := "web"

	_, err := siteService.Update(
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

	require.Error(t, err)
	require.Contains(t, err.Error(), "Queue not configured")

	var site models.Site
	require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
	require.Equal(t, sitetypes.PhpVersion83, *site.PhpVersion)
	require.Nil(t, site.PendingCaddyfileUpdateSince)
	require.Nil(t, site.PendingPhpVersion)
	require.Equal(t, "public", site.WebFolder)
}

func TestSitePHPUpdateRejectsStoppedTargetBeforeMutation(t *testing.T) {
	siteService, db := sitePHPUpdateServiceFixture(t, servertypes.ServiceStatusStopped)
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
	require.Contains(t, err.Error(), "not active")

	var site models.Site
	require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
	require.Equal(t, sitetypes.PhpVersion83, *site.PhpVersion)
	require.Nil(t, site.PendingCaddyfileUpdateSince)
	require.Nil(t, site.PendingPhpVersion)
}

func sitePHPUpdateServiceFixture(
	t *testing.T,
	targetStatus servertypes.ServiceStatus,
) (*SiteService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"),
		&gorm.Config{
			DisableForeignKeyConstraintWhenMigrating: true,
			Logger:                                   gormlogger.Default.LogMode(gormlogger.Silent),
		},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Site{}))

	phpVersion := sitetypes.PhpVersion83
	site := &models.Site{
		BaseModel:  basemodels.BaseModel{ID: "site-1"},
		Address:    "example.test",
		Type:       sitetypes.SiteTypeLaravel,
		TLSSetting: sitetypes.TLSSettingAuto,
		User:       "launch",
		Path:       "/home/launch/example.test",
		WebFolder:  "public",
		PhpVersion: &phpVersion,
	}
	site.ServerID = "server-1"
	site.TeamID = "team-1"
	site.UserID = "user-1"
	require.NoError(t, db.Create(site).Error)

	server := &servermodels.Server{
		BaseModel: basemodels.BaseModel{ID: "server-1"},
		Services: []servermodels.InstalledService{
			{
				Type:     servertypes.ServiceTypePhp,
				Software: servertypes.SoftwarePhp84.String(),
				Version:  "8.4.11",
				Status:   targetStatus,
			},
		},
	}
	server.TeamID = "team-1"

	logger := zerolog.Nop()
	deps := &ServiceDeps{
		ModuleDeps: service.ModuleDeps[*repositories.Registry]{
			Dependencies: service.Dependencies{
				DB:     db,
				Logger: &logger,
			},
			Repos: repositories.NewRegistry(db),
		},
	}
	siteService := NewSiteService(deps)
	siteService.SetServerReader(&phpUpdateServerReader{server: server})
	return siteService, db
}
