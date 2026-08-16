package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/modules/site/dto"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/certificatecheck"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/pkg/service"
)

type fakeCertificateChecker struct {
	result certificatecheck.Result
	hosts  []string
}

func (f *fakeCertificateChecker) Check(_ context.Context, host string) certificatecheck.Result {
	f.hosts = append(f.hosts, host)
	return f.result
}

func TestCheckCertificateUsesPublicCheckerForAutomaticTLS(t *testing.T) {
	sslService, _, site := sslLifecycleFixture(t)
	checker := &fakeCertificateChecker{result: certificatecheck.Result{
		Host:    site.Address,
		Status:  certificatecheck.StatusValid,
		Valid:   true,
		Message: "valid",
	}}
	sslService.SetCertificateChecker(checker)

	result, err := sslService.CheckCertificate(
		context.Background(), site.ID, site.ServerID, site.TeamID,
	)

	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Equal(t, []string{site.Address}, checker.hosts)
}

func TestCheckCertificateExplainsDisabledTLSWithoutNetworkCall(t *testing.T) {
	sslService, db, site := sslLifecycleFixture(t)
	require.NoError(t, db.Model(&models.Site{}).
		Where("id = ?", site.ID).
		Update("tls_setting", sitetypes.TLSSettingOff).Error)
	checker := &fakeCertificateChecker{}
	sslService.SetCertificateChecker(checker)

	result, err := sslService.CheckCertificate(
		context.Background(), site.ID, site.ServerID, site.TeamID,
	)

	require.NoError(t, err)
	assert.Equal(t, certificatecheck.StatusNotIssued, result.Status)
	assert.Contains(t, result.Message, "disabled")
	assert.Empty(t, checker.hosts)
}

func TestUpdateSSLRequiresQueueBeforeReservingTLSState(t *testing.T) {
	sslService, db, site := sslLifecycleFixture(t)

	err := sslService.UpdateSSL(
		context.Background(),
		site.ID,
		site.ServerID,
		site.TeamID,
		"user-1",
		&dto.UpdateSSLRequest{TLSSetting: string(sitetypes.TLSSettingInternal)},
	)

	require.ErrorIs(t, err, service.ErrQueueRequired)
	var persisted models.Site
	require.NoError(t, db.First(&persisted, "id = ?", site.ID).Error)
	assert.Equal(t, sitetypes.TLSSettingAuto, persisted.TLSSetting)
	assert.Nil(t, persisted.PendingTLSUpdateSince)
	assert.Nil(t, persisted.PendingCaddyfileUpdateSince)
}

func TestSiteConfigurationIdleIncludesTLSTransitions(t *testing.T) {
	now := time.Now()
	site := &models.Site{PendingTLSUpdateSince: &now}

	err := ensureSiteConfigurationIdle(site)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "configuration update is already in progress")
}

func sslLifecycleFixture(t *testing.T) (*SSLService, *gorm.DB, *models.Site) {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"),
		&gorm.Config{
			DisableForeignKeyConstraintWhenMigrating: true,
			Logger:                                   gormlogger.Default.LogMode(gormlogger.Silent),
		},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Site{}, &models.Certificate{}))

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
	return NewSSLService(deps), db, site
}
