package jobs

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/kkz6/launch-go/internal/database/serializers"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	serverrepos "github.com/kkz6/launch-go/internal/modules/server/repositories"
	servertasks "github.com/kkz6/launch-go/internal/modules/server/tasks"
	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	sitemodels "github.com/kkz6/launch-go/internal/modules/site/models"
	siterepos "github.com/kkz6/launch-go/internal/modules/site/repositories"
	sitetasks "github.com/kkz6/launch-go/internal/modules/site/tasks"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	basemodels "github.com/kkz6/launch-go/internal/pkg/models"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

func TestUpdateSitePHPVersionMigratesManagedPHPProcesses(t *testing.T) {
	jobDeps, dispatcher, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
	job := &UpdateSitePHPVersionJob{
		Deps: jobDeps,
		Payload: UpdateSitePHPVersionPayload{
			SiteID:          "site-1",
			PreviousVersion: "php83",
			Version:         "php84",
		},
	}

	err := job.Handle(context.Background())

	require.NoError(t, err)
	require.Equal(t, 1, dispatcher.ExecutionCount())
	require.Equal(t, "Switch example.test to PHP 8.4", dispatcher.LastExecution().Task.Name())

	var site sitemodels.Site
	require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
	require.NotNil(t, site.PhpVersion)
	require.Equal(t, sitetypes.PhpVersion84, *site.PhpVersion)
	require.Nil(t, site.PendingCaddyfileUpdateSince)
	require.Nil(t, site.PendingPhpVersion)

	var queue sitemodels.Queue
	require.NoError(t, db.First(&queue, "id = ?", "queue-1").Error)
	require.Equal(
		t,
		"php8.4 /home/launch/example.test/current/artisan queue:work",
		queue.Command,
	)

	var octane sitemodels.Queue
	require.NoError(t, db.First(&octane, "id = ?", "octane-1").Error)
	require.Equal(
		t,
		"php8.4 /home/launch/example.test/current/artisan octane:start --server=swoole --port=8000",
		octane.Command,
	)

	var inertia sitemodels.Queue
	require.NoError(t, db.First(&inertia, "id = ?", "inertia-1").Error)
	require.Equal(t, "node /home/launch/example.test/current/bootstrap/ssr/ssr.js", inertia.Command)

	var legacyQueue sitemodels.Queue
	require.NoError(t, db.First(&legacyQueue, "id = ?", "legacy-queue").Error)
	require.Equal(
		t,
		"php8.4 /home/launch/example.test/repository/artisan queue:work",
		legacyQueue.Command,
	)

	var cron servermodels.Cron
	require.NoError(t, db.First(&cron, "id = ?", "cron-1").Error)
	require.Contains(t, cron.GetCommand(), "&& php8.4 artisan schedule:run")

	var daemon servermodels.Daemon
	require.NoError(t, db.First(&daemon, "id = ?", "reverb-1").Error)
	require.True(t, strings.HasPrefix(daemon.Command, "php8.4 "))

	var task servermodels.Task
	require.NoError(t, db.First(&task).Error)
	require.Equal(t, "Switch example.test to PHP 8.4", task.Name)
	require.True(t, task.IsSuccessful())
}

func TestUpdateSitePHPVersionAcceptsFullInstalledPatchVersion(t *testing.T) {
	jobDeps, dispatcher, _ := phpVersionJobFixture(t, servertypes.ServiceStatusInstalled)
	job := &UpdateSitePHPVersionJob{
		Deps: jobDeps,
		Payload: UpdateSitePHPVersionPayload{
			SiteID:          "site-1",
			PreviousVersion: "php83",
			Version:         "php84",
		},
	}

	require.NoError(t, job.Handle(context.Background()))
	require.Equal(t, 1, dispatcher.ExecutionCount())
}

func TestUpdateSitePHPVersionMigratesLegacyNullVersionFromServerDefault(t *testing.T) {
	jobDeps, dispatcher, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
	require.NoError(t, db.Model(&sitemodels.Site{}).
		Where("id = ?", "site-1").
		Update("php_version", nil).Error)

	job := &UpdateSitePHPVersionJob{
		Deps: jobDeps,
		Payload: UpdateSitePHPVersionPayload{
			SiteID:                 "site-1",
			PreviousVersion:        "php83",
			PreviousVersionWasNull: true,
			Version:                "php84",
		},
	}

	require.NoError(t, job.Handle(context.Background()))
	require.Equal(t, 1, dispatcher.ExecutionCount())

	var site sitemodels.Site
	require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
	require.Equal(t, sitetypes.PhpVersion84, *site.PhpVersion)
	require.Nil(t, site.PendingPhpVersion)
}

func TestUpdateSitePHPVersionPinsLegacyGenericCommandsWhenSeriesIsUnchanged(t *testing.T) {
	jobDeps, dispatcher, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
	require.NoError(t, db.Model(&sitemodels.Site{}).
		Where("id = ?", "site-1").
		Update("php_version", nil).Error)
	require.NoError(t, db.Model(&sitemodels.Queue{}).
		Where("id = ?", "queue-1").
		Update(
			"command",
			"php /home/launch/example.test/current/artisan queue:work",
		).Error)
	require.NoError(t, db.Model(&sitemodels.Queue{}).
		Where("id = ?", "octane-1").
		Update(
			"command",
			"php8.4 /home/launch/example.test/current/artisan octane:start --server=swoole --port=8000",
		).Error)
	require.NoError(t, db.Model(&servermodels.Cron{}).
		Where("id = ?", "cron-1").
		Update(
			"command",
			dbtype.EncryptedString(
				"cd /home/launch/example.test/current && php artisan schedule:run",
			),
		).Error)
	require.NoError(t, db.Model(&servermodels.Daemon{}).
		Where("id = ?", "reverb-1").
		Update(
			"command",
			"php /home/launch/example.test/current/artisan reverb:start --port=6001",
		).Error)

	job := &UpdateSitePHPVersionJob{
		Deps: jobDeps,
		Payload: UpdateSitePHPVersionPayload{
			SiteID:                 "site-1",
			PreviousVersion:        "php84",
			PreviousVersionWasNull: true,
			Version:                "php84",
		},
	}

	require.NoError(t, job.Handle(context.Background()))
	require.Equal(t, 1, dispatcher.ExecutionCount())

	var queue sitemodels.Queue
	require.NoError(t, db.First(&queue, "id = ?", "queue-1").Error)
	require.Equal(
		t,
		"php8.4 /home/launch/example.test/current/artisan queue:work",
		queue.Command,
	)

	var cron servermodels.Cron
	require.NoError(t, db.First(&cron, "id = ?", "cron-1").Error)
	require.Equal(
		t,
		"cd /home/launch/example.test/current && php8.4 artisan schedule:run",
		cron.GetCommand(),
	)

	var daemon servermodels.Daemon
	require.NoError(t, db.First(&daemon, "id = ?", "reverb-1").Error)
	require.Equal(
		t,
		"php8.4 /home/launch/example.test/current/artisan reverb:start --port=6001",
		daemon.Command,
	)
}

func TestLegacyNullPHPTransitionRestoresNilCaddyRepresentation(t *testing.T) {
	jobDeps, _, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
	require.NoError(t, db.Model(&sitemodels.Site{}).
		Where("id = ?", "site-1").
		Update("php_version", nil).Error)

	var site sitemodels.Site
	require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
	var server servermodels.Server
	require.NoError(t, db.First(&server, "id = ?", "server-1").Error)

	job := &UpdateSitePHPVersionJob{
		Deps: jobDeps,
		Payload: UpdateSitePHPVersionPayload{
			SiteID:                 site.ID,
			PreviousVersion:        "php84",
			PreviousVersionWasNull: true,
			Version:                "php84",
		},
	}
	transition, err := job.buildRuntimeTransition(
		context.Background(),
		&site,
		&server,
		sitetypes.PhpVersion84,
	)
	require.NoError(t, err)

	caddyfilePath := site.Path + "/Caddyfile"
	var updateCaddyfile, rollbackCaddyfile string
	for _, file := range transition.UpdateConfig.Files {
		if file.Path == caddyfilePath {
			updateCaddyfile = file.Contents
			break
		}
	}
	for _, file := range transition.RollbackConfig.Files {
		if file.Path == caddyfilePath {
			rollbackCaddyfile = file.Contents
			break
		}
	}

	require.NotEmpty(t, updateCaddyfile)
	require.Contains(t, updateCaddyfile, "php_fastcgi unix//run/php/php8.4-fpm.sock")
	require.NotEmpty(t, rollbackCaddyfile)
	require.NotContains(t, rollbackCaddyfile, "php_fastcgi")
}

func TestUpdateSitePHPVersionRejectsLostReservation(t *testing.T) {
	jobDeps, dispatcher, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
	require.NoError(t, db.Model(&sitemodels.Site{}).
		Where("id = ?", "site-1").
		Updates(map[string]any{
			"pending_php_version":            "php82",
			"pending_caddyfile_update_since": time.Now().UTC(),
		}).Error)
	job := &UpdateSitePHPVersionJob{
		Deps: jobDeps,
		Payload: UpdateSitePHPVersionPayload{
			SiteID:          "site-1",
			PreviousVersion: "php83",
			Version:         "php84",
		},
	}

	err := job.Handle(context.Background())

	require.Error(t, err)
	require.Contains(t, err.Error(), "reservation changed")
	require.Zero(t, dispatcher.ExecutionCount())
}

func TestUpdateSitePHPVersionRejectsInactiveTarget(t *testing.T) {
	jobDeps, dispatcher, db := phpVersionJobFixture(t, servertypes.ServiceStatusStopped)
	job := &UpdateSitePHPVersionJob{
		Deps: jobDeps,
		Payload: UpdateSitePHPVersionPayload{
			SiteID:          "site-1",
			PreviousVersion: "php83",
			Version:         "php84",
		},
	}

	err := job.Handle(context.Background())

	require.Error(t, err)
	require.Contains(t, err.Error(), "not active")
	require.Zero(t, dispatcher.ExecutionCount())

	var site sitemodels.Site
	require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
	require.Equal(t, sitetypes.PhpVersion83, *site.PhpVersion)
}

func TestUpdateSitePHPVersionKeepsDatabaseOnRuntimeFailure(t *testing.T) {
	jobDeps, dispatcher, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
	dispatcher.SetDefaultFailure(1, "caddy validation failed")
	job := &UpdateSitePHPVersionJob{
		Deps: jobDeps,
		Payload: UpdateSitePHPVersionPayload{
			SiteID:          "site-1",
			PreviousVersion: "php83",
			Version:         "php84",
		},
	}

	err := job.Handle(context.Background())

	require.Error(t, err)
	require.Contains(t, err.Error(), "caddy validation failed")

	var site sitemodels.Site
	require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
	require.Equal(t, sitetypes.PhpVersion83, *site.PhpVersion)

	var queue sitemodels.Queue
	require.NoError(t, db.First(&queue, "id = ?", "queue-1").Error)
	require.True(t, strings.HasPrefix(queue.Command, "php8.3 "))
}

func TestUpdateSitePHPVersionFailureClearsPendingState(t *testing.T) {
	jobDeps, _, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
	job := &UpdateSitePHPVersionJob{
		Deps: jobDeps,
		Payload: UpdateSitePHPVersionPayload{
			SiteID:          "site-1",
			PreviousVersion: "php83",
			Version:         "php84",
		},
	}

	job.Failed(context.Background(), errors.New("ssh unavailable"))

	var site sitemodels.Site
	require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
	require.Nil(t, site.PendingCaddyfileUpdateSince)
	require.Nil(t, site.PendingPhpVersion)
	require.Equal(t, sitetypes.PhpVersion83, *site.PhpVersion)
}

func TestUpdateSitePHPVersionRollbackIsTracked(t *testing.T) {
	jobDeps, dispatcher, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
	var site sitemodels.Site
	require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
	var server servermodels.Server
	require.NoError(t, db.First(&server, "id = ?", "server-1").Error)
	job := &UpdateSitePHPVersionJob{
		Deps: jobDeps,
		Payload: UpdateSitePHPVersionPayload{
			SiteID:          site.ID,
			PreviousVersion: "php83",
			Version:         "php84",
		},
		site: &site,
	}

	err := job.rollbackRuntime(context.Background(), &server, sitetasks.UpdatePHPVersionConfig{
		SiteAddress: site.Address,
		Version:     "8.3",
		PHPBinary:   "php8.3",
		FPMService:  "php8.3-fpm",
		FPMSocket:   "/run/php/php8.3-fpm.sock",
	})

	require.NoError(t, err)
	require.Equal(t, 1, dispatcher.ExecutionCount())
	require.Equal(t, "Rollback example.test PHP update", dispatcher.LastExecution().Task.Name())
}

func TestPersistPHPTransitionRejectsConcurrentQueueEdit(t *testing.T) {
	jobDeps, _, db := phpVersionJobFixture(t, servertypes.ServiceStatusRunning)
	var site sitemodels.Site
	require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
	var server servermodels.Server
	require.NoError(t, db.First(&server, "id = ?", "server-1").Error)
	target := sitetypes.PhpVersion84
	job := &UpdateSitePHPVersionJob{
		Deps: jobDeps,
		Payload: UpdateSitePHPVersionPayload{
			SiteID:          site.ID,
			PreviousVersion: "php83",
			Version:         target.String(),
		},
		site:   &site,
		server: &server,
	}

	transition, err := job.buildRuntimeTransition(context.Background(), &site, &server, target)
	require.NoError(t, err)
	require.NoError(t, db.Model(&sitemodels.Queue{}).
		Where("id = ?", "queue-1").
		Update("command", "php8.3 artisan custom-worker").Error)

	err = job.persistTransition(context.Background(), &site, target, transition)

	require.Error(t, err)
	require.Contains(t, err.Error(), "queue queue-1 changed concurrently")
	require.NoError(t, db.First(&site, "id = ?", "site-1").Error)
	require.Equal(t, sitetypes.PhpVersion83, *site.PhpVersion)
}

func TestUpdateSitePHPVersionTaskPayload(t *testing.T) {
	userID := "user-1"
	task, err := NewUpdateSitePHPVersionTask("site-1", "php83", "php84", false, &userID)
	require.NoError(t, err)
	require.Equal(t, TypeUpdateSitePHPVersion, task.Type())

	payload, err := pkgjobs.UnmarshalPayload[UpdateSitePHPVersionPayload](task)
	require.NoError(t, err)
	require.Equal(t, "site-1", payload.SiteID)
	require.Equal(t, "php83", payload.PreviousVersion)
	require.False(t, payload.PreviousVersionWasNull)
	require.Equal(t, "php84", payload.Version)
	require.Equal(t, &userID, payload.UserID)
}

func TestReplacePHPExecutableOnlyReplacesExecutableToken(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string
		changed bool
	}{
		{
			name:    "prefix",
			command: "php8.3 /srv/app/artisan horizon",
			want:    "php8.4 /srv/app/artisan horizon",
			changed: true,
		},
		{
			name:    "after shell operator",
			command: "cd /srv/app && php8.3 artisan schedule:run",
			want:    "cd /srv/app && php8.4 artisan schedule:run",
			changed: true,
		},
		{
			name:    "legacy unversioned executable",
			command: "php /srv/app/artisan queue:work",
			want:    "php8.4 /srv/app/artisan queue:work",
			changed: true,
		},
		{
			name:    "legacy unversioned executable after shell operator",
			command: "cd /srv/app && php artisan schedule:run",
			want:    "cd /srv/app && php8.4 artisan schedule:run",
			changed: true,
		},
		{
			name:    "path text is untouched",
			command: "node /srv/php8.3/app.js",
			want:    "node /srv/php8.3/app.js",
			changed: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, changed := replacePHPExecutable(test.command, "php8.3", "php8.4")
			require.Equal(t, test.want, got)
			require.Equal(t, test.changed, changed)
		})
	}

	t.Run("same resolved series still pins generic executable", func(t *testing.T) {
		got, changed := replacePHPExecutable(
			"php /srv/app/artisan queue:work",
			"php8.4",
			"php8.4",
		)
		require.Equal(t, "php8.4 /srv/app/artisan queue:work", got)
		require.True(t, changed)
	})

	t.Run("same resolved series leaves versioned executable unchanged", func(t *testing.T) {
		command := "php8.4 /srv/app/artisan queue:work"
		got, changed := replacePHPExecutable(command, "php8.4", "php8.4")
		require.Equal(t, command, got)
		require.False(t, changed)
	})
}

func phpVersionJobFixture(
	t *testing.T,
	targetStatus servertypes.ServiceStatus,
) (*JobDeps, *taskrunner.FakeDispatcher, *gorm.DB) {
	t.Helper()
	require.NoError(t, serializers.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")))

	db, err := gorm.Open(
		sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"),
		&gorm.Config{
			DisableForeignKeyConstraintWhenMigrating: true,
			Logger:                                   gormlogger.Default.LogMode(gormlogger.Silent),
		},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&servermodels.Server{},
		&servermodels.InstalledService{},
		&servermodels.Task{},
		&servermodels.Cron{},
		&servermodels.Daemon{},
		&sitemodels.Site{},
		&sitemodels.Queue{},
		&sitemodels.Certificate{},
		&sitemodels.Redirect{},
	))

	ip := "192.0.2.10"
	server := &servermodels.Server{
		BaseModel:  basemodels.BaseModel{ID: "server-1"},
		Name:       "Test server",
		PublicIPv4: &ip,
		PrivateKey: dbtype.EncryptedString(
			"-----BEGIN OPENSSH PRIVATE KEY-----\ntest\n-----END OPENSSH PRIVATE KEY-----",
		),
	}
	server.TeamID = "team-1"
	server.UserID = "user-1"
	require.NoError(t, db.Create(server).Error)

	targetService := &servermodels.InstalledService{
		BaseModel: basemodels.BaseModel{ID: "php84-service"},
		Type:      servertypes.ServiceTypePhp,
		Software:  servertypes.SoftwarePhp84.String(),
		Version:   "8.4.11",
		Status:    targetStatus,
	}
	targetService.ServerID = server.ID
	require.NoError(t, db.Create(targetService).Error)

	now := time.Now().UTC()
	php83 := sitetypes.PhpVersion83
	php84 := sitetypes.PhpVersion84
	daemonID := "reverb-1"
	site := &sitemodels.Site{
		BaseModel:                   basemodels.BaseModel{ID: "site-1"},
		Address:                     "example.test",
		Type:                        sitetypes.SiteTypeLaravel,
		TLSSetting:                  sitetypes.TLSSettingAuto,
		User:                        "launch",
		Path:                        "/home/launch/example.test",
		WebFolder:                   "public",
		PhpVersion:                  &php83,
		PendingPhpVersion:           &php84,
		PendingCaddyfileUpdateSince: &now,
		EnabledFeatures: sitemodels.EnabledFeaturesSlice{
			{Name: FeatureReverb, DaemonID: &daemonID},
		},
	}
	site.InstalledAt = &now
	site.ServerID = server.ID
	site.TeamID = server.TeamID
	site.UserID = "user-1"
	require.NoError(t, db.Create(site).Error)

	createPHPVersionQueue(
		t,
		db,
		"queue-1",
		"php8.3 /home/launch/example.test/current/artisan queue:work",
		&now,
	)
	createPHPVersionQueue(
		t,
		db,
		"octane-1",
		"php8.3 /home/launch/example.test/current/artisan octane:start --server=swoole --port=8000",
		&now,
	)
	createPHPVersionQueue(
		t,
		db,
		"inertia-1",
		"node /home/launch/example.test/current/bootstrap/ssr/ssr.js",
		&now,
	)
	createPHPVersionQueue(t, db, "legacy-queue", "", &now)

	cron := &servermodels.Cron{
		BaseModel:  basemodels.BaseModel{ID: "cron-1"},
		SiteID:     &site.ID,
		User:       site.User,
		Expression: "* * * * *",
		Command: dbtype.EncryptedString(
			"cd /home/launch/example.test/current && php8.3 artisan schedule:run",
		),
		Frequency: "Every minute",
	}
	cron.ServerID = server.ID
	cron.InstalledAt = &now
	require.NoError(t, db.Create(cron).Error)

	daemon := &servermodels.Daemon{
		BaseModel:       basemodels.BaseModel{ID: daemonID},
		Command:         "php8.3 /home/launch/example.test/current/artisan reverb:start --port=6001",
		User:            site.User,
		Processes:       1,
		StopWaitSeconds: 10,
		StopSignal:      "TERM",
	}
	daemon.ServerID = server.ID
	daemon.InstalledAt = &now
	require.NoError(t, db.Create(daemon).Error)

	dispatcher := taskrunner.NewFakeDispatcher()
	logger := zerolog.Nop()
	jobDeps := &JobDeps{
		Deps: &pkgjobs.Deps{
			DB:     db,
			Logger: &logger,
		},
		Repos:       siterepos.NewRegistry(db),
		ServerRepos: serverrepos.NewRegistry(db),
		TaskRunnerDeps: &servertasks.TaskRunnerDeps{
			DB:         db,
			Dispatcher: dispatcher,
			Logger:     &logger,
		},
	}
	return jobDeps, dispatcher, db
}

func createPHPVersionQueue(
	t *testing.T,
	db *gorm.DB,
	id,
	command string,
	installedAt *time.Time,
) {
	t.Helper()
	queue := &sitemodels.Queue{
		BaseModel:       basemodels.BaseModel{ID: id},
		Command:         command,
		User:            "launch",
		QueueName:       "default",
		StopSignal:      "TERM",
		QueueConnection: "redis",
		NumProcs:        1,
	}
	queue.SiteID = "site-1"
	queue.ServerID = "server-1"
	queue.TeamID = "team-1"
	queue.UserID = "user-1"
	queue.InstalledAt = installedAt
	require.NoError(t, db.Create(queue).Error)
}
