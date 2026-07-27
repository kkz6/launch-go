package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/server/types"
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

	server  *models.Server
	service *models.InstalledService
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

	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("find server: %w", err)
	}

	if err := j.Deps.Repos.Service().UpdateStatus(ctx, j.service.ID, types.ServiceStatusInstalling); err != nil {
		return fmt.Errorf("update service status: %w", err)
	}

	j.Deps.BroadcastServerEvent(j.server, "service.status_changed", map[string]any{
		"service_id": j.service.ID,
		"server_id":  j.server.ID,
		"status":     string(types.ServiceStatusInstalling),
	})

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

	result, err := j.Deps.RunTask(j.server, task).
		AsRoot().
		TrackInDB().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("install service: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("install service: %s", result.GetOutput())
	}

	if err := j.Deps.Repos.Service().UpdateStatus(ctx, j.service.ID, types.ServiceStatusRunning); err != nil {
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

	if updateErr := j.Deps.Repos.Service().UpdateStatus(ctx, j.Payload.ServiceID, types.ServiceStatusFailed); updateErr != nil {
		j.Deps.Logger.Error().Err(updateErr).
			Str("service_id", j.Payload.ServiceID).
			Msg("Failed to mark service installation as failed")
	}
}

func NewAddServiceTask(serverID, serviceID, software string) (*asynq.Task, error) {
	return pkgjobs.TaskWithID(TypeAddService,
		AddServicePayload{ServerID: serverID, ServiceID: serviceID, Software: software},
		pkgjobs.Dedup("add_service", serverID, serviceID),
	)
}
