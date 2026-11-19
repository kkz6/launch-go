package jobs

import (
	"context"
	"errors"
	"fmt"

	"github.com/hibiken/asynq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/launch/status"
	"github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

const TypeAddService = "server:add_service"

type AddServicePayload struct {
	ServerID  string `json:"server_id"`
	ServiceID string `json:"service_id"`
	Software  string `json:"software"`
}

type AddServiceJob struct {
	Deps    *JobDeps
	Payload AddServicePayload

	server                *models.Server
	service               *models.InstalledService
	previousInstallTaskID string
	installationTaskID    string
}

func NewAddServiceJob(p AddServicePayload) pkgjobs.Handler {
	return &AddServiceJob{Deps: deps, Payload: p}
}

func (j *AddServiceJob) Handle(ctx context.Context) error {
	var err error

	j.service, err = j.Deps.Repos.Service().FindByID(ctx, j.Payload.ServiceID)
	if err != nil {
		return fmt.Errorf("find service: %w", err)
	}
	j.previousInstallTaskID, _ = j.service.TypeData["install_task_id"].(string)

	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("find server: %w", err)
	}
	if j.service.ServerID != j.server.ID || j.service.Software != j.Payload.Software {
		return errors.New("service installation scope does not match")
	}
	if !types.Software(j.Payload.Software).IsValid() {
		return errors.New("service installation software is invalid")
	}
	if j.service.Status != types.ServiceStatusPending && j.service.Status != types.ServiceStatusFailed {
		return nil // Another attempt or lifecycle operation already owns this service.
	}

	software := types.Software(j.Payload.Software)

	memoryInMB := 0
	if j.server.MemoryInMB != nil {
		memoryInMB = *j.server.MemoryInMB
	}

	publicIPv4 := ""
	if j.server.PublicIPv4 != nil {
		publicIPv4 = *j.server.PublicIPv4
	}

	task := tasks.InstallSoftware(software, tasks.SoftwareInstallConfig{
		Username:         j.server.GetUsername(),
		MemoryInMB:       memoryInMB,
		DatabasePassword: j.server.DatabasePassword.String(),
		DatabaseName:     j.server.GetUsername(),
		PublicIPv4:       publicIPv4,
	})

	// Prepare and persist the log target before dispatch. A failed association
	// must never leave a remote installation running without an accessible log.
	runner := j.Deps.RunTask(j.server, task).AsRoot().TrackInDB()
	prepared, err := runner.Prepare(ctx)
	if err != nil {
		return fmt.Errorf("prepare service installation: %w", err)
	}
	j.installationTaskID = prepared.ID
	err = j.withInstallationScope(ctx, func(tx *gorm.DB, _ *models.Server, service *models.InstalledService) error {
		current, _ := service.TypeData["install_task_id"].(string)
		if current != j.previousInstallTaskID || (service.Status != types.ServiceStatusPending && service.Status != types.ServiceStatusFailed) {
			return errors.New("service installation changed before linking task")
		}
		if err := linkServiceInstallationTask(tx, service, prepared.ID); err != nil {
			return err
		}
		return tx.Model(service).Update("status", types.ServiceStatusInstalling).Error
	})
	if err != nil {
		// Never copy database/provider errors into a user-visible task log.
		_ = j.Deps.TaskRunnerDeps.FailPreparedTask(ctx, prepared.ID, errors.New("could not save the installation log; retry installation"))
		return fmt.Errorf("persist service installation task: %w", err)
	}
	j.Deps.BroadcastServerEvent(j.server, "service.status_changed", map[string]any{
		"service_id": j.service.ID, "status": string(types.ServiceStatusInstalling), "task_id": prepared.ID,
	})
	result, err := runner.Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("install service: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("install service: %s", result.GetOutput())
	}

	if err := j.withInstallationScope(ctx, func(tx *gorm.DB, _ *models.Server, service *models.InstalledService) error {
		current, _ := service.TypeData["install_task_id"].(string)
		if current != j.installationTaskID || service.Status != types.ServiceStatusInstalling {
			return errors.New("service installation changed before completion")
		}
		return tx.Model(service).Update("status", types.ServiceStatusRunning).Error
	}); err != nil {
		return fmt.Errorf("update service status: %w", err)
	}

	// Detect + persist the REAL installed version now, so the row never
	// shows the install-time placeholder/static default (e.g. supervisor
	// "latest", docker "27.3"). Best-effort: the live status probe also
	// self-heals this on the Services tab, so a probe failure here is not
	// fatal to the install.
	j.persistInstalledVersion(ctx)

	j.Deps.Logger.Info().
		Str("service_id", j.service.ID).
		Str("server_id", j.server.ID).
		Str("software", j.Payload.Software).
		Msg("service installed successfully")

	j.Deps.BroadcastServerEvent(j.server, "service.installed", map[string]any{
		"service_id": j.service.ID,
		"server_id":  j.server.ID,
		"software":   j.Payload.Software,
	})

	return nil
}

// persistInstalledVersion probes the freshly-installed software for its
// real version over SSH and writes it to the service row, replacing the
// install-time placeholder/static default. Best-effort: any failure is
// logged and ignored (the live status probe self-heals later).
func (j *AddServiceJob) persistInstalledVersion(ctx context.Context) {
	vcmd := status.VersionCommand(j.Payload.Software)
	if vcmd == "" {
		return
	}
	probe := taskrunner.NewBaseTask(
		taskrunner.WithName("Probe "+j.Payload.Software+" version"),
		taskrunner.WithScript(vcmd),
		taskrunner.WithTimeoutSeconds(30),
	)
	res, err := j.Deps.RunTask(j.server, probe).WithoutTracking().AsRoot().Run(ctx)
	if err != nil || res == nil {
		return
	}
	v := status.ParseVersion(res.GetOutput())
	if v == "" {
		return
	}
	if err := j.Deps.Repos.Service().UpdateFields(ctx, j.service.ID, map[string]any{"version": v}); err != nil {
		j.Deps.Logger.Warn().Err(err).
			Str("service_id", j.service.ID).
			Str("version", v).
			Msg("failed to persist detected service version")
	}
}

func (j *AddServiceJob) Failed(ctx context.Context, err error) {
	j.Deps.Logger.Error().Err(err).
		Str("service_id", j.Payload.ServiceID).
		Str("server_id", j.Payload.ServerID).
		Msg("failed to install service")

	var eventServer *models.Server
	var changedTask *models.Task
	var taskID string
	updateErr := j.withInstallationScope(ctx, func(tx *gorm.DB, server *models.Server, service *models.InstalledService) error {
		current, _ := service.TypeData["install_task_id"].(string)
		if current != j.previousInstallTaskID && current != j.installationTaskID {
			return nil // A newer installation owns the service and its log.
		}
		if service.Status != types.ServiceStatusPending && service.Status != types.ServiceStatusInstalling &&
			!(service.Status == types.ServiceStatusFailed && current == j.previousInstallTaskID && (j.installationTaskID == "" || j.installationTaskID != current)) {
			return nil
		}
		if current != j.installationTaskID && service.Status == types.ServiceStatusInstalling {
			return nil // This invocation did not claim the active installation.
		}
		taskID = j.installationTaskID
		if taskID == "" {
			// A new attempt failed before task creation. Do not reuse a previous
			// attempt's output or persist arbitrary errors that may contain secrets.
			task := models.Task{Name: "Prepare service installation", Type: "server:installation_preparation", User: server.RootUsername(),
				Status: "failed", Output: dbtype.EncryptedString(servicePreparationFailureMessage)}
			task.ServerID = server.ID
			if err := tx.Create(&task).Error; err != nil {
				return err
			}
			taskID = task.ID
			changedTask = &task
		} else {
			var task models.Task
			if err := tx.Where("id = ? AND server_id = ?", taskID, server.ID).First(&task).Error; err != nil {
				return err
			}
			if task.Status == "pending" || task.Status == "running" {
				fields := map[string]any{"status": "failed"}
				if task.Output.IsEmpty() {
					message := servicePreparationFailureMessage
					if task.Status == "running" {
						message = "Service installation failed before its final output could be saved. Check the server connection, then retry installation."
					}
					fields["output"] = dbtype.EncryptedString(message)
				}
				if err := tx.Model(&task).Updates(fields).Error; err != nil {
					return err
				}
				changedTask = &task
			}
		}
		if err := linkServiceInstallationTask(tx, service, taskID); err != nil {
			return err
		}
		if err := tx.Model(service).Update("status", types.ServiceStatusFailed).Error; err != nil {
			return err
		}
		eventServer = server
		return nil
	})
	if updateErr != nil {
		j.Deps.Logger.Error().Err(updateErr).
			Str("service_id", j.Payload.ServiceID).
			Msg("Failed to mark service installation as failed")
		return
	}
	if eventServer != nil {
		j.installationTaskID = taskID
		if changedTask != nil {
			j.Deps.BroadcastServerEvent(eventServer, "task.updated", changedTask.BroadcastData(changedTask.Output.String()))
		}
		j.Deps.BroadcastServerEvent(eventServer, "service.status_changed", map[string]any{
			"service_id": j.Payload.ServiceID, "status": string(types.ServiceStatusFailed), "task_id": taskID,
		})
	}
}

const servicePreparationFailureMessage = "Service installation preparation failed. Check the server connection and software selection, then retry installation."

// Lock the server before its service and retain the scope observed by Handle.
// Failure reporting must not write into a moved service or a different account.
func (j *AddServiceJob) withInstallationScope(ctx context.Context, fn func(*gorm.DB, *models.Server, *models.InstalledService) error) error {
	if j.service == nil {
		return errors.New("service installation scope is unavailable")
	}
	return j.Deps.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var server models.Server
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&server, "id = ?", j.Payload.ServerID).Error; err != nil {
			return err
		}
		var service models.InstalledService
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND server_id = ? AND software = ?", j.Payload.ServiceID, server.ID, j.Payload.Software).First(&service).Error; err != nil {
			return err
		}
		if j.service.ID != service.ID || j.service.ServerID != server.ID || j.service.Software != service.Software ||
			(j.server != nil && (j.server.ID != server.ID || j.server.TeamID != server.TeamID)) {
			return errors.New("service installation ownership changed")
		}
		return fn(tx, &server, &service)
	})
}

func linkServiceInstallationTask(tx *gorm.DB, service *models.InstalledService, taskID string) error {
	if service.TypeData == nil {
		service.TypeData = make(dbtype.JSONMap)
	}
	service.TypeData["install_task_id"] = taskID
	return tx.Model(service).Updates(map[string]any{"task_id": taskID, "type_data": service.TypeData}).Error
}

func NewAddServiceTask(serverID, serviceID, software string) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeAddService,
		AddServicePayload{ServerID: serverID, ServiceID: serviceID, Software: software},
		pkgjobs.Dedup("add_service", serverID, serviceID),
	)
}
