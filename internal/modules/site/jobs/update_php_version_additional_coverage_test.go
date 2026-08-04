package jobs

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	sitemodels "github.com/kkz6/launch-go/internal/modules/site/models"
	sitetasks "github.com/kkz6/launch-go/internal/modules/site/tasks"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
)

func TestUpdateSitePHPVersionConstructorAndSmallHelpers(t *testing.T) {
	payload := UpdateSitePHPVersionPayload{SiteID: "site-1", Version: "php84"}
	assert.IsType(t, &UpdateSitePHPVersionJob{}, NewUpdateSitePHPVersionJob(payload))

	command := "php8.3 artisan queue:work"
	got, changed := replacePHPExecutable(command, "", "php8.4")
	assert.Equal(t, command, got)
	assert.False(t, changed)

	daemonID := "daemon-1"
	emptyID := ""
	site := &sitemodels.Site{EnabledFeatures: sitemodels.EnabledFeaturesSlice{
		{Name: "no daemon"},
		{Name: "empty", DaemonID: &emptyID},
		{Name: "first", DaemonID: &daemonID},
		{Name: "duplicate", DaemonID: &daemonID},
	}}
	assert.Equal(t, []string{daemonID}, enabledFeatureDaemonIDs(site))

	userID := "user-1"
	assert.Equal(t, userID, stringValue(&userID))
}

func TestUpdateSitePHPVersionHandleValidationFailures(t *testing.T) {
	t.Run("invalid target version", func(t *testing.T) {
		jobDeps, dispatcher, _ := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
		job := &UpdateSitePHPVersionJob{
			Deps: jobDeps,
			Payload: UpdateSitePHPVersionPayload{
				SiteID: "site-1", PreviousVersion: "php83", Version: "invalid",
			},
		}
		require.Error(t, job.Handle(context.Background()))
		assert.Zero(t, dispatcher.ExecutionCount())
	})

	t.Run("site lookup", func(t *testing.T) {
		jobDeps, dispatcher, _ := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
		job := &UpdateSitePHPVersionJob{
			Deps: jobDeps,
			Payload: UpdateSitePHPVersionPayload{
				SiteID: "missing", PreviousVersion: "php83", Version: "php84",
			},
		}
		err := job.Handle(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to find site")
		assert.Zero(t, dispatcher.ExecutionCount())
	})

	t.Run("already applied clears reservation", func(t *testing.T) {
		jobDeps, dispatcher, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
		require.NoError(t, db.Model(&sitemodels.Site{}).
			Where("id = ?", "site-1").
			Update("php_version", sitetypes.PhpVersion84.String()).Error)
		job := &UpdateSitePHPVersionJob{
			Deps: jobDeps,
			Payload: UpdateSitePHPVersionPayload{
				SiteID: "site-1", PreviousVersion: "php83", Version: "php84",
			},
		}

		require.NoError(t, job.Handle(context.Background()))
		assert.Zero(t, dispatcher.ExecutionCount())
		var site sitemodels.Site
		require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
		assert.Nil(t, site.PendingPhpVersion)
		assert.Nil(t, site.PendingCaddyfileUpdateSince)
	})

	for _, tc := range []struct {
		name            string
		previousWasNull bool
		current         any
	}{
		{name: "version changed", current: sitetypes.PhpVersion82.String()},
		{name: "null expectation changed", previousWasNull: true, current: sitetypes.PhpVersion83.String()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			jobDeps, dispatcher, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
			require.NoError(t, db.Model(&sitemodels.Site{}).
				Where("id = ?", "site-1").Update("php_version", tc.current).Error)
			job := &UpdateSitePHPVersionJob{
				Deps: jobDeps,
				Payload: UpdateSitePHPVersionPayload{
					SiteID:                 "site-1",
					PreviousVersion:        "php83",
					PreviousVersionWasNull: tc.previousWasNull,
					Version:                "php84",
				},
			}
			err := job.Handle(context.Background())
			require.Error(t, err)
			assert.Contains(t, err.Error(), "changed while update was queued")
			assert.Zero(t, dispatcher.ExecutionCount())
		})
	}

	t.Run("server lookup", func(t *testing.T) {
		jobDeps, dispatcher, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
		require.NoError(t, db.Delete(&servermodels.Server{}, "id = ?", "server-1").Error)
		job := newUpdatePHPVersionCoverageJob(jobDeps)
		err := job.Handle(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to find server")
		assert.Zero(t, dispatcher.ExecutionCount())
	})

	t.Run("target service lookup", func(t *testing.T) {
		jobDeps, dispatcher, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
		require.NoError(t, db.Delete(&servermodels.InstalledService{}, "id = ?", "php84-service").Error)
		job := newUpdatePHPVersionCoverageJob(jobDeps)
		err := job.Handle(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not installed")
		assert.Zero(t, dispatcher.ExecutionCount())
	})

	t.Run("target is not PHP", func(t *testing.T) {
		jobDeps, dispatcher, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
		require.NoError(t, db.Model(&servermodels.InstalledService{}).
			Where("id = ?", "php84-service").
			Update("type", servertypes.ServiceTypeRedis).Error)
		job := newUpdatePHPVersionCoverageJob(jobDeps)
		err := job.Handle(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not active")
		assert.Zero(t, dispatcher.ExecutionCount())
	})
}

func TestUpdateSitePHPVersionTracksDispatcherErrors(t *testing.T) {
	jobDeps, dispatcher, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
	dispatcher.SetRunError(errors.New("ssh transport failed"))
	job := newUpdatePHPVersionCoverageJob(jobDeps)

	err := job.Handle(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update site PHP runtime")
	var tracked servermodels.Task
	require.NoError(t, db.First(&tracked).Error)
	assert.Equal(t, string(servertypes.TaskStatusFailed), tracked.Status)
	assert.Contains(t, tracked.Output.String(), "ssh transport failed")
}

func TestUpdateSitePHPVersionRollsBackRuntimeWhenPersistenceFails(t *testing.T) {
	for _, tc := range []struct {
		name             string
		rollbackExitCode int
		rollbackOutput   string
		want             string
	}{
		{name: "rollback succeeds", want: "runtime was rolled back"},
		{name: "rollback fails", rollbackExitCode: 12, rollbackOutput: "rollback exploded", want: "runtime rollback also failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			jobDeps, dispatcher, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
			require.NoError(t, db.Exec(`
				CREATE TRIGGER fail_php_queue_persistence
				BEFORE UPDATE OF command ON queues
				WHEN OLD.id = 'queue-1'
				BEGIN SELECT RAISE(FAIL, 'persist failed'); END
			`).Error)
			if tc.rollbackExitCode != 0 {
				dispatcher.SetFailure(
					"Rollback example.test PHP update",
					tc.rollbackExitCode,
					tc.rollbackOutput,
				)
			}
			job := newUpdatePHPVersionCoverageJob(jobDeps)

			err := job.Handle(context.Background())

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
			assert.Equal(t, 2, dispatcher.ExecutionCount())
			var site sitemodels.Site
			require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
			assert.Equal(t, sitetypes.PhpVersion83, *site.PhpVersion)
		})
	}
}

func TestUpdateSitePHPVersionRollbackFailureModes(t *testing.T) {
	jobDeps, dispatcher, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
	var site sitemodels.Site
	require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
	var server servermodels.Server
	require.NoError(t, db.First(&server, "id = ?", "server-1").Error)
	job := newUpdatePHPVersionCoverageJob(jobDeps)
	job.site = &site
	config := sitetasks.UpdatePHPVersionConfig{SiteAddress: site.Address, Version: "8.3"}

	dispatcher.SetRunError(errors.New("rollback transport failed"))
	err := job.rollbackRuntime(context.Background(), &server, config)
	assert.ErrorContains(t, err, "rollback transport failed")

	dispatcher.Reset()
	dispatcher.SetFailure("Rollback example.test PHP update", 7, "restore script failed")
	err = job.rollbackRuntime(context.Background(), &server, config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rollback failed: restore script failed")
}

func TestBuildPHPVersionTransitionRepositoryFailures(t *testing.T) {
	t.Run("invalid previous version", func(t *testing.T) {
		jobDeps, _, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
		job := newUpdatePHPVersionCoverageJob(jobDeps)
		job.Payload.PreviousVersion = "invalid"
		var site sitemodels.Site
		require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
		var server servermodels.Server
		require.NoError(t, db.First(&server, "id = ?", "server-1").Error)
		_, err := job.buildRuntimeTransition(context.Background(), &site, &server, sitetypes.PhpVersion84)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid previous PHP version")
	})

	for _, tc := range []struct {
		name      string
		prepare   func(*testing.T, *UpdateSitePHPVersionJob, *sitemodels.Site, *servermodels.Server)
		wantError string
	}{
		{
			name: "installed redirects lookup",
			prepare: func(t *testing.T, job *UpdateSitePHPVersionJob, _ *sitemodels.Site, _ *servermodels.Server) {
				require.NoError(t, job.Deps.DB.Migrator().DropTable(&sitemodels.Redirect{}))
			},
			wantError: "find installed redirects",
		},
		{
			name: "active certificate lookup",
			prepare: func(t *testing.T, job *UpdateSitePHPVersionJob, _ *sitemodels.Site, _ *servermodels.Server) {
				require.NoError(t, job.Deps.DB.Migrator().DropTable(&sitemodels.Certificate{}))
			},
			wantError: "find active certificate",
		},
		{
			name: "queue lookup",
			prepare: func(t *testing.T, job *UpdateSitePHPVersionJob, site *sitemodels.Site, _ *servermodels.Server) {
				site.InstalledAt = nil
				require.NoError(t, job.Deps.DB.Migrator().DropTable(&sitemodels.Queue{}))
			},
			wantError: "find site queues",
		},
		{
			name: "cron lookup",
			prepare: func(t *testing.T, job *UpdateSitePHPVersionJob, site *sitemodels.Site, _ *servermodels.Server) {
				site.InstalledAt = nil
				require.NoError(t, job.Deps.DB.Migrator().DropTable(&servermodels.Cron{}))
			},
			wantError: "find server crons",
		},
		{
			name: "daemon lookup",
			prepare: func(_ *testing.T, _ *UpdateSitePHPVersionJob, site *sitemodels.Site, _ *servermodels.Server) {
				site.InstalledAt = nil
				missing := "missing-daemon"
				site.EnabledFeatures = sitemodels.EnabledFeaturesSlice{{Name: FeatureReverb, DaemonID: &missing}}
			},
			wantError: "find site daemon missing-daemon",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			jobDeps, _, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
			job := newUpdatePHPVersionCoverageJob(jobDeps)
			var site sitemodels.Site
			require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
			var server servermodels.Server
			require.NoError(t, db.First(&server, "id = ?", "server-1").Error)
			tc.prepare(t, job, &site, &server)

			_, err := job.buildRuntimeTransition(context.Background(), &site, &server, sitetypes.PhpVersion84)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantError)
		})
	}
}

func TestPersistPHPVersionTransitionAdditionalFailures(t *testing.T) {
	t.Run("database required", func(t *testing.T) {
		jobDeps, _, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
		var site sitemodels.Site
		require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
		job := newUpdatePHPVersionCoverageJob(jobDeps)
		job.Deps.DB = nil
		err := job.persistTransition(
			context.Background(), &site, sitetypes.PhpVersion84, &phpRuntimeTransition{},
		)
		assert.ErrorContains(t, err, "database is not configured")
	})

	t.Run("site changed concurrently", func(t *testing.T) {
		jobDeps, _, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
		var site sitemodels.Site
		require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
		require.NoError(t, db.Model(&sitemodels.Site{}).
			Where("id = ?", site.ID).Update("pending_php_version", "php82").Error)
		job := newUpdatePHPVersionCoverageJob(jobDeps)
		err := job.persistTransition(
			context.Background(), &site, sitetypes.PhpVersion84, &phpRuntimeTransition{},
		)
		assert.ErrorContains(t, err, "site PHP version changed concurrently")
	})

	t.Run("cron changed concurrently", func(t *testing.T) {
		jobDeps, _, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
		var site sitemodels.Site
		require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
		job := newUpdatePHPVersionCoverageJob(jobDeps)
		transition := &phpRuntimeTransition{Crons: []commandChange{{
			ID: "cron-1", PreviousCommand: "different", Command: "php8.4 artisan schedule:run",
		}}}
		err := job.persistTransition(context.Background(), &site, sitetypes.PhpVersion84, transition)
		assert.ErrorContains(t, err, "cron cron-1 changed concurrently")
	})

	t.Run("daemon changed concurrently", func(t *testing.T) {
		jobDeps, _, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
		var site sitemodels.Site
		require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
		job := newUpdatePHPVersionCoverageJob(jobDeps)
		transition := &phpRuntimeTransition{Daemons: []commandChange{{
			ID: "reverb-1", PreviousCommand: "different", Command: "php8.4 artisan reverb:start",
		}}}
		err := job.persistTransition(context.Background(), &site, sitetypes.PhpVersion84, transition)
		assert.ErrorContains(t, err, "daemon reverb-1 changed concurrently")
	})
}

func TestUpdateSitePHPVersionCleanupWithoutDatabaseOrModels(t *testing.T) {
	jobDeps, _, _ := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
	job := newUpdatePHPVersionCoverageJob(jobDeps)
	job.Deps.DB = nil
	require.NoError(t, job.clearPendingUpdate(context.Background()))

	job.Payload.SiteID = "missing"
	job.Failed(context.Background(), errors.New("update failed"))
}

func newUpdatePHPVersionCoverageJob(jobDeps *JobDeps) *UpdateSitePHPVersionJob {
	return &UpdateSitePHPVersionJob{
		Deps: jobDeps,
		Payload: UpdateSitePHPVersionPayload{
			SiteID:          "site-1",
			PreviousVersion: "php83",
			Version:         "php84",
		},
	}
}
