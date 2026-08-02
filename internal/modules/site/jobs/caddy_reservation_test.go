package jobs

import (
	"context"
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
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
)

func TestCaddyPendingCleanupOnlyClearsOwnedTLSReservation(t *testing.T) {
	for _, test := range []struct {
		name        string
		tlsUpdate   bool
		wantPending bool
	}{
		{name: "TLS update", tlsUpdate: true, wantPending: false},
		{name: "generic update", tlsUpdate: false, wantPending: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := caddyReservationFixture(t)
			job := &UpdateCaddyfileJob{
				Deps: &JobDeps{Deps: &pkgjobs.Deps{DB: db}},
				Payload: CaddyfilePayload{
					SiteID:    "site-1",
					TLSUpdate: test.tlsUpdate,
				},
			}

			require.NoError(t, job.clearPendingState(context.Background()))

			var site models.Site
			require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
			assert.Nil(t, site.PendingCaddyfileUpdateSince)
			assert.Equal(t, test.wantPending, site.PendingTLSUpdateSince != nil)
		})
	}
}

func caddyReservationFixture(t *testing.T) *gorm.DB {
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

	now := time.Now().UTC()
	phpVersion := sitetypes.PhpVersion83
	site := &models.Site{
		BaseModel:                   basemodels.BaseModel{ID: "site-1"},
		Address:                     "example.test",
		Type:                        sitetypes.SiteTypeLaravel,
		TLSSetting:                  sitetypes.TLSSettingAuto,
		User:                        "launch",
		Path:                        "/home/launch/example.test",
		WebFolder:                   "public",
		PhpVersion:                  &phpVersion,
		PendingTLSUpdateSince:       &now,
		PendingCaddyfileUpdateSince: &now,
	}
	site.ServerID = "server-1"
	site.TeamID = "team-1"
	site.UserID = "user-1"
	require.NoError(t, db.Create(site).Error)
	return db
}
