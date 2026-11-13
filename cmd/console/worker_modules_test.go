package main

import (
	"testing"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/config"
	"github.com/kkz6/launch-go/internal/modules/certificate"
	certjobs "github.com/kkz6/launch-go/internal/modules/certificate/jobs"
	dockerjobs "github.com/kkz6/launch-go/internal/modules/docker/jobs"
	"github.com/kkz6/launch-go/internal/pkg/app"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
	"github.com/kkz6/launch-go/internal/schedule"
)

func bootTestWorker(t *testing.T) (*app.Kernel, *asynq.ServeMux, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		taskrunner.RegisterHostKeyPersister(nil)
		require.NoError(t, sqlDB.Close())
	})
	// Recovery runs during worker construction; an empty table keeps startup
	// realistic without launching SSH tasks or requiring external services.
	require.NoError(t, db.Exec("CREATE TABLE tasks (id TEXT PRIMARY KEY, server_id TEXT, status TEXT)").Error)
	logger := zerolog.Nop()
	kernel := newWorkerKernel(&config.Config{}, db, &logger, nil, nil, nil)
	mux := asynq.NewServeMux()
	kernel.BootJobs(mux)
	return kernel, mux, db
}

func TestWorkerKernelHandlesEveryScheduledTask(t *testing.T) {
	_, mux, _ := bootTestWorker(t)
	scheduledTasks := schedule.GetScheduledTasks()
	require.NotEmpty(t, scheduledTasks)
	for _, scheduled := range scheduledTasks {
		t.Run(scheduled.Name, func(t *testing.T) {
			require.NotNil(t, scheduled.Task, "scheduled task construction must succeed")
			_, pattern := mux.Handler(scheduled.Task)
			assert.Equal(t, scheduled.Task.Type(), pattern, "scheduled job must have a worker handler")
		})
	}
}

func TestWorkerKernelHandlesCertificateFanout(t *testing.T) {
	_, mux, _ := bootTestWorker(t)
	task, err := certjobs.NewFanoutCertificateTask("certificate-1", "team-1")
	require.NoError(t, err)
	_, pattern := mux.Handler(task)
	assert.Equal(t, task.Type(), pattern, "certificate rotations must have a worker handler")
}

func TestWorkerKernelSharesCertificateRepositoriesWithDockerJobs(t *testing.T) {
	kernel, _, db := bootTestWorker(t)
	application := dockerjobs.NewSyncTraefikConfigJob(dockerjobs.SyncTraefikConfigPayload{}).(*dockerjobs.SyncTraefikConfigJob)
	compose := dockerjobs.NewSyncComposeTraefikConfigJob(dockerjobs.SyncComposeTraefikConfigPayload{}).(*dockerjobs.SyncComposeTraefikConfigJob)
	for name, deps := range map[string]*dockerjobs.JobDeps{
		"application": application.Deps,
		"compose":     compose.Deps,
	} {
		t.Run(name, func(t *testing.T) {
			require.NotNil(t, deps)
			require.NotNil(t, deps.CertRepos, "stored certificate domains need certificate lookup in the worker")
			module, ok := kernel.GetModule(certificate.ModuleName).(*certificate.Module)
			require.True(t, ok, "certificate module must be registered in the worker")
			assert.Same(t, module.Repos(), deps.CertRepos)
			assert.Same(t, db, deps.DB)
		})
	}
	fanout := certjobs.NewFanoutCertificateJob(certjobs.FanoutCertificatePayload{}).(*certjobs.FanoutCertificateJob)
	require.NotNil(t, fanout.Deps)
	assert.Same(t, application.Deps.CertRepos, fanout.Deps.Repos)
}
