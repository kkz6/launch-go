package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeSetDefaultPhp = "server:set_default_php"

type SetDefaultPhpPayload struct {
	ServerID       string              `json:"server_id"`
	ServiceID      string              `json:"service_id"`
	Version        string              `json:"version"`
	PreviousStatus types.ServiceStatus `json:"previous_status"`
	UserID         *string             `json:"user_id,omitempty"`
}

// SetDefaultPhpJob sets the default PHP version on a server
type SetDefaultPhpJob struct {
	Deps    *JobDeps
	Payload SetDefaultPhpPayload

	server          *models.Server
	service         *models.InstalledService
	previousDefault *models.InstalledService
	previousRuntime string
}

func NewSetDefaultPhpJob(p SetDefaultPhpPayload) pkgjobs.Handler {
	return &SetDefaultPhpJob{Deps: deps, Payload: p}
}

func (j *SetDefaultPhpJob) Handle(ctx context.Context) error {
	var err error

	if !j.Payload.PreviousStatus.IsActive() {
		return errors.New("default PHP change has an invalid previous service status")
	}

	j.server, err = j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	j.service, err = j.Deps.Repos.Service().FindByID(ctx, j.Payload.ServiceID)
	if err != nil {
		return fmt.Errorf("failed to find service: %w", err)
	}
	if j.service.ServerID != j.Payload.ServerID {
		return fmt.Errorf("PHP service does not belong to server")
	}
	if j.server.PendingDefaultPHPServiceID == nil ||
		*j.server.PendingDefaultPHPServiceID != j.Payload.ServiceID {
		return errors.New("default PHP change reservation was lost")
	}
	if j.service.Type != types.ServiceTypePhp || !j.service.GetSoftware().IsPhp() {
		return fmt.Errorf("service is not a PHP installation")
	}
	if j.service.Status != types.ServiceStatusUpdating {
		return errors.New("default PHP target service reservation was lost")
	}

	version := j.service.PhpVersionSeries()
	if j.Payload.Version != version {
		return fmt.Errorf("PHP version does not match installed service")
	}

	phpServices, err := j.Deps.Repos.Service().FindByServerAndType(
		ctx,
		j.server.ID,
		types.ServiceTypePhp,
	)
	if err != nil {
		return fmt.Errorf("find current default PHP: %w", err)
	}
	for i := range phpServices {
		if phpServices[i].IsDefault {
			currentDefault := phpServices[i]
			j.previousDefault = &currentDefault
			break
		}
	}
	if j.service.IsDefault {
		restored, releaseErr := j.releaseReservation(ctx)
		if releaseErr != nil {
			return releaseErr
		}
		if !restored {
			return errors.New("default PHP target service reservation was lost")
		}
		j.service.Status = j.Payload.PreviousStatus
		return nil
	}

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("version", version).
		Msg("setting default PHP version")

	// Run the update alternatives task
	task := tasks.UpdateAlternatives(version)
	task.SetName("Set Default PHP " + version)

	result, err := j.Deps.RunTask(j.server, task).
		AsRoot().
		TrackInDB().
		Dispatch(ctx)

	if err != nil {
		return fmt.Errorf("failed to set default PHP: %w", err)
	}

	if !result.IsSuccessful() {
		return fmt.Errorf("failed to set default PHP: %s", result.GetOutput())
	}

	j.previousRuntime = tasks.ParsePreviousDefaultPHPVersion(result.GetOutput())
	if !types.SoftwareFromPhpVersion(j.previousRuntime).IsPhp() {
		if j.previousDefault != nil {
			j.previousRuntime = j.previousDefault.PhpVersionSeries()
		}
	}

	if err := j.persistDefault(ctx); err != nil {
		if !types.SoftwareFromPhpVersion(j.previousRuntime).IsPhp() {
			return fmt.Errorf(
				"persist default PHP: %w; previous default runtime is unknown",
				err,
			)
		}
		if j.previousRuntime == version {
			return fmt.Errorf(
				"persist default PHP: %w (runtime was already PHP %s)",
				err,
				version,
			)
		}
		rollbackCtx, cancelRollback := context.WithTimeout(
			context.WithoutCancel(ctx),
			3*time.Minute,
		)
		defer cancelRollback()
		if rollbackErr := j.rollbackDefault(rollbackCtx, j.previousRuntime); rollbackErr != nil {
			return fmt.Errorf(
				"persist default PHP: %w; runtime rollback failed: %v",
				err,
				rollbackErr,
			)
		}
		return fmt.Errorf("persist default PHP: %w (runtime was rolled back)", err)
	}
	j.service.IsDefault = true
	j.service.Status = j.Payload.PreviousStatus
	j.server.PendingDefaultPHPServiceID = nil

	j.Deps.Logger.Info().
		Str("server_id", j.server.ID).
		Str("service_id", j.service.ID).
		Str("version", j.Payload.Version).
		Msg("default PHP version set successfully")

	j.Deps.BroadcastServerEvent(j.server, "php.default_changed", map[string]any{
		"server_id":  j.server.ID,
		"service_id": j.service.ID,
		"version":    version,
	})
	j.Deps.BroadcastServerEvent(j.server, "php.default_change", map[string]any{
		"server_id":  j.server.ID,
		"service_id": j.service.ID,
		"status":     "finished",
		"version":    version,
	})

	return nil
}

func (j *SetDefaultPhpJob) Failed(ctx context.Context, err error) {
	cleanupCtx, cancelCleanup := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancelCleanup()
	restored, releaseErr := j.releaseReservation(cleanupCtx)
	if releaseErr != nil {
		j.Deps.Logger.Error().
			Err(releaseErr).
			Str("server_id", j.Payload.ServerID).
			Msg("failed to release default PHP reservation")
	} else if !restored {
		j.Deps.Logger.Warn().
			Str("server_id", j.Payload.ServerID).
			Str("service_id", j.Payload.ServiceID).
			Msg("default PHP target state changed before reservation cleanup")
	}
	j.Deps.Logger.Error().Err(err).
		Str("server_id", j.Payload.ServerID).
		Str("version", j.Payload.Version).
		Msg("failed to set default PHP version")
	if j.server != nil {
		j.Deps.BroadcastServerEvent(j.server, "php.default_change", map[string]any{
			"server_id":  j.Payload.ServerID,
			"service_id": j.Payload.ServiceID,
			"status":     "failed",
			"version":    j.Payload.Version,
			"error":      err.Error(),
		})
	}
}

func (j *SetDefaultPhpJob) persistDefault(ctx context.Context) error {
	db := j.Deps.DB
	if db == nil {
		db = j.Deps.Repos.DB()
	}
	if db == nil {
		return errors.New("database is not configured")
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		target := tx.Model(&models.InstalledService{}).
			Where(
				"id = ? AND server_id = ? AND type = ? AND status = ?",
				j.Payload.ServiceID,
				j.Payload.ServerID,
				types.ServiceTypePhp,
				types.ServiceStatusUpdating,
			).
			Updates(map[string]any{
				"is_default": true,
				"status":     j.Payload.PreviousStatus,
			})
		if target.Error != nil {
			return target.Error
		}
		if target.RowsAffected != 1 {
			return errors.New("default PHP target service reservation was lost")
		}
		if err := tx.Model(&models.InstalledService{}).
			Where(
				"server_id = ? AND type = ? AND id <> ?",
				j.Payload.ServerID,
				types.ServiceTypePhp,
				j.Payload.ServiceID,
			).
			Update("is_default", false).Error; err != nil {
			return err
		}
		reservation := tx.Model(&models.Server{}).
			Where(
				"id = ? AND pending_default_php_service_id = ?",
				j.Payload.ServerID,
				j.Payload.ServiceID,
			).
			Update("pending_default_php_service_id", nil)
		if reservation.Error != nil {
			return reservation.Error
		}
		if reservation.RowsAffected != 1 {
			return errors.New("default PHP change reservation was lost")
		}
		return nil
	})
}

func (j *SetDefaultPhpJob) rollbackDefault(ctx context.Context, version string) error {
	task := tasks.UpdateAlternatives(version)
	task.SetName("Rollback Default PHP to " + version)
	result, err := j.Deps.RunTask(j.server, task).AsRoot().TrackInDB().Dispatch(ctx)
	if err != nil {
		return err
	}
	if result == nil {
		return errors.New("rollback default PHP returned no result")
	}
	if !result.IsSuccessful() {
		return fmt.Errorf("rollback default PHP: %s", result.GetOutput())
	}
	return nil
}

func (j *SetDefaultPhpJob) releaseReservation(ctx context.Context) (bool, error) {
	db := j.Deps.DB
	if db == nil {
		db = j.Deps.Repos.DB()
	}
	if db == nil {
		return false, nil
	}
	cleanupCtx, cancelCleanup := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancelCleanup()

	restored := false
	err := db.WithContext(cleanupCtx).Transaction(func(tx *gorm.DB) error {
		reservation := tx.Model(&models.Server{}).
			Where(
				"id = ? AND pending_default_php_service_id = ?",
				j.Payload.ServerID,
				j.Payload.ServiceID,
			).
			Update("pending_default_php_service_id", nil)
		if reservation.Error != nil {
			return reservation.Error
		}
		if reservation.RowsAffected != 1 {
			return nil
		}
		if !j.Payload.PreviousStatus.IsActive() {
			return nil
		}

		target := tx.Model(&models.InstalledService{}).
			Where(
				"id = ? AND server_id = ? AND type = ? AND status = ?",
				j.Payload.ServiceID,
				j.Payload.ServerID,
				types.ServiceTypePhp,
				types.ServiceStatusUpdating,
			).
			Update("status", j.Payload.PreviousStatus)
		if target.Error != nil {
			return target.Error
		}
		restored = target.RowsAffected == 1
		return nil
	})
	return restored, err
}

func NewSetDefaultPhpTask(
	serverID,
	serviceID,
	version string,
	previousStatus types.ServiceStatus,
	userID *string,
) (*asynq.Task, error) {
	return pkgjobs.Task(
		TypeSetDefaultPhp,
		SetDefaultPhpPayload{
			ServerID:       serverID,
			ServiceID:      serviceID,
			Version:        version,
			PreviousStatus: previousStatus,
			UserID:         userID,
		},
	)
}
