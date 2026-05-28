package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"github.com/kkz6/launch-go/internal/modules/server/models"
	pkgjobs "github.com/kkz6/launch-go/internal/pkg/jobs"
)

const TypeBackfillDetectedOS = "server:backfill_detected_os"

// BackfillDetectedOSPayload — empty ServerID means "scan every
// connected server in the team / globally and backfill each one that
// still has a NULL detected_os_id". Specific ServerID means "just
// this one." Most callers will pass a specific ID; the all-servers
// path is reserved for a future scheduled-task wakeup.
type BackfillDetectedOSPayload struct {
	ServerID string `json:"server_id,omitempty"`
}

// BackfillDetectedOSJob backfills the detected_os_* columns on
// already-provisioned servers that were created before the detect_os
// provision step existed. Runs a one-shot SSH command (no script
// upload, no marker streaming) since this is purely informational
// and doesn't need to participate in the provisioning lifecycle.
type BackfillDetectedOSJob struct {
	Deps    *JobDeps
	Payload BackfillDetectedOSPayload
}

func NewBackfillDetectedOSJob(p BackfillDetectedOSPayload) pkgjobs.Handler {
	return &BackfillDetectedOSJob{Deps: deps, Payload: p}
}

func (j *BackfillDetectedOSJob) Handle(ctx context.Context) error {
	if j.Payload.ServerID != "" {
		server, err := j.Deps.Repos.Server().FindByID(ctx, j.Payload.ServerID)
		if err != nil {
			return fmt.Errorf("failed to find server: %w", err)
		}
		return j.backfillOne(ctx, server)
	}

	// All-servers branch. Scan only connected, non-archived rows that
	// still lack detected_os_id; everything else is either offline
	// (no point retrying), already populated (no-op), or scheduled
	// for deletion.
	var servers []models.Server
	if err := j.Deps.DB.WithContext(ctx).
		Where("connected = TRUE AND archived_at IS NULL AND detected_os_id IS NULL").
		Find(&servers).Error; err != nil {
		return fmt.Errorf("failed to scan servers: %w", err)
	}

	j.Deps.Logger.Info().
		Int("candidates", len(servers)).
		Msg("BackfillDetectedOS: starting global sweep")

	var errs []string
	for i := range servers {
		if err := j.backfillOne(ctx, &servers[i]); err != nil {
			// Don't fail the whole sweep on one bad server — log and
			// continue. A single unreachable box shouldn't block
			// backfilling the rest.
			j.Deps.Logger.Warn().Err(err).
				Str("server_id", servers[i].ID).
				Msg("BackfillDetectedOS: per-server failure, continuing")
			errs = append(errs, fmt.Sprintf("%s: %s", servers[i].ID, err.Error()))
		}
	}
	if len(errs) > 0 {
		// Return a summary error so asynq's retry policy can kick in
		// for transient cases (one Redis blip would otherwise lose the
		// whole sweep). The per-row errors are already in the log.
		return fmt.Errorf("backfill completed with %d failure(s); first: %s", len(errs), errs[0])
	}
	return nil
}

// backfillOne SSHes into a single box, asks it for its OS facts, and
// writes them onto the server row. Kept stateless and side-effect-
// free: no marker streaming, no progress updates, no broadcasts beyond
// the resulting server.updated emit at the end. This is a maintenance
// task, not part of the provisioning lifecycle.
func (j *BackfillDetectedOSJob) backfillOne(ctx context.Context, server *models.Server) error {
	if server.PublicIPv4 == nil || *server.PublicIPv4 == "" {
		return fmt.Errorf("no IP recorded for %s", server.ID)
	}
	if server.PrivateKey.IsEmpty() {
		return fmt.Errorf("no private key recorded for %s", server.ID)
	}

	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	conn := server.ConnectionAsRoot()
	client, err := conn.Dial()
	if err != nil {
		return fmt.Errorf("ssh dial failed: %w", err)
	}
	defer client.Close()

	// One-liner that mirrors detect_os.sh's emission format. printf
	// keeps it on a single output line so we don't have to reassemble
	// multi-line output. Pipe separator matches the provision script.
	const cmd = `set -e
. /etc/os-release
KERNEL_ARCH="$(uname -m 2>/dev/null || echo "")"
case "${KERNEL_ARCH}" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    armv7l)  ARCH="armhf" ;;
    *)       ARCH="${KERNEL_ARCH}" ;;
esac
KERNEL="$(uname -r 2>/dev/null || echo "")"
printf "%s|%s|%s|%s|%s" "${ID:-}" "${VERSION_ID:-}" "${VERSION_CODENAME:-}" "${ARCH}" "${KERNEL}"
`

	result, err := client.Run(dialCtx, cmd)
	if err != nil {
		return fmt.Errorf("ssh run failed: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("remote script exited %d: %s", result.ExitCode, strings.TrimSpace(result.Stderr))
	}

	parts := strings.SplitN(strings.TrimSpace(result.Stdout), "|", 5)
	for len(parts) < 5 {
		parts = append(parts, "")
	}

	updates := map[string]any{
		"detected_os_id":               nilIfEmpty(parts[0]),
		"detected_os_version":          nilIfEmpty(parts[1]),
		"detected_os_version_codename": nilIfEmpty(parts[2]),
		"detected_arch":                nilIfEmpty(parts[3]),
		"detected_kernel":              nilIfEmpty(parts[4]),
		"detected_at":                  time.Now(),
	}
	if err := j.Deps.DB.WithContext(ctx).
		Model(&models.Server{}).
		Where("id = ?", server.ID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("db update failed: %w", err)
	}

	j.Deps.BroadcastServerEvent(server, "server.updated", map[string]any{
		"id":        server.ID,
		"model":     "server",
		"action":    "updated",
		"server_id": server.ID,
		"team_id":   server.TeamID,
	})
	j.Deps.Logger.Info().
		Str("server_id", server.ID).
		Str("os_id", parts[0]).
		Str("os_version", parts[1]).
		Str("os_codename", parts[2]).
		Str("arch", parts[3]).
		Str("kernel", parts[4]).
		Msg("Detected OS facts backfilled")
	return nil
}

func nilIfEmpty(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

// NewBackfillDetectedOSTask creates an asynq task. Empty serverID
// means "scan all eligible servers"; a specific ID means "just this
// one." Deduplicates per-server so the same server can't be enqueued
// twice while one is still in flight.
func NewBackfillDetectedOSTask(serverID string) (*asynq.Task, error) {
	dedupKey := "all"
	if serverID != "" {
		dedupKey = serverID
	}
	return pkgjobs.Task(TypeBackfillDetectedOS, BackfillDetectedOSPayload{
		ServerID: serverID,
	}, asynq.TaskID(pkgjobs.Dedup("backfill_detected_os", dedupKey)))
}
