package jobs

import (
	"context"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/repositories"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

func TestUpdateCaddyfileRejectsLostOrConflictingReservations(t *testing.T) {
	for _, test := range []struct {
		name    string
		payload CaddyfilePayload
		prepare func(*testing.T, *models.Site)
		want    string
	}{
		{
			name:    "PHP update owns Caddyfile",
			payload: CaddyfilePayload{SiteID: "site-1"},
			prepare: func(t *testing.T, site *models.Site) {
				version := sitetypes.PhpVersion84
				site.PendingPhpVersion = &version
			},
			want: "already owns the Caddyfile",
		},
		{
			name:    "TLS reservation is missing",
			payload: CaddyfilePayload{SiteID: "site-1", TLSUpdate: true},
			prepare: func(t *testing.T, site *models.Site) {
				site.PendingTLSUpdateSince = nil
			},
			want: "TLS update reservation was lost",
		},
		{
			name:    "Caddy reservation is missing",
			payload: CaddyfilePayload{SiteID: "site-1", ReservationClaimed: true},
			prepare: func(t *testing.T, site *models.Site) {
				site.PendingCaddyfileUpdateSince = nil
				site.PendingTLSUpdateSince = nil
			},
			want: "caddyfile update reservation was lost",
		},
		{
			name:    "TLS owns generic reservation",
			payload: CaddyfilePayload{SiteID: "site-1", ReservationClaimed: true},
			prepare: func(t *testing.T, _ *models.Site) {},
			want:    "TLS update already owns",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := caddyReservationFixture(t)
			var site models.Site
			require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
			test.prepare(t, &site)
			require.NoError(t, db.Save(&site).Error)

			logger := zerolog.Nop()
			job := &UpdateCaddyfileJob{
				Deps: &JobDeps{
					Deps:  &pkgjobs.Deps{DB: db, Logger: &logger},
					Repos: repositories.NewRegistry(db),
				},
				Payload: test.payload,
			}

			err := job.Handle(context.Background())
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.want)
		})
	}
}

func TestCaddyfileTaskFactoriesPreserveLifecycleFlags(t *testing.T) {
	userID := "user-1"
	tests := []struct {
		name     string
		factory  func(string, *string) (*asynq.Task, error)
		taskType string
		reserved bool
		tls      bool
	}{
		{name: "install", factory: NewInstallCaddyfileTask, taskType: TypeInstallCaddyfile},
		{name: "update", factory: NewUpdateCaddyfileTask, taskType: TypeUpdateCaddyfile},
		{name: "reserved update", factory: NewReservedUpdateCaddyfileTask, taskType: TypeUpdateCaddyfile, reserved: true},
		{name: "TLS update", factory: NewTLSUpdateCaddyfileTask, taskType: TypeUpdateCaddyfile, tls: true},
		{name: "uninstall", factory: NewUninstallCaddyfileTask, taskType: TypeUninstallCaddyfile},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			task, err := test.factory("site-1", &userID)
			require.NoError(t, err)
			assert.Equal(t, test.taskType, task.Type())
			payload, err := pkgjobs.UnmarshalPayload[CaddyfilePayload](task)
			require.NoError(t, err)
			assert.Equal(t, "site-1", payload.SiteID)
			assert.Equal(t, &userID, payload.UserID)
			assert.Equal(t, test.reserved, payload.ReservationClaimed)
			assert.Equal(t, test.tls, payload.TLSUpdate)
		})
	}
}

func TestCaddyfileJobConstructorsAndCertificatePointer(t *testing.T) {
	payload := CaddyfilePayload{SiteID: "site-1"}
	assert.IsType(t, &InstallCaddyfileJob{}, NewInstallCaddyfileJob(payload))
	assert.IsType(t, &UpdateCaddyfileJob{}, NewUpdateCaddyfileJob(payload))
	assert.IsType(t, &UninstallCaddyfileJob{}, NewUninstallCaddyfileJob(payload))
	assert.Empty(t, pointerString(nil))
	value := "certificate"
	assert.Equal(t, value, pointerString(&value))
}
