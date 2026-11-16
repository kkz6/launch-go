package jobs

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/modules/notification/channels"
	notificationmodels "github.com/kkz6/launch-go/internal/modules/notification/models"
	notificationtypes "github.com/kkz6/launch-go/internal/modules/notification/types"
	"github.com/kkz6/launch-go/internal/modules/server/models"
	"github.com/kkz6/launch-go/internal/modules/server/tasks"
	"github.com/kkz6/launch-go/internal/modules/server/types"
	fiberutil "github.com/kkz6/launch-go/internal/pkg/fiber"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const (
	TypePollSystemUpdates = "server:poll_system_updates"
	TypeReadSystemUpdates = "server:read_system_updates"
)

type ReadSystemUpdatesPayload struct {
	ServerID string `json:"server_id"`
}

type PollSystemUpdatesJob struct{ Deps *JobDeps }

func (j *PollSystemUpdatesJob) Handle(ctx context.Context) error {
	var ids []string
	if err := j.Deps.DB.WithContext(ctx).Model(&models.SystemUpdateMonitor{}).
		Joins("JOIN servers ON servers.id = server_system_update_monitors.server_id").
		Where("servers.status = ? AND servers.archived_at IS NULL", types.ServerStatusRunning).
		Pluck("server_system_update_monitors.server_id", &ids).Error; err != nil {
		return err
	}
	for _, id := range ids {
		err := j.Deps.Dispatch(TypeReadSystemUpdates, ReadSystemUpdatesPayload{ServerID: id},
			asynq.Queue("low"), asynq.Unique(2*time.Minute), asynq.MaxRetry(1), asynq.Timeout(time.Minute))
		if err != nil && !errors.Is(err, asynq.ErrDuplicateTask) {
			return err
		}
	}
	return nil
}

func NewPollSystemUpdatesTask() (*asynq.Task, error) {
	return pkgjobs.Task(TypePollSystemUpdates, struct{}{}, asynq.Unique(time.Minute))
}

type ReadSystemUpdatesJob struct {
	Deps    *JobDeps
	Payload ReadSystemUpdatesPayload
}

func (j *ReadSystemUpdatesJob) Handle(ctx context.Context) error {
	server, err := j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
	if err != nil {
		if fiberutil.IsNotFound(err) {
			return nil
		}
		return err
	}
	if server.IsArchived() || server.Status != types.ServerStatusRunning {
		return nil
	}
	db := j.Deps.DB.WithContext(ctx)
	var monitor models.SystemUpdateMonitor
	if err := db.First(&monitor, "server_id = ?", server.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	result, err := tasks.MonitorSystemUpdates(ctx, server, j.Deps.Dispatcher, j.Deps.Logger)
	if err != nil {
		return err
	}
	// Scheduled runs execute on the host. Publish changes when the monitor
	// observes them, independently of whether report notifications are enabled.
	state, err := json.Marshal(result)
	if err != nil {
		return err
	}
	stateHash := fmt.Sprintf("%x", sha256.Sum256(state))
	if stateHash != monitor.StateHash {
		j.Deps.BroadcastServerEvent(server, "server.system_updates.updated", map[string]any{
			"team_id": server.TeamID,
		})
		if err := db.Model(&monitor).Update("state_hash", stateHash).Error; err != nil {
			return err
		}
	}
	if !result.Supported || !result.Managed {
		return nil
	}

	// Advance silently while notifications are disabled so opting back in
	// does not send a backlog of historical reports.
	for _, report := range result.Reports {
		if report.ID <= monitor.LastRunID {
			continue
		}
		if result.Notifications && (report.Notifications == nil || *report.Notifications) {
			message := systemUpdateReportMessage(server.Name, report)
			if err := j.send(ctx, server, "Security update report", message); err != nil {
				return err
			}
		}
		if err := db.Model(&monitor).Update("last_run_id", report.ID).Error; err != nil {
			return err
		}
		monitor.LastRunID = report.ID
	}
	if result.RebootID == monitor.RebootID {
		return nil
	}
	// The boot ID deduplicates the alert even if another package
	// updates the reboot marker before the server is restarted.
	if result.Notifications && result.RebootRequired {
		if err := j.send(ctx, server, "Server reboot required", fmt.Sprintf("%s requires a reboot to finish applying updates.", server.Name)); err != nil {
			return err
		}
	}
	return db.Model(&monitor).Update("reboot_id", result.RebootID).Error
}

func systemUpdateReportMessage(name string, report tasks.SystemUpdateReport) string {
	var body strings.Builder
	fmt.Fprintf(&body, "%s completed a security update run at %s.\n", name, report.FinishedAt.UTC().Format("2006-01-02 15:04 UTC"))
	if report.Status == "failed" {
		body.Reset()
		fmt.Fprintf(&body, "Security updates failed on %s at %s. Open System Updates and choose View update log to diagnose the failure.\n", name, report.FinishedAt.UTC().Format("2006-01-02 15:04 UTC"))
	}
	fmt.Fprintf(&body, "\nPackages upgraded: %d", len(report.Packages))
	for i, pkg := range report.Packages {
		if i == 15 {
			fmt.Fprintf(&body, "\n… and %d more", len(report.Packages)-i)
			break
		}
		fmt.Fprintf(&body, "\n%s %s", pkg.Name, pkg.Version)
	}
	if report.RebootRequired {
		body.WriteString("\n\nA reboot is required.")
	}
	return body.String()
}

type systemUpdateNotification struct {
	*notificationmodels.BaseNotification
	subject string
}

func (n systemUpdateNotification) ToEmail() *channels.EmailMessage {
	return &channels.EmailMessage{Subject: n.subject, Body: n.RawText()}
}

func (j *ReadSystemUpdatesJob) send(ctx context.Context, server *models.Server, subject, message string) error {
	notifier := j.Deps.TaskRunnerDeps.Notifier
	if notifier == nil {
		return nil
	}
	if j.Deps.FrontendURL != "" {
		message += "\n\n" + strings.TrimRight(j.Deps.FrontendURL, "/") +
			"/servers/" + url.PathEscape(server.ID) + "?tab=advanced&subtab=system-updates"
	}
	return notifier.SendToTeam(ctx, server.TeamID, systemUpdateNotification{
		BaseNotification: notificationmodels.NewBaseNotification(notificationtypes.NotificationTypeSystemUpdates, message),
		subject:          subject,
	})
}
