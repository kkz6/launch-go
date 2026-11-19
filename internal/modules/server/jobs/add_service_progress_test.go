package jobs

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/dto"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/repositories"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

func TestServiceInstallationRetainsTaskLogs(t *testing.T) {
	for _, fails := range []bool{false, true} {
		name := "success"
		if fails {
			name = "failure"
		}
		t.Run(name, func(t *testing.T) {
			job, service, db, dispatcher, broadcaster := newServiceInstallationTestJob(t)
			if fails {
				dispatcher.SetDefaultFailure(1, "installation failed")
			}
			err := job.Handle(context.Background())
			if fails {
				require.Error(t, err)
				job.Failed(context.Background(), err)
				require.NoError(t, db.First(service, "id = ?", service.ID).Error)
				require.Equal(t, types.ServiceStatusFailed, service.Status)
				require.True(t, broadcaster.hasEvent("service.status_changed", "failed"))
			} else {
				require.NoError(t, err)
				require.NoError(t, db.First(service, "id = ?", service.ID).Error)
				require.Equal(t, types.ServiceStatusRunning, service.Status)
			}
			response := dto.ToServiceResponse(service)
			require.NotNil(t, response.InstallTaskID)
			var tracked models.Task
			require.NoError(t, db.First(&tracked, "id = ?", *response.InstallTaskID).Error)
			require.Equal(t, service.ServerID, tracked.ServerID)
			require.NotNil(t, service.TypeData["extensions"], "linking logs must retain other service metadata")
			var count int64
			require.NoError(t, db.Model(&models.Task{}).Count(&count).Error)
			require.Equal(t, int64(1), count, "Prepare and Dispatch must use one tracked task")
		})
	}
}

func newServiceInstallationTestJob(t *testing.T) (*AddServiceJob, *models.InstalledService, *gorm.DB, *taskrunner.FakeDispatcher, *patchRecordingBroadcaster) {
	t.Helper()
	fixture, service, db, dispatcher, broadcaster := newPatchPhpVersionTestJob(t)
	service.Status, service.Software = types.ServiceStatusPending, "bun"
	require.NoError(t, db.Save(service).Error)
	fixture.Deps.Repos = repositories.NewRegistry(db)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	return &AddServiceJob{Deps: fixture.Deps, Payload: AddServicePayload{ServerID: service.ServerID, ServiceID: service.ID, Software: "bun"}}, service, db, dispatcher, broadcaster
}

func TestServiceInstallationPreparationFailureRetainsSafeDiagnostic(t *testing.T) {
	job, service, db, dispatcher, broadcaster := newServiceInstallationTestJob(t)
	privateError := errors.New("database unavailable https://token:private@example.com/private?password=secret")
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("fail_install_task_once", func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "tasks" {
			_ = tx.AddError(privateError)
		}
	}))
	err := job.Handle(context.Background())
	require.ErrorIs(t, err, privateError)
	require.NoError(t, db.Callback().Create().Remove("fail_install_task_once"))
	job.Failed(context.Background(), err)
	job.Failed(context.Background(), err)
	require.Zero(t, dispatcher.ExecutionCount())
	require.NoError(t, db.First(service, "id = ?", service.ID).Error)
	require.Equal(t, types.ServiceStatusFailed, service.Status)
	response := dto.ToServiceResponse(service)
	require.NotNil(t, response.InstallTaskID)
	var task models.Task
	require.NoError(t, db.First(&task, "id = ?", *response.InstallTaskID).Error)
	require.Equal(t, service.ServerID, task.ServerID)
	require.Equal(t, servicePreparationFailureMessage, task.Output.String())
	require.Equal(t, "failed", task.Status)
	require.Empty(t, task.Script)
	require.Empty(t, task.Instance)
	require.Nil(t, task.ExitCode, "no process executed during preparation")
	var raw string
	require.NoError(t, db.Raw("SELECT output FROM tasks WHERE id = ?", task.ID).Scan(&raw).Error)
	require.NotContains(t, raw, "Service installation preparation failed")
	var count int64
	require.NoError(t, db.Model(&models.Task{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
	require.True(t, broadcaster.hasEvent("service.status_changed", "failed"))
	require.True(t, broadcaster.hasEvent("task.updated", "failed"))
}

func TestServiceInstallationLinkFailureStopsDispatch(t *testing.T) {
	job, service, db, dispatcher, broadcaster := newServiceInstallationTestJob(t)
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("reject_install_task_link", func(tx *gorm.DB) {
		fields, ok := tx.Statement.Dest.(map[string]any)
		if ok && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "services" && fields["task_id"] != nil {
			_ = tx.AddError(errors.New("database private diagnostic"))
		}
	}))
	err := job.Handle(context.Background())
	require.ErrorContains(t, err, "persist service installation task")
	require.Zero(t, dispatcher.ExecutionCount())
	var task models.Task
	require.NoError(t, db.First(&task).Error)
	require.Equal(t, "failed", task.Status)
	require.Equal(t, "could not save the installation log; retry installation", task.Output.String())
	for _, event := range broadcaster.events {
		if event.event == "service.status_changed" {
			require.Empty(t, event.data["task_id"], "an unpersisted log link must not be advertised")
		}
	}
	require.NoError(t, db.Callback().Update().Remove("reject_install_task_link"))
	job.Failed(context.Background(), err)
	require.NoError(t, db.First(service, "id = ?", service.ID).Error)
	require.Equal(t, task.ID, service.TypeData["install_task_id"])
	require.Equal(t, types.ServiceStatusFailed, service.Status)
}

func TestServiceInstallationFailuresPreserveCurrentAttemptLog(t *testing.T) {
	for _, failure := range []string{"connection", "dispatch", "missing_dispatcher"} {
		t.Run(failure, func(t *testing.T) {
			job, service, db, dispatcher, broadcaster := newServiceInstallationTestJob(t)
			switch failure {
			case "connection":
				require.NoError(t, db.Model(&models.Server{}).Where("id = ?", service.ServerID).Update("public_ipv4", nil).Error)
			case "dispatch":
				dispatcher.SetRunError(errors.New("SSH handshake failed"))
			case "missing_dispatcher":
				job.Deps.TaskRunnerDeps.Dispatcher = nil
			}
			err := job.Handle(context.Background())
			require.Error(t, err)
			job.Failed(context.Background(), err)
			require.NoError(t, db.First(service, "id = ?", service.ID).Error)
			require.Equal(t, types.ServiceStatusFailed, service.Status)
			var task models.Task
			require.NoError(t, db.First(&task, "id = ?", service.TypeData["install_task_id"]).Error)
			require.Equal(t, "failed", task.Status)
			switch failure {
			case "connection":
				require.Contains(t, task.Output.String(), "server has no public IP address")
			case "dispatch":
				require.Equal(t, "SSH handshake failed", task.Output.String())
			case "missing_dispatcher":
				require.Equal(t, servicePreparationFailureMessage, task.Output.String())
				require.True(t, broadcaster.hasEvent("task.updated", "failed"))
			}
		})
	}
}

func TestServiceInstallationLateFailureCannotReplaceRetry(t *testing.T) {
	job, service, db, dispatcher, _ := newServiceInstallationTestJob(t)
	dispatcher.SetDefaultFailure(1, "first attempt failed")
	err := job.Handle(context.Background())
	require.Error(t, err)
	job.Failed(context.Background(), err)
	firstID := job.installationTaskID
	retry := &AddServiceJob{Deps: job.Deps, Payload: job.Payload}
	dispatcher.SetDefaultFailure(1, "second attempt failed")
	err = retry.Handle(context.Background())
	require.Error(t, err)
	retry.Failed(context.Background(), err)
	job.Failed(context.Background(), errors.New("late first failure"))
	require.NoError(t, db.First(service, "id = ?", service.ID).Error)
	require.NotEqual(t, firstID, service.TypeData["install_task_id"])
	require.Equal(t, retry.installationTaskID, service.TypeData["install_task_id"])
	var task models.Task
	require.NoError(t, db.First(&task, "id = ?", retry.installationTaskID).Error)
	require.Equal(t, "second attempt failed", task.Output.String())
	task = models.Task{}
	require.NoError(t, db.First(&task, "id = ?", firstID).Error)
	require.Equal(t, "first attempt failed", task.Output.String())
}

func TestServiceInstallationPreparationRetryGetsItsOwnDiagnostic(t *testing.T) {
	job, service, db, dispatcher, _ := newServiceInstallationTestJob(t)
	dispatcher.SetDefaultFailure(1, "original installation output")
	err := job.Handle(context.Background())
	require.Error(t, err)
	job.Failed(context.Background(), err)
	firstID := job.installationTaskID
	retry := &AddServiceJob{Deps: job.Deps, Payload: job.Payload}
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("fail_retry_task", func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "tasks" {
			_ = tx.AddError(errors.New("task preparation failed"))
		}
	}))
	err = retry.Handle(context.Background())
	require.ErrorContains(t, err, "prepare service installation")
	// A late callback arriving before the retry has linked its log is harmless.
	job.Failed(context.Background(), errors.New("late previous failure"))
	require.NoError(t, db.Callback().Create().Remove("fail_retry_task"))
	retry.Failed(context.Background(), err)
	require.NoError(t, db.First(service, "id = ?", service.ID).Error)
	require.NotEqual(t, firstID, service.TypeData["install_task_id"])
	var diagnostic models.Task
	require.NoError(t, db.First(&diagnostic, "id = ?", service.TypeData["install_task_id"]).Error)
	require.Equal(t, servicePreparationFailureMessage, diagnostic.Output.String())
	var original models.Task
	require.NoError(t, db.First(&original, "id = ?", firstID).Error)
	require.Equal(t, "original installation output", original.Output.String())
}

func TestServiceInstallationDuplicateDoesNotClaimActiveService(t *testing.T) {
	for _, status := range []types.ServiceStatus{types.ServiceStatusInstalling, types.ServiceStatusRunning} {
		t.Run(string(status), func(t *testing.T) {
			job, service, db, dispatcher, _ := newServiceInstallationTestJob(t)
			require.NoError(t, db.Model(service).Update("status", status).Error)
			require.NoError(t, job.Handle(context.Background()))
			require.Zero(t, dispatcher.ExecutionCount())
			var count int64
			require.NoError(t, db.Model(&models.Task{}).Count(&count).Error)
			require.Zero(t, count)
			require.NoError(t, db.First(service, "id = ?", service.ID).Error)
			require.Equal(t, status, service.Status)
		})
	}
}

func TestServiceInstallationPreparationDiagnosticIsAtomicAndScoped(t *testing.T) {
	for _, variant := range []string{"duplicate", "rollback", "wrong_server", "moved_account"} {
		t.Run(variant, func(t *testing.T) {
			job, service, db, dispatcher, _ := newServiceInstallationTestJob(t)
			// Invalid software fails before task creation, without template panics.
			job.Payload.Software = "unavailable-software"
			require.NoError(t, db.Model(service).Update("software", job.Payload.Software).Error)
			err := job.Handle(context.Background())
			require.ErrorContains(t, err, "software is invalid")
			switch variant {
			case "rollback":
				require.NoError(t, db.Callback().Update().Before("gorm:update").Register("reject_diagnostic_link", func(tx *gorm.DB) {
					if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "services" {
						_ = tx.AddError(errors.New("cannot save diagnostic link"))
					}
				}))
			case "wrong_server":
				job.Payload.ServerID = "another-server"
			case "moved_account":
				require.NoError(t, db.Model(&models.Server{}).Where("id = ?", service.ServerID).Update("team_id", "another-team").Error)
			}
			var wg sync.WaitGroup
			for i := 0; i < 6; i++ {
				copyJob := *job
				wg.Add(1)
				go func() { defer wg.Done(); copyJob.Failed(context.Background(), err) }()
			}
			wg.Wait()
			require.Zero(t, dispatcher.ExecutionCount())
			var count int64
			require.NoError(t, db.Model(&models.Task{}).Count(&count).Error)
			if variant == "duplicate" {
				require.Equal(t, int64(1), count)
			} else {
				require.Zero(t, count)
			}
		})
	}
}

func TestServiceInstallationRejectsMismatchedTarget(t *testing.T) {
	for _, variant := range []string{"server", "software"} {
		t.Run(variant, func(t *testing.T) {
			job, service, db, dispatcher, _ := newServiceInstallationTestJob(t)
			if variant == "server" {
				other := &models.Server{}
				other.ID, other.TeamID = "other-server", "other-team"
				require.NoError(t, db.Create(other).Error)
				job.Payload.ServerID = other.ID
			} else {
				job.Payload.Software = "node21"
			}
			err := job.Handle(context.Background())
			require.ErrorContains(t, err, "scope does not match")
			job.Failed(context.Background(), err)
			require.Zero(t, dispatcher.ExecutionCount())
			var count int64
			require.NoError(t, db.Model(&models.Task{}).Count(&count).Error)
			require.Zero(t, count)
			require.NoError(t, db.First(service, "id = ?", service.ID).Error)
			require.Equal(t, types.ServiceStatusPending, service.Status)
		})
	}
}
