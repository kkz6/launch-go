package jobs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	serverformatter "github.com/kkz6/launch-go/internal/modules/server/formatter"
	servermodels "github.com/kkz6/launch-go/internal/modules/server/models"
	servertypes "github.com/kkz6/launch-go/internal/modules/server/types"
	"github.com/kkz6/launch-go/internal/modules/site/models"
	"github.com/kkz6/launch-go/internal/modules/site/tasks"
	sitetypes "github.com/kkz6/launch-go/internal/modules/site/types"
	"github.com/kkz6/launch-go/internal/pkg/dbtype"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
	"github.com/kkz6/launch-go/internal/pkg/launch/activity"
)

const TypeUpdateSitePHPVersion = "site:update_php_version"

// UpdateSitePHPVersionPayload holds the requested transition. PreviousVersion
// is an optimistic concurrency guard so an old queued request cannot overwrite
// a newer site setting.
type UpdateSitePHPVersionPayload struct {
	SiteID                 string  `json:"site_id"`
	PreviousVersion        string  `json:"previous_version"`
	PreviousVersionWasNull bool    `json:"previous_version_was_null,omitempty"`
	Version                string  `json:"version"`
	UserID                 *string `json:"user_id,omitempty"`
}

type commandChange struct {
	ID              string
	PreviousCommand string
	Command         string
}

type phpRuntimeTransition struct {
	UpdateConfig   tasks.UpdatePHPVersionConfig
	RollbackConfig tasks.UpdatePHPVersionConfig
	Queues         []commandChange
	Crons          []commandChange
	Daemons        []commandChange
}

// UpdateSitePHPVersionJob switches a site and every PHP-bound managed process
// to the selected PHP runtime.
type UpdateSitePHPVersionJob struct {
	Deps    *JobDeps
	Payload UpdateSitePHPVersionPayload

	site   *models.Site
	server *servermodels.Server
}

func NewUpdateSitePHPVersionJob(payload UpdateSitePHPVersionPayload) pkgjobs.Handler {
	return &UpdateSitePHPVersionJob{Deps: deps, Payload: payload}
}

func (j *UpdateSitePHPVersionJob) Handle(ctx context.Context) error {
	target, err := sitetypes.ParsePhpVersion(j.Payload.Version)
	if err != nil {
		return err
	}

	site, err := j.Deps.Repos.Site().FindByID(ctx, j.Payload.SiteID)
	if err != nil {
		return fmt.Errorf("failed to find site: %w", err)
	}
	j.site = site

	if site.PendingPhpVersion == nil || site.PendingPhpVersion.String() != target.String() {
		return fmt.Errorf(
			"site PHP update reservation changed: expected %q",
			target.String(),
		)
	}

	if site.PhpVersion != nil && site.PhpVersion.String() == target.String() {
		return j.clearPendingUpdate(ctx)
	}

	current := ""
	if site.PhpVersion != nil {
		current = site.PhpVersion.String()
	}
	if (j.Payload.PreviousVersionWasNull && site.PhpVersion != nil) ||
		(!j.Payload.PreviousVersionWasNull && current != j.Payload.PreviousVersion) {
		return fmt.Errorf(
			"site PHP version changed while update was queued: expected %q, found %q",
			j.Payload.PreviousVersion,
			current,
		)
	}

	server, err := j.Deps.ServerRepos.Server().FindByID(ctx, site.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}
	j.server = server

	if err := j.validateTargetService(ctx, server, target); err != nil {
		return err
	}

	transition, err := j.buildRuntimeTransition(ctx, site, server, target)
	if err != nil {
		return err
	}

	updateTask := tasks.UpdatePHPVersion(transition.UpdateConfig)
	result, err := j.Deps.RunTask(server, updateTask).AsRoot().Dispatch(ctx)
	if err != nil {
		return fmt.Errorf("failed to update site PHP runtime: %w", err)
	}
	if result == nil || !result.IsSuccessful() {
		output := ""
		if result != nil {
			output = result.GetOutput()
		}
		return fmt.Errorf("failed to update site PHP runtime: %s", output)
	}

	if err := j.persistTransition(ctx, site, target, transition); err != nil {
		rollbackCtx, cancelRollback := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Minute)
		defer cancelRollback()
		rollbackErr := j.rollbackRuntime(rollbackCtx, server, transition.RollbackConfig)
		if rollbackErr != nil {
			return fmt.Errorf("persist PHP version: %w; runtime rollback also failed: %v", err, rollbackErr)
		}
		return fmt.Errorf("persist PHP version: %w (runtime was rolled back)", err)
	}

	site.PhpVersion = &target
	site.PendingCaddyfileUpdateSince = nil
	site.PendingPhpVersion = nil
	activity.RecordEventWithProps(
		ctx,
		"php_version_updated",
		stringValue(j.Payload.UserID),
		site,
		fmt.Sprintf("%s now uses PHP %s", site.Address, target.GetVersion()),
		map[string]any{
			"previous_version": j.Payload.PreviousVersion,
			"version":          target.String(),
		},
	)
	j.Deps.BroadcastServerEvent(server, "site.php_version_updated", map[string]any{
		"team_id":     server.TeamID,
		"server_id":   server.ID,
		"site_id":     site.ID,
		"address":     site.Address,
		"php_version": target.String(),
	})
	j.Deps.BroadcastServerEvent(server, "site.updated", map[string]any{
		"team_id":   server.TeamID,
		"server_id": server.ID,
		"site_id":   site.ID,
	})

	return nil
}

func (j *UpdateSitePHPVersionJob) validateTargetService(
	ctx context.Context,
	server *servermodels.Server,
	target sitetypes.PhpVersion,
) error {
	software, err := servertypes.ParseSoftware(target.String())
	if err != nil {
		return err
	}

	service, err := j.Deps.ServerRepos.Service().FindByServerAndSoftware(ctx, server.ID, software)
	if err != nil {
		return fmt.Errorf("PHP %s is not installed on this server: %w", target.GetVersion(), err)
	}
	if service.Type != servertypes.ServiceTypePhp || !service.Status.IsActive() {
		return fmt.Errorf(
			"PHP %s is not active on this server (status: %s)",
			target.GetVersion(),
			service.Status,
		)
	}

	return nil
}

func (j *UpdateSitePHPVersionJob) buildRuntimeTransition(
	ctx context.Context,
	site *models.Site,
	server *servermodels.Server,
	target sitetypes.PhpVersion,
) (*phpRuntimeTransition, error) {
	previous := sitetypes.PhpVersion(j.Payload.PreviousVersion)
	if !previous.IsValid() {
		return nil, fmt.Errorf("invalid previous PHP version: %s", j.Payload.PreviousVersion)
	}

	updateConfig := phpTaskConfig(site, target)
	rollbackConfig := phpTaskConfig(site, previous)

	// Caddy always needs to move to the selected FPM socket for installed
	// sites. Both versions are generated before touching remote state so a
	// database failure can restore the exact previous runtime configuration.
	if site.InstalledAt != nil {
		installedRedirects, err := j.Deps.Repos.Redirect().FindBySiteForCaddy(ctx, site.ID)
		if err != nil {
			return nil, fmt.Errorf("find installed redirects: %w", err)
		}
		pendingRedirects, err := j.Deps.Repos.Redirect().FindPendingBySite(ctx, site.ID)
		if err != nil {
			return nil, fmt.Errorf("find pending redirects: %w", err)
		}
		redirects := append(installedRedirects, pendingRedirects...)
		helper := &UpdateCaddyfileJob{Deps: j.Deps}
		loadBalancerIP := helper.resolveLoadBalancerIP(ctx, site)
		if site.IsLoadBalanced() && loadBalancerIP == "" {
			return nil, errors.New("resolve load balancer IP for site")
		}
		activeCert, err := j.Deps.Repos.Certificate().FindActiveBySite(ctx, site.ID)
		if err != nil {
			return nil, fmt.Errorf("find active certificate: %w", err)
		}

		nextSite := *site
		nextSite.PhpVersion = &target
		previousSite := *site
		if j.Payload.PreviousVersionWasNull {
			previousSite.PhpVersion = nil
		} else {
			previousSite.PhpVersion = &previous
		}

		caddyfilePath := fmt.Sprintf("%s/Caddyfile", site.Path)
		updateConfig.CaddyfilePath = caddyfilePath
		rollbackConfig.CaddyfilePath = caddyfilePath
		updateConfig.Files = append(updateConfig.Files, tasks.PHPVersionConfigFile{
			Path:     caddyfilePath,
			Contents: generateCaddyfile(&nextSite, redirects, loadBalancerIP, activeCert),
			Owner:    site.User + ":" + site.User,
		})
		rollbackConfig.Files = append(rollbackConfig.Files, tasks.PHPVersionConfigFile{
			Path:     caddyfilePath,
			Contents: generateCaddyfile(&previousSite, redirects, loadBalancerIP, activeCert),
			Owner:    site.User + ":" + site.User,
		})
	}

	transition := &phpRuntimeTransition{
		UpdateConfig:   updateConfig,
		RollbackConfig: rollbackConfig,
	}
	oldBinary, newBinary := previous.BinaryPath(), target.BinaryPath()
	serverUsername := server.GetUsername()

	queues, err := j.Deps.Repos.Queue().FindBySite(ctx, site.ID)
	if err != nil {
		return nil, fmt.Errorf("find site queues: %w", err)
	}
	for i := range queues {
		previousQueue := queues[i]
		if strings.TrimSpace(previousQueue.Command) == "" {
			previousQueue.Command = fmt.Sprintf(
				"%s %s/artisan queue:work",
				oldBinary,
				site.GetApplicationDirectory(),
			)
		}
		updatedCommand, changed := replacePHPExecutable(previousQueue.Command, oldBinary, newBinary)
		if !changed {
			continue
		}

		nextQueue := previousQueue
		nextQueue.Command = updatedCommand
		transition.Queues = append(transition.Queues, commandChange{
			ID:              nextQueue.ID,
			PreviousCommand: queues[i].Command,
			Command:         updatedCommand,
		})

		if queueIsInstalled(&queues[i]) {
			transition.UpdateConfig.Files = append(transition.UpdateConfig.Files, tasks.PHPVersionConfigFile{
				Path:     nextQueue.GetPath(),
				Contents: tasks.BuildQueueSupervisorConfig(&nextQueue, serverUsername),
				Owner:    "root:root",
			})
			transition.RollbackConfig.Files = append(transition.RollbackConfig.Files, tasks.PHPVersionConfigFile{
				Path:     previousQueue.GetPath(),
				Contents: tasks.BuildQueueSupervisorConfig(&previousQueue, serverUsername),
				Owner:    "root:root",
			})
			transition.UpdateConfig.SupervisorPrograms = append(transition.UpdateConfig.SupervisorPrograms, nextQueue.ID)
			transition.RollbackConfig.SupervisorPrograms = append(transition.RollbackConfig.SupervisorPrograms, previousQueue.ID)
		}
	}

	crons, err := j.Deps.ServerRepos.Cron().FindByServer(ctx, server.ID)
	if err != nil {
		return nil, fmt.Errorf("find server crons: %w", err)
	}
	for i := range crons {
		if crons[i].SiteID == nil || *crons[i].SiteID != site.ID {
			continue
		}
		updatedCommand, changed := replacePHPExecutable(crons[i].GetCommand(), oldBinary, newBinary)
		if !changed {
			continue
		}

		previousCron := crons[i]
		previousCron.Server = server
		nextCron := previousCron
		nextCron.Command = dbtype.EncryptedString(updatedCommand)
		transition.Crons = append(transition.Crons, commandChange{
			ID:              nextCron.ID,
			PreviousCommand: crons[i].GetCommand(),
			Command:         updatedCommand,
		})

		if installableIsActive(nextCron.InstalledAt, nextCron.InstallationFailedAt) {
			transition.UpdateConfig.Files = append(transition.UpdateConfig.Files, tasks.PHPVersionConfigFile{
				Path:     nextCron.Path(),
				Contents: serverformatter.CronFileContents(&nextCron),
				Owner:    "root:root",
			})
			transition.RollbackConfig.Files = append(transition.RollbackConfig.Files, tasks.PHPVersionConfigFile{
				Path:     previousCron.Path(),
				Contents: serverformatter.CronFileContents(&previousCron),
				Owner:    "root:root",
			})
		}
	}

	daemonIDs := enabledFeatureDaemonIDs(site)
	for _, daemonID := range daemonIDs {
		daemon, err := j.Deps.ServerRepos.Daemon().FindByIDAndServer(ctx, daemonID, server.ID)
		if err != nil {
			return nil, fmt.Errorf("find site daemon %s: %w", daemonID, err)
		}
		updatedCommand, changed := replacePHPExecutable(daemon.Command, oldBinary, newBinary)
		if !changed {
			continue
		}

		previousDaemon := *daemon
		previousDaemon.Server = server
		nextDaemon := previousDaemon
		nextDaemon.Command = updatedCommand
		transition.Daemons = append(transition.Daemons, commandChange{
			ID:              nextDaemon.ID,
			PreviousCommand: daemon.Command,
			Command:         updatedCommand,
		})

		if installableIsActive(nextDaemon.InstalledAt, nextDaemon.InstallationFailedAt) {
			transition.UpdateConfig.Files = append(transition.UpdateConfig.Files, tasks.PHPVersionConfigFile{
				Path:     nextDaemon.Path(),
				Contents: serverformatter.SupervisorConfig(&nextDaemon),
				Owner:    "root:root",
			})
			transition.RollbackConfig.Files = append(transition.RollbackConfig.Files, tasks.PHPVersionConfigFile{
				Path:     previousDaemon.Path(),
				Contents: serverformatter.SupervisorConfig(&previousDaemon),
				Owner:    "root:root",
			})
			transition.UpdateConfig.SupervisorPrograms = append(
				transition.UpdateConfig.SupervisorPrograms,
				nextDaemon.ProgramName(),
			)
			transition.RollbackConfig.SupervisorPrograms = append(
				transition.RollbackConfig.SupervisorPrograms,
				previousDaemon.ProgramName(),
			)
		}
	}

	return transition, nil
}

func (j *UpdateSitePHPVersionJob) persistTransition(
	ctx context.Context,
	site *models.Site,
	target sitetypes.PhpVersion,
	transition *phpRuntimeTransition,
) error {
	if j.Deps.DB == nil {
		return errors.New("database is not configured")
	}

	return j.Deps.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Model(&models.Site{}).
			Where("id = ? AND pending_php_version = ?", site.ID, target.String())
		if j.Payload.PreviousVersionWasNull {
			query = query.Where("php_version IS NULL")
		} else {
			query = query.Where("php_version = ?", j.Payload.PreviousVersion)
		}
		result := query.Updates(map[string]any{
			"php_version":                    target.String(),
			"pending_caddyfile_update_since": nil,
			"pending_php_version":            nil,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("site PHP version changed concurrently")
		}

		for _, change := range transition.Queues {
			result := tx.Model(&models.Queue{}).
				Where(
					"id = ? AND site_id = ? AND command = ?",
					change.ID,
					site.ID,
					change.PreviousCommand,
				).
				Update("command", change.Command)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return fmt.Errorf("queue %s changed concurrently", change.ID)
			}
		}
		for _, change := range transition.Crons {
			var cron servermodels.Cron
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ? AND site_id = ?", change.ID, site.ID).
				First(&cron).Error; err != nil {
				return err
			}
			if cron.GetCommand() != change.PreviousCommand {
				return fmt.Errorf("cron %s changed concurrently", change.ID)
			}
			if err := tx.Model(&cron).
				Update("command", dbtype.EncryptedString(change.Command)).Error; err != nil {
				return err
			}
		}
		for _, change := range transition.Daemons {
			result := tx.Model(&servermodels.Daemon{}).
				Where(
					"id = ? AND server_id = ? AND command = ?",
					change.ID,
					site.ServerID,
					change.PreviousCommand,
				).
				Update("command", change.Command)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return fmt.Errorf("daemon %s changed concurrently", change.ID)
			}
		}
		return tx.Model(&models.Redirect{}).
			Where("site_id = ? AND status = ?", site.ID, "pending").
			Update("status", "installed").Error
	})
}

func (j *UpdateSitePHPVersionJob) rollbackRuntime(
	ctx context.Context,
	server *servermodels.Server,
	config tasks.UpdatePHPVersionConfig,
) error {
	rollbackTask := tasks.UpdatePHPVersion(config)
	rollbackTask.SetName(fmt.Sprintf("Rollback %s PHP update", j.site.Address))
	result, err := j.Deps.RunTask(server, rollbackTask).AsRoot().Dispatch(ctx)
	if err != nil {
		return err
	}
	if result == nil || !result.IsSuccessful() {
		if result == nil {
			return errors.New("rollback task returned no result")
		}
		return fmt.Errorf("rollback failed: %s", result.GetOutput())
	}
	return nil
}

func (j *UpdateSitePHPVersionJob) clearPendingUpdate(ctx context.Context) error {
	if j.Deps.DB == nil {
		return nil
	}
	cleanupCtx, cancelCleanup := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancelCleanup()
	return j.Deps.DB.WithContext(cleanupCtx).
		Model(&models.Site{}).
		Where(
			"id = ? AND pending_php_version = ?",
			j.Payload.SiteID,
			j.Payload.Version,
		).
		Updates(map[string]any{
			"pending_caddyfile_update_since": nil,
			"pending_php_version":            nil,
		}).Error
}

func (j *UpdateSitePHPVersionJob) Failed(ctx context.Context, err error) {
	cleanupCtx, cancelCleanup := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancelCleanup()
	_ = j.clearPendingUpdate(cleanupCtx)
	j.Deps.Logger.Error().
		Err(err).
		Str("site_id", j.Payload.SiteID).
		Str("php_version", j.Payload.Version).
		Msg("Site PHP version update failed")

	site := j.site
	if site == nil {
		site, _ = j.Deps.Repos.Site().FindByID(cleanupCtx, j.Payload.SiteID)
	}
	if site == nil {
		return
	}

	activity.RecordEventWithProps(
		cleanupCtx,
		"php_version_update_failed",
		stringValue(j.Payload.UserID),
		site,
		fmt.Sprintf("Failed to update %s to PHP %s: %v", site.Address, j.Payload.Version, err),
		map[string]any{
			"previous_version": j.Payload.PreviousVersion,
			"version":          j.Payload.Version,
			"error":            err.Error(),
		},
	)
	server := j.server
	if server == nil {
		server, _ = j.Deps.ServerRepos.Server().FindByID(cleanupCtx, site.ServerID)
	}
	if server != nil {
		j.Deps.BroadcastServerEvent(server, "site.php_version_update_failed", map[string]any{
			"team_id":     server.TeamID,
			"server_id":   server.ID,
			"site_id":     site.ID,
			"address":     site.Address,
			"php_version": j.Payload.Version,
			"error":       err.Error(),
		})
	}
}

func NewUpdateSitePHPVersionTask(
	siteID,
	previousVersion,
	version string,
	previousVersionWasNull bool,
	userID *string,
) (*asynq.Task, error) {
	return pkgjobs.Task(
		TypeUpdateSitePHPVersion,
		UpdateSitePHPVersionPayload{
			SiteID:                 siteID,
			PreviousVersion:        previousVersion,
			PreviousVersionWasNull: previousVersionWasNull,
			Version:                version,
			UserID:                 userID,
		},
		asynq.MaxRetry(0),
	)
}

func phpTaskConfig(site *models.Site, version sitetypes.PhpVersion) tasks.UpdatePHPVersionConfig {
	return tasks.UpdatePHPVersionConfig{
		SiteAddress: site.Address,
		Version:     version.GetVersion(),
		PHPBinary:   version.BinaryPath(),
		FPMService:  version.FPMServiceName(),
		FPMSocket:   version.SocketPath(),
	}
}

func replacePHPExecutable(command, previous, next string) (string, bool) {
	if previous == "" {
		return command, false
	}

	// Older queue and scheduler records used the unversioned `php` binary.
	// Those processes still belong to the site runtime and must be pinned to
	// the selected series during a switch.
	executables := []string{previous, "php"}
	if previous == next {
		// A legacy site with a NULL PHP version resolves its current runtime
		// from the server default. Selecting that same series still needs to
		// pin generic `php` commands, while already-versioned commands are
		// already in their desired state.
		executables = []string{"php"}
	}
	for _, executable := range executables {
		if strings.HasPrefix(command, executable+" ") {
			return next + strings.TrimPrefix(command, executable), true
		}
		for _, separator := range []string{"&& ", "; ", "|| ", "| "} {
			needle := separator + executable + " "
			if strings.Contains(command, needle) {
				return strings.Replace(command, needle, separator+next+" ", 1), true
			}
		}
	}
	return command, false
}

func enabledFeatureDaemonIDs(site *models.Site) []string {
	seen := make(map[string]struct{})
	ids := make([]string, 0)
	for _, feature := range site.EnabledFeatures {
		if feature.DaemonID == nil || *feature.DaemonID == "" {
			continue
		}
		if _, ok := seen[*feature.DaemonID]; ok {
			continue
		}
		seen[*feature.DaemonID] = struct{}{}
		ids = append(ids, *feature.DaemonID)
	}
	return ids
}

func queueIsInstalled(queue *models.Queue) bool {
	return installableIsActive(queue.InstalledAt, queue.InstallationFailedAt)
}

func installableIsActive(installedAt, failedAt *time.Time) bool {
	return installedAt != nil && failedAt == nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
