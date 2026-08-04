package jobs

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	sitemodels "github.com/kkz6/launch-go/internal/modules/site/models"
	sitetemplates "github.com/kkz6/launch-go/internal/modules/site/tasks/templates"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	tasktemplates "github.com/kkz6/launch-go/internal/pkg/taskrunner/templates"
)

var (
	siteTaskTemplatesOnce sync.Once
	siteTaskTemplatesErr  error
)

func TestInstallCaddyfileHandlePersistsSuccessfulLifecycle(t *testing.T) {
	ensureSiteTaskTemplates(t)
	jobDeps, dispatcher, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
	now := time.Now().UTC()
	require.NoError(t, db.Model(&sitemodels.Site{}).
		Where("id = ?", "site-1").
		Updates(map[string]any{
			"installed_at":                   nil,
			"installation_failed_at":         now,
			"pending_caddyfile_update_since": now,
		}).Error)
	redirect := &sitemodels.Redirect{
		BaseModel: basemodels.BaseModel{ID: "redirect-pending"},
		Mode:      301,
		From:      "/old",
		To:        "/new",
		Status:    "pending",
	}
	redirect.SiteID = "site-1"
	redirect.TeamID = "team-1"
	redirect.UserID = "user-1"
	require.NoError(t, db.Create(redirect).Error)
	job := &InstallCaddyfileJob{
		Deps:    jobDeps,
		Payload: CaddyfilePayload{SiteID: "site-1"},
	}

	require.NoError(t, job.Handle(context.Background()))
	assert.Equal(t, 2, dispatcher.ExecutionCount())
	assert.Equal(t, "Update Caddy Site Imports", dispatcher.LastExecution().Task.Name())

	var site sitemodels.Site
	require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
	assert.NotNil(t, site.InstalledAt)
	assert.Nil(t, site.InstallationFailedAt)
	assert.Nil(t, site.PendingCaddyfileUpdateSince)
	require.NoError(t, db.First(redirect, "id = ?", redirect.ID).Error)
	assert.Equal(t, "installed", redirect.Status)

	var tracked []servermodels.Task
	require.NoError(t, db.Order("created_at asc").Find(&tracked).Error)
	require.Len(t, tracked, 2)
	assert.Equal(t, "Update Caddyfile", tracked[0].Name)
	assert.Equal(t, string(servertypes.TaskStatusFinished), tracked[0].Status)
	assert.Equal(t, "Update Caddy Site Imports", tracked[1].Name)
	assert.Equal(t, string(servertypes.TaskStatusFinished), tracked[1].Status)
}

func TestInstallCaddyfileHandleLookupAndTaskFailures(t *testing.T) {
	t.Run("site lookup", func(t *testing.T) {
		ensureSiteTaskTemplates(t)
		jobDeps, dispatcher, _ := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
		job := &InstallCaddyfileJob{Deps: jobDeps, Payload: CaddyfilePayload{SiteID: "missing"}}
		err := job.Handle(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to find site")
		assert.Zero(t, dispatcher.ExecutionCount())
	})

	t.Run("server lookup", func(t *testing.T) {
		ensureSiteTaskTemplates(t)
		jobDeps, dispatcher, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
		require.NoError(t, db.Delete(&servermodels.Server{}, "id = ?", "server-1").Error)
		job := &InstallCaddyfileJob{Deps: jobDeps, Payload: CaddyfilePayload{SiteID: "site-1"}}
		err := job.Handle(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to find server")
		assert.Zero(t, dispatcher.ExecutionCount())
	})

	t.Run("transport error terminates tracked task", func(t *testing.T) {
		ensureSiteTaskTemplates(t)
		jobDeps, dispatcher, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
		dispatcher.SetRunError(errors.New("ssh disconnected"))
		job := &InstallCaddyfileJob{Deps: jobDeps, Payload: CaddyfilePayload{SiteID: "site-1"}}

		err := job.Handle(context.Background())
		require.Error(t, err)
		assert.ErrorContains(t, err, "exit code -1")
		var tracked servermodels.Task
		require.NoError(t, db.First(&tracked).Error)
		assert.Equal(t, string(servertypes.TaskStatusFailed), tracked.Status)
		assert.Contains(t, tracked.Output.String(), "ssh disconnected")
	})

	t.Run("nonzero exit", func(t *testing.T) {
		ensureSiteTaskTemplates(t)
		jobDeps, dispatcher, _ := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
		dispatcher.SetFailure("Update Caddyfile", 23, "invalid Caddyfile")
		job := &InstallCaddyfileJob{Deps: jobDeps, Payload: CaddyfilePayload{SiteID: "site-1"}}

		err := job.Handle(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exit code 23")
	})
}

func TestInstallCaddyfileWritesCustomCertificateBeforeConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name     string
		failCert bool
	}{
		{name: "success"},
		{name: "certificate write fails", failCert: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ensureSiteTaskTemplates(t)
			jobDeps, dispatcher, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
			require.NoError(t, db.Model(&sitemodels.Site{}).
				Where("id = ?", "site-1").Update("tls_setting", sitetypes.TLSSettingCustom).Error)
			certificatePEM := "certificate pem"
			cert := &sitemodels.Certificate{
				BaseModel:   basemodels.BaseModel{ID: "cert-active"},
				Type:        sitetypes.CertificateTypeCustom,
				Certificate: &certificatePEM,
				PrivateKey:  dbtype.EncryptedString("private key"),
				IsActive:    true,
			}
			cert.SiteID = "site-1"
			cert.TeamID = "team-1"
			require.NoError(t, db.Create(cert).Error)
			if tc.failCert {
				dispatcher.SetFailure("Write Site Certificates", 8, "write failed")
			}
			job := &InstallCaddyfileJob{Deps: jobDeps, Payload: CaddyfilePayload{SiteID: "site-1"}}

			err := job.Handle(context.Background())
			if tc.failCert {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "certificate file write failed with exit code 8")
				assert.Equal(t, 1, dispatcher.ExecutionCount())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, 3, dispatcher.ExecutionCount())
			assert.Equal(t, "Write Site Certificates", dispatcher.GetExecution(0).Task.Name())
			assert.Equal(t, "Update Caddyfile", dispatcher.GetExecution(1).Task.Name())
		})
	}
}

func TestInstallCaddyfileFailedMarksInstallationFailure(t *testing.T) {
	t.Run("persists failure", func(t *testing.T) {
		jobDeps, _, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
		job := &InstallCaddyfileJob{Deps: jobDeps, Payload: CaddyfilePayload{SiteID: "site-1"}}

		job.Failed(context.Background(), errors.New("Caddy rejected config"))

		var site sitemodels.Site
		require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
		assert.Nil(t, site.InstalledAt)
		assert.NotNil(t, site.InstallationFailedAt)
	})

	t.Run("missing site is ignored", func(t *testing.T) {
		jobDeps, _, _ := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
		job := &InstallCaddyfileJob{Deps: jobDeps, Payload: CaddyfilePayload{SiteID: "missing"}}
		assert.NotPanics(t, func() {
			job.Failed(context.Background(), errors.New("install failed"))
		})
	})

	t.Run("missing server is ignored after persistence", func(t *testing.T) {
		jobDeps, _, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
		require.NoError(t, db.Delete(&servermodels.Server{}, "id = ?", "server-1").Error)
		job := &InstallCaddyfileJob{Deps: jobDeps, Payload: CaddyfilePayload{SiteID: "site-1"}}
		assert.NotPanics(t, func() {
			job.Failed(context.Background(), errors.New("install failed"))
		})
	})
}

func TestInstallCaddyfileContentDelegatesToSharedGenerator(t *testing.T) {
	site := newLaravelSite()
	job := &InstallCaddyfileJob{}
	assert.Equal(
		t,
		generateCaddyfile(site, nil, "", nil),
		job.generateCaddyfileContent(site, nil, "", nil),
	)
}

func TestUpdateCaddyfileHandleClaimsAndClearsGenericReservation(t *testing.T) {
	ensureSiteTaskTemplates(t)
	jobDeps, dispatcher, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
	require.NoError(t, db.Model(&sitemodels.Site{}).
		Where("id = ?", "site-1").
		Updates(map[string]any{
			"pending_php_version":            nil,
			"pending_caddyfile_update_since": nil,
			"pending_tls_update_since":       nil,
		}).Error)
	job := &UpdateCaddyfileJob{Deps: jobDeps, Payload: CaddyfilePayload{SiteID: "site-1"}}

	require.NoError(t, job.Handle(context.Background()))
	assert.Equal(t, 1, dispatcher.ExecutionCount())
	var site sitemodels.Site
	require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
	assert.Nil(t, site.PendingCaddyfileUpdateSince)
}

func TestUpdateCaddyfileTLSFailureCallbackRollsBackOrCompletes(t *testing.T) {
	for _, tc := range []struct {
		name       string
		tlsApplied bool
		wantTLS    sitetypes.TLSSetting
		wantCert   bool
	}{
		{name: "before Caddy applied", wantTLS: sitetypes.TLSSettingAuto},
		{name: "after Caddy applied", tlsApplied: true, wantTLS: sitetypes.TLSSettingCustom, wantCert: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			jobDeps, _, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
			configureCaddyTLSReservation(t, db)
			job := &UpdateCaddyfileJob{
				Deps:       jobDeps,
				Payload:    CaddyfilePayload{SiteID: "site-1", TLSUpdate: true},
				tlsApplied: tc.tlsApplied,
			}

			job.Failed(context.Background(), errors.New("Caddy reload failed"))

			var site sitemodels.Site
			require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
			assert.Equal(t, tc.wantTLS, site.TLSSetting)
			assert.Nil(t, site.PendingTLSUpdateSince)
			assert.Nil(t, site.PendingCaddyfileUpdateSince)
			err := db.First(&sitemodels.Certificate{}, "id = ?", "cert-new").Error
			if tc.wantCert {
				require.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
			}
		})
	}
}

func TestUpdateCaddyfileFailureClearsGenericReservation(t *testing.T) {
	jobDeps, _, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
	require.NoError(t, db.Model(&sitemodels.Site{}).
		Where("id = ?", "site-1").Update("pending_php_version", nil).Error)
	job := &UpdateCaddyfileJob{
		Deps:    jobDeps,
		Payload: CaddyfilePayload{SiteID: "site-1", ReservationClaimed: true},
	}

	job.Failed(context.Background(), errors.New("update failed"))

	var site sitemodels.Site
	require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
	assert.Nil(t, site.PendingCaddyfileUpdateSince)
}

func configureCaddyTLSReservation(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	previousSetting := sitetypes.TLSSettingAuto
	replacementID := "cert-new"
	require.NoError(t, db.Model(&sitemodels.Site{}).
		Where("id = ?", "site-1").
		Updates(map[string]any{
			"tls_setting":                            sitetypes.TLSSettingCustom,
			"pending_php_version":                    nil,
			"pending_tls_update_since":               now,
			"pending_tls_previous_setting":           previousSetting,
			"pending_tls_previous_certificate_ids":   dbtype.JSONStringSlice{"cert-old"},
			"pending_tls_replacement_certificate_id": replacementID,
			"pending_caddyfile_update_since":         now,
		}).Error)
	previous := &sitemodels.Certificate{
		BaseModel: basemodels.BaseModel{ID: "cert-old"},
		Type:      sitetypes.CertificateTypeCustom,
		IsActive:  false,
	}
	previous.SiteID = "site-1"
	previous.TeamID = "team-1"
	replacement := &sitemodels.Certificate{
		BaseModel: basemodels.BaseModel{ID: replacementID},
		Type:      sitetypes.CertificateTypeCustom,
		IsActive:  true,
	}
	replacement.SiteID = "site-1"
	replacement.TeamID = "team-1"
	require.NoError(t, db.Create(previous).Error)
	require.NoError(t, db.Create(replacement).Error)
}

func ensureSiteTaskTemplates(t *testing.T) {
	t.Helper()
	siteTaskTemplatesOnce.Do(func() {
		siteTaskTemplatesErr = tasktemplates.Register(
			"site",
			sitetemplates.FS,
			&tasktemplates.RegisterOptions{UseLenientShellMode: true},
		)
	})
	require.NoError(t, siteTaskTemplatesErr)
}
