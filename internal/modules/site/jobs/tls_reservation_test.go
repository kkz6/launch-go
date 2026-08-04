package jobs

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

func TestCompleteTLSReservationClearsRollbackState(t *testing.T) {
	db := tlsReservationFixture(t)

	require.NoError(t, completeTLSReservation(context.Background(), db, "site-1"))

	var site models.Site
	require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
	assert.Equal(t, sitetypes.TLSSettingCustom, site.TLSSetting)
	assert.Nil(t, site.PendingTLSUpdateSince)
	assert.Nil(t, site.PendingCaddyfileUpdateSince)
	assert.Nil(t, site.PendingTLSPreviousSetting)
	assert.Empty(t, site.PendingTLSPreviousCertIDs)
	assert.Nil(t, site.PendingTLSReplacementCertID)

	var replacement models.Certificate
	require.NoError(t, db.First(&replacement, "id = ?", "cert-new").Error)
	assert.True(t, replacement.IsActive)
}

func TestRollbackTLSReservationRestoresPreviousState(t *testing.T) {
	db := tlsReservationFixture(t)

	require.NoError(t, rollbackTLSReservation(context.Background(), db, "site-1"))

	var site models.Site
	require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
	assert.Equal(t, sitetypes.TLSSettingAuto, site.TLSSetting)
	assert.Nil(t, site.PendingTLSUpdateSince)
	assert.Nil(t, site.PendingCaddyfileUpdateSince)
	assert.Nil(t, site.PendingTLSPreviousSetting)
	assert.Empty(t, site.PendingTLSPreviousCertIDs)
	assert.Nil(t, site.PendingTLSReplacementCertID)

	var previous models.Certificate
	require.NoError(t, db.First(&previous, "id = ?", "cert-old").Error)
	assert.True(t, previous.IsActive)
	assert.ErrorIs(
		t,
		db.First(&models.Certificate{}, "id = ?", "cert-new").Error,
		gorm.ErrRecordNotFound,
	)
}

func TestRollbackTLSReservationDoesNotOverwriteLostOwnership(t *testing.T) {
	db := tlsReservationFixture(t)
	require.NoError(t, db.Model(&models.Site{}).
		Where("id = ?", "site-1").
		Updates(map[string]any{
			"pending_tls_update_since":               nil,
			"pending_caddyfile_update_since":         nil,
			"pending_tls_previous_setting":           nil,
			"pending_tls_previous_certificate_ids":   nil,
			"pending_tls_replacement_certificate_id": nil,
		}).Error)

	err := rollbackTLSReservation(context.Background(), db, "site-1")

	require.Error(t, err)
	assert.True(t, errors.Is(err, errTLSReservationLost))
	var site models.Site
	require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
	assert.Equal(t, sitetypes.TLSSettingCustom, site.TLSSetting)
	var replacement models.Certificate
	require.NoError(t, db.First(&replacement, "id = ?", "cert-new").Error)
	assert.True(t, replacement.IsActive)
}

func tlsReservationFixture(t *testing.T) *gorm.DB {
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

	now := time.Now().UTC().Truncate(time.Second)
	previousSetting := sitetypes.TLSSettingAuto
	replacementID := "cert-new"
	phpVersion := sitetypes.PhpVersion83
	site := &models.Site{
		BaseModel:                   basemodels.BaseModel{ID: "site-1"},
		Address:                     "example.test",
		Type:                        sitetypes.SiteTypeLaravel,
		TLSSetting:                  sitetypes.TLSSettingCustom,
		User:                        "launch",
		Path:                        "/home/launch/example.test",
		WebFolder:                   "public",
		PhpVersion:                  &phpVersion,
		PendingTLSUpdateSince:       &now,
		PendingTLSPreviousSetting:   &previousSetting,
		PendingTLSPreviousCertIDs:   dbtype.JSONStringSlice{"cert-old"},
		PendingTLSReplacementCertID: &replacementID,
		PendingCaddyfileUpdateSince: &now,
	}
	site.ServerID = "server-1"
	site.TeamID = "team-1"
	site.UserID = "user-1"
	require.NoError(t, db.Create(site).Error)

	previous := &models.Certificate{
		BaseModel: basemodels.BaseModel{ID: "cert-old"},
		Type:      sitetypes.CertificateTypeCustom,
		IsActive:  false,
	}
	previous.SiteID = site.ID
	previous.TeamID = site.TeamID
	replacement := &models.Certificate{
		BaseModel: basemodels.BaseModel{ID: replacementID},
		Type:      sitetypes.CertificateTypeCustom,
		IsActive:  true,
	}
	replacement.SiteID = site.ID
	replacement.TeamID = site.TeamID
	require.NoError(t, db.Create(previous).Error)
	require.NoError(t, db.Create(replacement).Error)
	return db
}
